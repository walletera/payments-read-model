package mongodb

import (
    "context"

    "github.com/walletera/payments-read-model/internal/domain/payments"

    "go.mongodb.org/mongo-driver/v2/mongo"
)

type Iterator struct {
    cursor *mongo.Cursor
}

func (m *Iterator) Next() (bool, payments.Payment, error) {
    if !m.cursor.Next(context.Background()) {
        if err := m.cursor.Err(); err != nil {
            return false, payments.Payment{}, err
        }
        return false, payments.Payment{}, nil
    }

    var paymentBSON PaymentBSON
    if err := m.cursor.Decode(&paymentBSON); err != nil {
        return false, payments.Payment{}, err
    }

    payment := payments.Payment{
        ID:               paymentBSON.ID,
        AggregateVersion: paymentBSON.AggregateVersion,
        Data:             paymentBSON.Data,
    }
    return true, payment, nil
}
