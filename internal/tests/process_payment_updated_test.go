package tests

import (
    "context"
    "fmt"
    "testing"

    "github.com/cucumber/godog"
    paymentsevents "github.com/walletera/payments-types/events"
    "github.com/walletera/payments-types/publicapi"
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
    event := paymentUpdatedEventFromCtx(ctx)

    listPaymentsOk, err := retrievePayments(ctx, publicapi.ListPaymentsParams{ID: publicapi.NewOptUUID(event.Data.PaymentId)})
    if err != nil {
        return ctx, err
    }

    retrievedPayment := listPaymentsOk.Items[0]

    if string(retrievedPayment.Status) != string(event.Data.Status) {
        return ctx, fmt.Errorf("expected payment status to be %s, but got %s", event.Data.Status, retrievedPayment.Status)
    }

    if retrievedPayment.ExternalId.Value != event.Data.ExternalId.Value {
        return ctx, fmt.Errorf("expected payment externalId to be %s, but got %s", event.Data.ExternalId.Value, retrievedPayment.ExternalId.Value)
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
