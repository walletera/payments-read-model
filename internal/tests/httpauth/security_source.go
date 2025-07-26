package httpauth

import (
    "context"

    api "github.com/walletera/payments-types/publicapi"
)

type SecuritySource struct {
    token string
}

func NewSecuritySource(token string) *SecuritySource {
    return &SecuritySource{token: token}
}

func (s *SecuritySource) BearerAuth(ctx context.Context, operationName api.OperationName) (api.BearerAuth, error) {
    return api.BearerAuth{Token: s.token}, nil
}
