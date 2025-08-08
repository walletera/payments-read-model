package public

import (
    "context"
    "log/slog"

    "payments-read-model/internal/domain/payments"
    "payments-read-model/pkg/logattr"

    privconv "github.com/walletera/payments-types/converters/privateapi"
    "github.com/walletera/payments-types/privateapi"
    "github.com/walletera/payments-types/publicapi"
    "github.com/walletera/werrors"
)

type Handler struct {
    repository payments.Repository
    logger     *slog.Logger
}

var _ publicapi.Handler = (*Handler)(nil)

func NewHandler(repository payments.Repository, logger *slog.Logger) *Handler {
    return &Handler{repository: repository, logger: logger}
}

func (h Handler) PostPayment(ctx context.Context, req *publicapi.PostPaymentReq, params publicapi.PostPaymentParams) (publicapi.PostPaymentRes, error) {
    return &publicapi.PostPaymentMethodNotAllowed{}, nil
}

func (h Handler) GetPayment(ctx context.Context, params publicapi.GetPaymentParams) (publicapi.GetPaymentRes, error) {
    payment, err := h.repository.GetPayment(ctx, params.PaymentId)
    if err != nil {
        switch err.Code() {
        case werrors.ResourceNotFoundErrorCode:
            return &publicapi.GetPaymentNotFound{}, nil
        default:
            h.logger.Error(
                "failed getting payment",
                logattr.Error(err.Error()),
                logattr.PaymentId(params.PaymentId.String()),
            )
            return &publicapi.GetPaymentInternalServerError{}, nil
        }
    }

    return buildPublicPaymentFromPrivatePayment(payment.Data), nil
}

func (h Handler) ListPayments(ctx context.Context, params publicapi.ListPaymentsParams) (publicapi.ListPaymentsRes, error) {
    result, err := h.repository.SearchPayments(ctx, params)
    if err != nil {
        h.logger.Error(
            "failed listing payments",
            logattr.Error(err.Error()),
        )
        return &publicapi.ListPaymentsInternalServerError{
            ErrorMessage: "unexpected internal error",
        }, nil
    }
    var paymentsList []publicapi.Payment
    for {
        ok, payment, err := result.Iterator.Next()
        if err != nil {
            h.logger.Error(
                "failed listing payments",
                logattr.Error(err.Error()),
            )
            return &publicapi.ListPaymentsInternalServerError{
                ErrorMessage: "unexpected internal error",
            }, nil
        }
        if !ok {
            break
        }
        paymentsList = append(paymentsList, *buildPublicPaymentFromPrivatePayment(payment.Data))
    }
    return &publicapi.ListPaymentsOK{
        Items: paymentsList,
        Total: publicapi.OptInt{
            Value: int(result.Total),
            Set:   true,
        },
    }, nil
}

func buildPublicPaymentFromPrivatePayment(p privateapi.Payment) *publicapi.Payment {
    return &publicapi.Payment{
        ID:          p.ID,
        Amount:      p.Amount,
        Currency:    publicapi.Currency(p.Currency),
        Debtor:      privconv.Convert(p.Debtor),
        Beneficiary: privconv.Convert(p.Beneficiary),
        Direction:   publicapi.Direction(p.Direction),
        Status:      publicapi.PaymentStatus(p.Status),
        Gateway:     publicapi.Gateway(p.Gateway),
        ExternalId: publicapi.OptString{
            Value: p.ExternalId.Value,
            Set:   p.ExternalId.IsSet(),
        },
        CreatedAt: p.CreatedAt,
        UpdatedAt: p.UpdatedAt,
    }
}
