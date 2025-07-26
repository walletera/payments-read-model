package public

import (
    "context"

    api "github.com/walletera/payments-types/publicapi"
)

type SecurityHandler struct {
    //pubKey *rsa.PublicKey
}

//func NewSecurityHandler(pubKey *rsa.PublicKey) *SecurityHandler {
//    return &SecurityHandler{
//        pubKey: pubKey,
//    }
//}

func (s *SecurityHandler) HandleBearerAuth(ctx context.Context, operationName api.OperationName, t api.BearerAuth) (context.Context, error) {
    //wjwt, err := auth.ParseAndValidate(t.GetToken(), s.pubKey)
    //if err != nil {
    //    return nil, err
    //}
    //if len(wjwt.UID) == 0 {
    //    return nil, fmt.Errorf("uid is missing")
    //}
    //if wjwt.State != "active" {
    //    return nil, fmt.Errorf("customer is not active")
    //}
    return ctx, nil
}
