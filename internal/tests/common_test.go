package tests

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "time"

    "github.com/walletera/payments-read-model/internal/app"
    "github.com/walletera/payments-read-model/internal/tests/httpauth"

    "github.com/cucumber/godog"
    "github.com/walletera/eventskit/events"
    "github.com/walletera/eventskit/rabbitmq"
    slogwatcher "github.com/walletera/logs-watcher/slog"
    paymentsevents "github.com/walletera/payments-types/events"
    "github.com/walletera/payments-types/publicapi"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
    "go.uber.org/zap"
    "go.uber.org/zap/exp/zapslog"
    "go.uber.org/zap/zapcore"
)

const (
    appKey                    = "app"
    appCtxCancelFuncKey       = "appCtxCancelFuncKey"
    logsWatcherKey            = "logsWatcher"
    rawEventKey               = "rawEvent"
    deserializedEventKey      = "deserializedEvent"
    logsWatcherWaitForTimeout = 5 * time.Second
    publicApiHttpServerPort   = 8484
    mongodbURL                = "mongodb://localhost:27017/?retryWrites=true&w=majority"
)

var mongodbClient *mongo.Client

func beforeScenarioHook(ctx context.Context, _ *godog.Scenario) (context.Context, error) {
    handler, err := newZapHandler()
    if err != nil {
        return ctx, err
    }
    logsWatcher := slogwatcher.NewWatcher(handler)
    ctx = context.WithValue(ctx, logsWatcherKey, logsWatcher)

    client, err := getMongodbClient()
    if err != nil {
        return ctx, err
    }

    // cleanup database before each scenario
    err = client.Database("payments").Collection("payments").Drop(ctx)
    if err != nil {
        return nil, err
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

func aRunningPaymentsReadModel(ctx context.Context) (context.Context, error) {
    logHandler := logsWatcherFromCtx(ctx).DecoratedHandler()

    appCtx, appCtxCancelFunc := context.WithCancel(ctx)

    paymentsRMApp, err := app.NewApp(
        app.WithPublicAPIConfig(app.PublicAPIConfig{
            PublicAPIHttpServerPort: publicApiHttpServerPort,
        }),
        app.WithRabbitmqHost(rabbitmq.DefaultHost),
        app.WithRabbitmqPort(rabbitmq.DefaultPort),
        app.WithRabbitmqUser(rabbitmq.DefaultUser),
        app.WithRabbitmqPassword(rabbitmq.DefaultPassword),
        app.WithMongoDBURL(mongodbURL),
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

func anEvent(ctx context.Context, eventJsonFilePath *godog.DocString) (context.Context, error) {
    if eventJsonFilePath == nil || len(eventJsonFilePath.Content) == 0 {
        return ctx, fmt.Errorf("the eventJsonFilePath is empty or was not defined")
    }

    rawEvent, err := os.ReadFile(eventJsonFilePath.Content)
    if err != nil {
        return ctx, fmt.Errorf("error reading event JSON file: %w", err)
    }

    return deserializeAndAddtoContext(ctx, rawEvent)
}

func deserializeAndAddtoContext(ctx context.Context, rawEvent []byte) (context.Context, error) {
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

    var routingKey string
    deserializedEvent := ctx.Value(deserializedEventKey)
    switch deserializedEvent.(type) {
    case paymentsevents.PaymentCreated:
        routingKey = app.RabbitMQPaymentCreatedRoutingKey
    case paymentsevents.PaymentUpdated:
        routingKey = app.RabbitMQPaymentUpdatedRoutingKey
    default:
        return ctx, fmt.Errorf("unsupported event type: %T", deserializedEvent)
    }
    rawEvent := ctx.Value(rawEventKey).([]byte)
    err = publisher.Publish(ctx, publishable{rawEvent: rawEvent}, events.RoutingInfo{
        Topic:      app.RabbitMQPaymentsExchangeName,
        RoutingKey: routingKey,
    })
    if err != nil {
        return ctx, fmt.Errorf("error publishing WithdrawalCreated event to rabbitmq: %s", err.Error())
    }

    return ctx, nil
}

func retrievePayments(ctx context.Context, params publicapi.ListPaymentsParams) (*publicapi.ListPaymentsOK, error) {
    paymentsClient, err := publicapi.NewClient(
        fmt.Sprintf("http://127.0.0.1:%d", publicApiHttpServerPort),
        // TODO add valid token
        httpauth.NewSecuritySource("ajsonwebtoken"),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to create payments client: %w", err)
    }

    listPaymentsRes, err := paymentsClient.ListPayments(ctx, params)
    if err != nil {
        return nil, fmt.Errorf("failed to list payments: %w", err)
    }

    listPaymentsOk, ok := listPaymentsRes.(*publicapi.ListPaymentsOK)
    if !ok {
        return nil, fmt.Errorf("failed to cast listPaymentsRes to ListPaymentsOK")
    }

    return listPaymentsOk, nil
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

func getMongodbClient() (*mongo.Client, error) {
    if mongodbClient != nil {
        return mongodbClient, nil
    }

    MongodbUri := "mongodb://localhost:27017/?retryWrites=true&w=majority"

    // Use the SetServerAPIOptions() method to set the Stable API version to 1
    serverAPI := options.ServerAPI(options.ServerAPIVersion1)
    opts := options.Client().ApplyURI(MongodbUri).SetServerAPIOptions(serverAPI)

    // Create a new client and connect to the server
    mongodbClient, err := mongo.Connect(opts)
    if err != nil {
        return nil, err
    }

    return mongodbClient, nil
}
