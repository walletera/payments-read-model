package tests

import (
    "context"
    "fmt"
    "testing"

    "payments-read-model/internal/adapters/mongodb"

    "github.com/cucumber/godog"
    paymentsevents "github.com/walletera/payments-types/events"
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
)

func TestPaymentUpdatedEventProcessing(t *testing.T) {

    suite := godog.TestSuite{
        ScenarioInitializer: InitializeProcessPaymentUpdatedFeature,
        Options: &godog.Options{
            Format:   "pretty",
            Paths:    []string{"features/process_payment_updated.feature"},
            TestingT: t, // Testing instance that will run subtests.
        },
    }

    if suite.Run() != 0 {
        t.Fatal("non-zero status returned, failed to run feature tests")
    }
}

func InitializeProcessPaymentUpdatedFeature(ctx *godog.ScenarioContext) {
    ctx.Before(beforeScenarioHook)
    ctx.Given(`^a running payments-read-model$`, aRunningPaymentsReadModel)
    ctx.Given(`^a PaymentCreated event:$`, anEvent)
    ctx.Given(`^the event is published$`, theEventIsPublished)
    ctx.Given(`^the payments-read-model produces the following log:$`, thePaymentsRMProducesTheFollowingLog)
    ctx.Given(`^a PaymentUpdated event:$`, anEvent)
    ctx.When(`^the event is published$`, theEventIsPublished)
    ctx.Then(`^the payments-read-model produces the following log:$`, thePaymentsRMProducesTheFollowingLog)
    ctx.Then(`^the payment in the payments-read-model has the expected new values in the updated fields$`, thePaymentsReadModelHasTheExpectedNewValues)
    ctx.After(afterScenarioHook)
}

func thePaymentsReadModelHasTheExpectedNewValues(ctx context.Context) (context.Context, error) {
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

    event := paymentUpdatedEventFromCtx(ctx)

    coll := client.Database("payments").Collection("payments")

    retrievedPayment := mongodb.PaymentBSON{}
    singleResult := coll.FindOne(ctx, bson.D{{"_id", event.Data.PaymentId}})
    if singleResult.Err() != nil {
        return ctx, singleResult.Err()
    }

    err = singleResult.Decode(&retrievedPayment)
    if err != nil {
        return ctx, err
    }

    if retrievedPayment.Data.Status != event.Data.Status {
        return ctx, fmt.Errorf("expected payment status to be %s, but got %s", event.Data.Status, retrievedPayment.Data.Status)
    }

    if retrievedPayment.Data.ExternalId != event.Data.ExternalId {
        return ctx, fmt.Errorf("expected payment externalId to be %s, but got %s", event.Data.ExternalId.Value, retrievedPayment.Data.ExternalId.Value)
    }

    return ctx, nil
}

func paymentUpdatedEventFromCtx(ctx context.Context) paymentsevents.PaymentUpdated {
    value := ctx.Value(deserializedEventKey)
    if value == nil {
        panic("paymentUpdatedEvent not found in context")
    }
    paymentUpdatedEvent, ok := value.(paymentsevents.PaymentUpdated)
    if !ok {
        panic("paymentUpdatedEvent has invalid type")
    }
    return paymentUpdatedEvent
}
