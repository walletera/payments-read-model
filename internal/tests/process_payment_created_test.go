package tests

import (
    "context"
    "fmt"
    "testing"

    "github.com/cucumber/godog"
    paymentsevents "github.com/walletera/payments-types/events"
    "github.com/walletera/payments-types/publicapi"
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
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

func thePaymentExistInThePaymentsReadModel(ctx context.Context) (context.Context, error) {
    paymentCreatedEvent := paymentCreatedEventFromCtx(ctx)

    payments, err := retrievePayments(ctx, publicapi.ListPaymentsParams{ID: publicapi.NewOptUUID(paymentCreatedEvent.Data.ID)})
    if err != nil {
        return ctx, err
    }

    if payments.Items == nil {
        return ctx, fmt.Errorf("ListPaymentsResponse.Items is nil")
    }

    if len(payments.Items) != 1 {
        return ctx, fmt.Errorf("expected exactly one payment with ID %s, but found %d", paymentCreatedEvent.Data.ID, len(payments.Items))
    }

    if !payments.Total.IsSet() {
        return ctx, fmt.Errorf("ListPaymentsResponse.Total is not set in ListPaymentsResponse")
    }

    if payments.Total.Value != 1 {
        return ctx, fmt.Errorf("expected exactly one payment with ID %s, but found %d", paymentCreatedEvent.Data.ID, payments.Total.Value)
    }

    payment := payments.Items[0]

    if payment.ID != paymentCreatedEvent.Data.ID {
        return ctx, fmt.Errorf("expected payment ID to be %s, but got %s", paymentCreatedEvent.Data.ID, payment.ID)
    }

    if payment.ExternalId.Value != paymentCreatedEvent.Data.ExternalId.Value {
        return ctx, fmt.Errorf("expected payment externalId to be %s, but got %s", paymentCreatedEvent.Data.ExternalId.Value, payment.ExternalId.Value)
    }

    if string(payment.Status) != string(paymentCreatedEvent.Data.Status) {
        return ctx, fmt.Errorf("expected payment status to be %s, but got %s", paymentCreatedEvent.Data.Status, payment.Status)
    }

    if payment.Amount != paymentCreatedEvent.Data.Amount {
        return ctx, fmt.Errorf("expected payment amount to be %f, but got %f", paymentCreatedEvent.Data.Amount, payment.Amount)
    }

    if string(payment.Currency) != string(paymentCreatedEvent.Data.Currency) {
        return ctx, fmt.Errorf("expected payment currency to be %s, but got %s", paymentCreatedEvent.Data.Currency, payment.Currency)
    }

    if !payment.CreatedAt.Equal(paymentCreatedEvent.Data.CreatedAt) {
        return ctx, fmt.Errorf("expected payment createdAt to be %s, but got %s", paymentCreatedEvent.Data.CreatedAt, payment.CreatedAt)
    }

    if !payment.UpdatedAt.Equal(paymentCreatedEvent.Data.UpdatedAt) {
        return ctx, fmt.Errorf("expected payment updatedAt to be %s, but got %s", paymentCreatedEvent.Data.UpdatedAt, payment.UpdatedAt)
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
