package tests

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "testing"

    "github.com/cucumber/godog"
    "github.com/walletera/payments-types/publicapi"
)

const (
    responseStatusCodeKey = "responseStatusCode"
    getPaymentOkKey       = "getPayment"
)

func TestGetPayment(t *testing.T) {

    suite := godog.TestSuite{
        ScenarioInitializer: InitializeGetPaymentFeature,
        Options: &godog.Options{
            Format:   "pretty",
            Paths:    []string{"features/get_payment.feature"},
            TestingT: t, // Testing instance that will run subtests.
        },
    }

    if suite.Run() != 0 {
        t.Fatal("non-zero status returned, failed to run feature tests")
    }
}

func InitializeGetPaymentFeature(ctx *godog.ScenarioContext) {
    ctx.Before(beforeScenarioHook)
    ctx.Given(`^a running payments-read-model$`, aRunningPaymentsReadModel)
    ctx.Given(`^a list of payment created events is published and processed successfully by the payments-read-model:$`, aListOfPaymentCreatedEvents)
    ctx.When(`^the payments-read-model receives a GET request on endpoint \/payments\/paymentId with paymentId (.+)$`, thePaymentsRMReceivesAGETRequestOnEndpointPaymentsId)
    ctx.Then(`^the payments-read-model respond with status code (\d+)$`, thePaymentsRMRespondWithStatusCode)
    ctx.Then(`^payment id (.+)$`, theReturnedPaymentIdIs)
    ctx.Then(`^external id (.+)$`, theReturnedPaymentExternalIdIs)
    ctx.Then(`^status (.+)$`, theReturnedPaymentStatusIs)
    ctx.After(afterScenarioHook)
}

func thePaymentsRMReceivesAGETRequestOnEndpointPaymentsId(ctx context.Context, paymentId string) (context.Context, error) {
    url := fmt.Sprintf("http://127.0.0.1:%d/payments/%s", publicApiHttpServerPort, paymentId)
    request, err := http.NewRequest(http.MethodGet, url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    request.Header.Set("Authorization", "Bearer ajsonwebtoken")

    resp, err := http.DefaultClient.Do(request)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer func(Body io.ReadCloser) {
        err := Body.Close()
        if err != nil {
            panic(err)
        }
    }(resp.Body)

    ctx = context.WithValue(ctx, responseStatusCodeKey, resp.StatusCode)

    if resp.StatusCode == http.StatusOK {
        var payment publicapi.Payment
        err = json.NewDecoder(resp.Body).Decode(&payment)
        if err != nil {
            return nil, fmt.Errorf("failed to decode response: %w", err)
        }

        ctx = context.WithValue(ctx, getPaymentOkKey, payment)
    }

    return ctx, nil
}

func thePaymentsRMRespondWithStatusCode(ctx context.Context, statusCode int) (context.Context, error) {
    responseStatusCode := getResponseStatusCodeFromCtx(ctx)
    if responseStatusCode != statusCode {
        return ctx, fmt.Errorf("expected response status code to be %d, but got %d", statusCode, responseStatusCode)
    }
    return ctx, nil
}

func getResponseStatusCodeFromCtx(ctx context.Context) int {
    value := ctx.Value(responseStatusCodeKey)
    if value == nil {
        panic("responseStatusCode not found in context")
    }
    statusCode, ok := value.(int)
    if !ok {
        panic("responseStatusCode has invalid type")
    }
    return statusCode
}

func theReturnedPaymentIdIs(ctx context.Context, paymentId string) error {
    if getResponseStatusCodeFromCtx(ctx) != http.StatusOK {
        return nil
    }
    payment := getPaymentFromCtx(ctx)
    if payment.ID.String() != paymentId {
        return fmt.Errorf("expected payment ID to be %s, but got %s", paymentId, payment.ID.String())
    }
    return nil
}

func theReturnedPaymentExternalIdIs(ctx context.Context, externalId string) error {
    if getResponseStatusCodeFromCtx(ctx) != http.StatusOK {
        return nil
    }
    payment := getPaymentFromCtx(ctx)
    if payment.ExternalId.Value != externalId {
        return fmt.Errorf("expected payment externalId to be %s, but got %s", externalId, payment.ExternalId.Value)
    }
    return nil
}

func theReturnedPaymentStatusIs(ctx context.Context, status string) error {
    if getResponseStatusCodeFromCtx(ctx) != http.StatusOK {
        return nil
    }
    payment := getPaymentFromCtx(ctx)
    if string(payment.Status) != status {
        return fmt.Errorf("expected payment status to be %s, but got %s", status, string(payment.Status))
    }
    return nil
}

func getPaymentFromCtx(ctx context.Context) publicapi.Payment {
    value := ctx.Value(getPaymentOkKey)
    if value == nil {
        panic("getPaymentOk not found in context")
    }
    getPaymentOk, ok := value.(publicapi.Payment)
    if !ok {
        panic("getPaymentOk has invalid type")
    }
    return getPaymentOk
}
