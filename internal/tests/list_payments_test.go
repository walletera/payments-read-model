package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"testing"

	"github.com/cucumber/godog"
	"github.com/walletera/payments-types/publicapi"
)

const listPaymentsOkKey = "listPaymentsOkKey"

func TestListPayments(t *testing.T) {

	suite := godog.TestSuite{
		ScenarioInitializer: InitializeListPaymentsFeature,
		Options: &godog.Options{
			Format:   "pretty",
			Paths:    []string{"features/list_payments.feature"},
			TestingT: t, // Testing instance that will run subtests.
		},
	}

	if suite.Run() != 0 {
		t.Fatal("non-zero status returned, failed to run feature tests")
	}
}

func InitializeListPaymentsFeature(ctx *godog.ScenarioContext) {
	ctx.Before(beforeScenarioHook)
	ctx.Given(`^a running payments-read-model$`, aRunningPaymentsReadModel)
	ctx.Step(`^a list of payment created events is published and processed successfully by the payments-read-model:$`, aListOfPaymentCreatedEvents)
	ctx.Step(`^the payments-read-model receives a GET request on endpoint \/payments with filters (.+)$`, thePaymentsRMReceivesAGETRequestOnEndpointPaymentsWithFilters)
	ctx.Step(`^the returned payments ids match (.+)$`, theReturnedPaymentsIdsMatch)
	ctx.After(afterScenarioHook)
}

func aListOfPaymentCreatedEvents(ctx context.Context, eventsListJsonFilePath *godog.DocString) (context.Context, error) {
	if eventsListJsonFilePath == nil || len(eventsListJsonFilePath.Content) == 0 {
		return ctx, fmt.Errorf("the eventsListJsonFilePath is empty or was not defined")
	}

	rawEventsList, err := os.ReadFile(eventsListJsonFilePath.Content)
	if err != nil {
		return ctx, fmt.Errorf("error reading event JSON file: %w", err)
	}

	var eventsList []json.RawMessage
	err = json.Unmarshal(rawEventsList, &eventsList)
	if err != nil {
		return ctx, fmt.Errorf("error unmarshalling event JSON file: %w", err)
	}

	for _, event := range eventsList {
		ctx, err = deserializeAndAddtoContext(ctx, event)
		if err != nil {
			return ctx, err
		}
		ctx, err = theEventIsPublished(ctx)
		if err != nil {
			return ctx, err
		}
	}

	logsWatcherFromCtx(ctx).WaitForNTimes(
		"payment saved",
		logsWatcherWaitForTimeout,
		len(eventsList),
	)

	return ctx, nil
}

func thePaymentsRMReceivesAGETRequestOnEndpointPaymentsWithFilters(ctx context.Context, filters string) (context.Context, error) {
	if filters == "" {
		return ctx, fmt.Errorf("filters is empty")
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/payments%s", publicApiHttpServerPort, filters)
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

	var listPaymentsOK publicapi.ListPaymentsOK
	err = json.NewDecoder(resp.Body).Decode(&listPaymentsOK)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return context.WithValue(ctx, listPaymentsOkKey, listPaymentsOK), nil
}

func theReturnedPaymentsIdsMatch(ctx context.Context, paymentIdsJson string) error {
	listPaymentsOk := listPaymentsOkFromCtx(ctx)

	var paymentIds []string
	err := json.Unmarshal([]byte(paymentIdsJson), &paymentIds)
	if err != nil {
		return fmt.Errorf("failed to unmarshal paymentIdsJson: %w", err)
	}

	returnedIds := make([]string, len(listPaymentsOk.Items))
	for i, payment := range listPaymentsOk.Items {
		returnedIds[i] = payment.ID.String()
	}

	slices.Sort(paymentIds)
	slices.Sort(returnedIds)

	if !slices.Equal(paymentIds, returnedIds) {
		return fmt.Errorf("returned payment IDs %v do not match expected IDs %v", returnedIds, paymentIds)
	}

	return nil
}

func listPaymentsOkFromCtx(ctx context.Context) publicapi.ListPaymentsOK {
	value := ctx.Value(listPaymentsOkKey)
	if value == nil {
		panic("listPaymentsOk not found in context")
	}
	listPaymentsOk, ok := value.(publicapi.ListPaymentsOK)
	if !ok {
		panic("listPaymentsOk has invalid type")
	}
	return listPaymentsOk
}
