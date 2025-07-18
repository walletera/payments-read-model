package payments

import (
    "context"
    "time"

    "github.com/google/uuid"
    "github.com/walletera/payments-types/privateapi"
    "github.com/walletera/werrors"
)

type Payment struct {
    ID               uuid.UUID
    AggregateVersion uint64
    Data             privateapi.Payment
}

type PaymentUpdate struct {
    PaymentId        uuid.UUID
    AggregateVersion uint64
    ExternalId       privateapi.OptString
    Status           privateapi.PaymentStatus
    UpdatedAt        time.Time
}

type Iterator interface {
    Next() (*Payment, error)
}

type QueryResult struct {
    Iterator Iterator
    Total    uint64
}

type Repository interface {
    GetPayment(ctx context.Context, id uuid.UUID) (Payment, werrors.WError)
    SavePayment(ctx context.Context, payment Payment) werrors.WError
    UpdatePayment(ctx context.Context, payment PaymentUpdate) werrors.WError
    SearchPayments(ctx context.Context, query string) (QueryResult, werrors.WError)
}
