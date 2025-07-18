package tests

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "testing"
    "time"

    "payments-read-model/internal/adapters/mongodb"
    "payments-read-model/internal/app"

    "github.com/cucumber/godog"
    "github.com/walletera/eventskit/events"
    slogwatcher "github.com/walletera/logs-watcher/slog"
    paymentsevents "github.com/walletera/payments-types/events"
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
    "go.uber.org/zap"
    "go.uber.org/zap/exp/zapslog"
    "go.uber.org/zap/zapcore"

    "github.com/walletera/eventskit/rabbitmq"
)

const (
    appKey                    = "app"
    appCtxCancelFuncKey       = "appCtxCancelFuncKey"
    logsWatcherKey            = "logsWatcher"
    rawEventKey               = "rawEvent"
    deserializedEventKey      = "deserializedEvent"
    logsWatcherWaitForTimeout = 5 * time.Second
)

func TestPaymentCreatedEventProcessing(t *testing.T) {

    suite := godog.TestSuite{
        ScenarioInitializer: InitializeProcessPaymentCreatedFeature,
        Options: &godog.Options{
            Format:   "pretty",
            Paths:    []string{"features/process_payment_created.feature"},
            TestingT: t, // Testing instance that will run subtests.
        },
    }

    if suite.Run() != 0 {
        t.Fatal("non-zero status returned, failed to run feature tests")
    }
}

func InitializeProcessPaymentCreatedFeature(ctx *godog.ScenarioContext) {
    ctx.Before(beforeScenarioHook)
    ctx.Given(`^a running payments-read-model$`, aRunningPaymentsReadModel)
    ctx.Given(`^a PaymentCreated event:$`, anEvent)
    ctx.Given(`^the event is published$`, theEventIsPublished)
    ctx.Given(`^the payments-read-model produces the following log:$`, thePaymentsRMProducesTheFollowingLog)
    ctx.When(`^the event is published$`, theEventIsPublished)
    ctx.When(`^the same PaymentCreated event is published again$`, theSamePaymentCreatedEventIsPublishedAgain)
    ctx.Then(`^the payments-read-model produces the following log:$`, thePaymentsRMProducesTheFollowingLog)
    ctx.Then(`^the payment exist in the payments-read-model$`, thePaymentExistInThePaymentsReadModel)
    ctx.Then(`^only one payment with the given id exists in the payments-read-model$`, onlyOnePaymentExist)
    ctx.After(afterScenarioHook)
}

func beforeScenarioHook(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
    handler, err := newZapHandler()
    if err != nil {
        return ctx, err
    }
    logsWatcher := slogwatcher.NewWatcher(handler)
    ctx = context.WithValue(ctx, logsWatcherKey, logsWatcher)
    return ctx, nil
}

func aRunningPaymentsReadModel(ctx context.Context) (context.Context, error) {
    logHandler := logsWatcherFromCtx(ctx).DecoratedHandler()

    appCtx, appCtxCancelFunc := context.WithCancel(ctx)

    paymentsRMApp, err := app.NewApp(
        app.WithRabbitmqHost(rabbitmq.DefaultHost),
        app.WithRabbitmqPort(rabbitmq.DefaultPort),
        app.WithRabbitmqUser(rabbitmq.DefaultUser),
        app.WithRabbitmqPassword(rabbitmq.DefaultPassword),
        app.WithLogHandler(logHandler),
    )
    if err != nil {
        appCtxCancelFunc()
        return ctx, fmt.Errorf("failed initializing paymentsRMApp: " + err.Error())
    }

    err = paymentsRMApp.Run(appCtx)
    if err != nil {
        appCtxCancelFunc()
        return ctx, fmt.Errorf("failed running paymentsRMApp" + err.Error())
    }

    ctx = context.WithValue(ctx, appKey, paymentsRMApp)
    ctx = context.WithValue(ctx, appCtxCancelFuncKey, appCtxCancelFunc)

    foundLogEntry := logsWatcherFromCtx(ctx).WaitFor("payments-read-model started", logsWatcherWaitForTimeout)
    if !foundLogEntry {
        return ctx, fmt.Errorf("paymentsRMApp startup failed (didn't find expected log entry)")
    }

    return ctx, nil
}

func afterScenarioHook(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
    logsWatcher := logsWatcherFromCtx(ctx)

    appFromCtx(ctx).Stop(ctx)
    foundLogEntry := logsWatcher.WaitFor("payments-read-model stopped", logsWatcherWaitForTimeout)
    if !foundLogEntry {
        return ctx, fmt.Errorf("app termination failed (didn't find expected log entry)")
    }

    err = logsWatcher.Stop()
    if err != nil {
        return ctx, fmt.Errorf("failed stopping the logsWatcher: %w", err)
    }

    return ctx, nil
}

func anEvent(ctx context.Context, event *godog.DocString) (context.Context, error) {
    if event == nil || len(event.Content) == 0 {
        return ctx, fmt.Errorf("the event is empty or was not defined")
    }
    rawEvent := []byte(event.Content)
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    deserializer := paymentsevents.NewDeserializer(logger)
    deserializedEvent, err := deserializer.Deserialize(rawEvent)
    if err != nil {
        return ctx, err
    }
    ctx = context.WithValue(ctx, deserializedEventKey, deserializedEvent)
    return context.WithValue(ctx, rawEventKey, rawEvent), nil
}

func theEventIsPublished(ctx context.Context) (context.Context, error) {
    publisher, err := rabbitmq.NewClient(
        rabbitmq.WithExchangeName(app.RabbitMQPaymentsExchangeName),
        rabbitmq.WithExchangeType(app.RabbitMQExchangeType),
    )
    if err != nil {
        return nil, fmt.Errorf("error creating rabbitmq client: %s", err.Error())
    }

    rawEvent := ctx.Value(rawEventKey).([]byte)
    err = publisher.Publish(ctx, publishable{rawEvent: rawEvent}, events.RoutingInfo{
        Topic:      app.RabbitMQPaymentsExchangeName,
        RoutingKey: app.RabbitMQPaymentCreatedRoutingKey,
    })
    if err != nil {
        return nil, fmt.Errorf("error publishing WithdrawalCreated event to rabbitmq: %s", err.Error())
    }

    return ctx, nil
}

func thePaymentExistInThePaymentsReadModel(ctx context.Context) (context.Context, error) {
    MongodbUri := "mongodb://localhost:27017/?retryWrites=true&w=majority"

    // Use the SetServerAPIOptions() method to set the Stable API version to 1
    serverAPI := options.ServerAPI(options.ServerAPIVersion1)
    opts := options.Client().ApplyURI(MongodbUri).SetServerAPIOptions(serverAPI)

    // Create a new client and connect to the server
    client, err := mongo.Connect(opts)
    if err != nil {
        panic(err)
    }
    defer func() {
        if err = client.Disconnect(context.TODO()); err != nil {
            panic(err)
        }
    }()

    paymentCreatedEvent := paymentCreatedEventFromCtx(ctx)

    coll := client.Database("payments").Collection("payments")

    retrievedPayment := mongodb.PaymentBSON{}
    singleResult := coll.FindOne(ctx, bson.D{{"_id", paymentCreatedEvent.Data.ID}})
    if singleResult.Err() != nil {
        return ctx, singleResult.Err()
    }

    err = singleResult.Decode(&retrievedPayment)
    if err != nil {
        return ctx, err
    }

    if retrievedPayment.ID != paymentCreatedEvent.Data.ID {
        return ctx, fmt.Errorf("expected payment ID to be %s, but got %s", paymentCreatedEvent.Data.ID, retrievedPayment.ID)
    }

    if retrievedPayment.Data.ExternalId != paymentCreatedEvent.Data.ExternalId {
        return ctx, fmt.Errorf("expected payment externalId to be %s, but got %s", paymentCreatedEvent.Data.ExternalId.Value, retrievedPayment.Data.ExternalId.Value)
    }

    if retrievedPayment.Data.Status != paymentCreatedEvent.Data.Status {
        return ctx, fmt.Errorf("expected payment status to be %s, but got %s", paymentCreatedEvent.Data.Status, retrievedPayment.Data.Status)
    }

    if retrievedPayment.Data.Amount != paymentCreatedEvent.Data.Amount {
        return ctx, fmt.Errorf("expected payment amount to be %f, but got %f", paymentCreatedEvent.Data.Amount, retrievedPayment.Data.Amount)
    }

    if retrievedPayment.Data.Currency != paymentCreatedEvent.Data.Currency {
        return ctx, fmt.Errorf("expected payment currency to be %s, but got %s", paymentCreatedEvent.Data.Currency, retrievedPayment.Data.Currency)
    }

    if !retrievedPayment.Data.CreatedAt.Equal(paymentCreatedEvent.Data.CreatedAt) {
        return ctx, fmt.Errorf("expected payment createdAt to be %s, but got %s", paymentCreatedEvent.Data.CreatedAt, retrievedPayment.Data.CreatedAt)
    }

    if !retrievedPayment.Data.UpdatedAt.Equal(paymentCreatedEvent.Data.UpdatedAt) {
        return ctx, fmt.Errorf("expected payment updatedAt to be %s, but got %s", paymentCreatedEvent.Data.UpdatedAt, retrievedPayment.Data.UpdatedAt)
    }

    if retrievedPayment.Data.Beneficiary != paymentCreatedEvent.Data.Beneficiary {
        return ctx, fmt.Errorf("expected payment beneficiary to be %v, but got %v", paymentCreatedEvent.Data.Beneficiary, retrievedPayment.Data.Beneficiary)
    }

    if retrievedPayment.Data.Debtor != paymentCreatedEvent.Data.Debtor {
        return ctx, fmt.Errorf("expected payment debtor to be %v, but got %v", paymentCreatedEvent.Data.Debtor, retrievedPayment.Data.Debtor)
    }

    return ctx, nil
}

func onlyOnePaymentExist(ctx context.Context) (context.Context, error) {
    MongodbUri := "mongodb://localhost:27017/?retryWrites=true&w=majority"

    // Use the SetServerAPIOptions() method to set the Stable API version to 1
    serverAPI := options.ServerAPI(options.ServerAPIVersion1)
    opts := options.Client().ApplyURI(MongodbUri).SetServerAPIOptions(serverAPI)

    // Create a new client and connect to the server
    client, err := mongo.Connect(opts)
    if err != nil {
        panic(err)
    }
    defer func() {
        if err = client.Disconnect(context.TODO()); err != nil {
            panic(err)
        }
    }()

    paymentCreatedEvent := paymentCreatedEventFromCtx(ctx)

    coll := client.Database("payments").Collection("payments")

    cursor, err := coll.Find(ctx, bson.D{{"_id", paymentCreatedEvent.Data.ID}})
    if err != nil {
        return ctx, fmt.Errorf("failed to find payments: %w", err)
    }
    defer cursor.Close(ctx)

    count := 0
    for cursor.Next(ctx) {
        count++
    }
    if err := cursor.Err(); err != nil {
        return ctx, fmt.Errorf("error iterating cursor: %w", err)
    }

    if count != 1 {
        return ctx, fmt.Errorf("expected exactly one payment with ID %s, but found %d", paymentCreatedEvent.Data.ID, count)
    }

    return ctx, nil
}

func thePaymentsRMProducesTheFollowingLog(ctx context.Context, logMsg string) (context.Context, error) {
    logsWatcher := logsWatcherFromCtx(ctx)
    foundLogEntry := logsWatcher.WaitFor(logMsg, logsWatcherWaitForTimeout)
    if !foundLogEntry {
        return ctx, fmt.Errorf("didn't find expected log entry")
    }
    return ctx, nil
}

func theSamePaymentCreatedEventIsPublishedAgain(ctx context.Context) (context.Context, error) {
    return theEventIsPublished(ctx)
}

func logsWatcherFromCtx(ctx context.Context) *slogwatcher.Watcher {
    value := ctx.Value(logsWatcherKey)
    if value == nil {
        panic("logs watcher not found in context")
    }
    watcher, ok := value.(*slogwatcher.Watcher)
    if !ok {
        panic("logs watcher has invalid type")
    }
    return watcher
}

func appFromCtx(ctx context.Context) *app.App {
    value := ctx.Value(appKey)
    if value == nil {
        panic("paymentsRMApp not found in context")
    }
    paymentsRMApp, ok := value.(*app.App)
    if !ok {
        panic("paymentsRMApp has invalid type")
    }
    return paymentsRMApp
}

func paymentCreatedEventFromCtx(ctx context.Context) paymentsevents.PaymentCreated {
    value := ctx.Value(deserializedEventKey)
    if value == nil {
        panic("paymentCreatedEvent not found in context")
    }
    paymentCreatedEvent, ok := value.(paymentsevents.PaymentCreated)
    if !ok {
        panic("paymentCreatedEvent has invalid type")
    }
    return paymentCreatedEvent
}

func newZapHandler() (slog.Handler, error) {
    encoderConfig := zap.NewProductionEncoderConfig()
    encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
    zapConfig := zap.Config{
        Level:             zap.NewAtomicLevelAt(zap.DebugLevel),
        Development:       false,
        DisableStacktrace: true,
        Sampling: &zap.SamplingConfig{
            Initial:    100,
            Thereafter: 100,
        },
        Encoding:         "json",
        EncoderConfig:    encoderConfig,
        OutputPaths:      []string{"stderr"},
        ErrorOutputPaths: []string{"stderr"},
    }
    zapLogger, err := zapConfig.Build()
    if err != nil {
        return nil, err
    }
    if zapLogger.Core() == nil {
        return nil, fmt.Errorf("zapLogger.Core() is nil")
    }
    return zapslog.NewHandler(zapLogger.Core()), nil
}
