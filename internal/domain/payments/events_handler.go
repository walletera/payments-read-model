package payments

import (
    "context"
    "log/slog"

    "github.com/walletera/payments-read-model/pkg/logattr"

    "github.com/walletera/payments-types/events"
    "github.com/walletera/werrors"
)

type EventsHandler struct {
    repository Repository
    logger     *slog.Logger
}

func NewEventsHandler(repository Repository, logger *slog.Logger) *EventsHandler {
    return &EventsHandler{
        repository: repository,
        logger:     logger,
    }
}

func (e *EventsHandler) HandlePaymentCreated(ctx context.Context, paymentCreatedEvent events.PaymentCreated) werrors.WError {
    payment := Payment{
        ID:               paymentCreatedEvent.Data.ID,
        AggregateVersion: paymentCreatedEvent.AggregateVersion(),
        Data:             paymentCreatedEvent.Data,
    }
    werr := e.repository.SavePayment(ctx, payment)
    if werr != nil {
        e.logger.Error(
            "failed saving payment",
            logattr.Error(werr.Message()),
            logattr.PaymentId(paymentCreatedEvent.Data.ID.String()),
            logattr.CorrelationId(paymentCreatedEvent.CorrelationID()),
        )
        return werr
    }
    e.logger.Info(
        "payment saved",
        logattr.PaymentId(paymentCreatedEvent.Data.ID.String()),
        logattr.CorrelationId(paymentCreatedEvent.CorrelationID()),
    )
    return nil
}

func (e *EventsHandler) HandlePaymentUpdated(ctx context.Context, paymentUpdated events.PaymentUpdated) werrors.WError {
    paymentUpdate := PaymentUpdate{
        PaymentId:        paymentUpdated.Data.PaymentId,
        AggregateVersion: paymentUpdated.AggregateVersion(),
        Status:           paymentUpdated.Data.Status,
        ExternalId:       paymentUpdated.Data.ExternalId,
    }
    werr := e.repository.UpdatePayment(ctx, paymentUpdate)
    if werr != nil {
        e.logger.Error(
            "failed updating payment",
            logattr.Error(werr.Message()),
            logattr.PaymentId(paymentUpdated.Data.PaymentId.String()),
            logattr.CorrelationId(paymentUpdated.CorrelationID()),
        )
        return werr
    }
    e.logger.Info(
        "payment updated",
        logattr.PaymentId(paymentUpdated.Data.PaymentId.String()),
        logattr.CorrelationId(paymentUpdated.CorrelationID()),
    )
    return nil
}
