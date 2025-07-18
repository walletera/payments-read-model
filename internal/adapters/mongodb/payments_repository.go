package mongodb

import (
    "context"

    "payments-read-model/internal/domain/payments"

    "github.com/google/uuid"
    "github.com/walletera/payments-types/privateapi"
    "github.com/walletera/werrors"
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
)

type PaymentBSON struct {
    ID               uuid.UUID          `bson:"_id"`
    AggregateVersion uint64             `bson:"version"`
    Data             privateapi.Payment `bson:"data"`
}

type PaymentsRepository struct {
    client         *mongo.Client
    dbName         string
    collectionName string
}

func NewPaymentsRepository(client *mongo.Client, dbName string, collectionName string) *PaymentsRepository {
    return &PaymentsRepository{client: client, dbName: dbName, collectionName: collectionName}
}

func (p *PaymentsRepository) GetPayment(ctx context.Context, id uuid.UUID) (payments.Payment, werrors.WError) {
    //TODO implement me
    panic("implement me")
}

func (p *PaymentsRepository) SavePayment(ctx context.Context, payment payments.Payment) werrors.WError {
    paymentBSON := PaymentBSON(payment)
    coll := p.client.Database(p.dbName).Collection(p.collectionName)
    _, err := coll.InsertOne(ctx, paymentBSON)
    if err != nil {
        if mongo.IsDuplicateKeyError(err) {
            return werrors.NewNonRetryableInternalError("duplicate key error: %s", err.Error())
        }
        return werrors.NewRetryableInternalError("failed to save payment: %s", err.Error())
    }
    return nil
}

func (p *PaymentsRepository) UpdatePayment(ctx context.Context, paymentUpdate payments.PaymentUpdate) werrors.WError {
    if paymentUpdate.AggregateVersion == 0 {
        return werrors.NewNonRetryableInternalError("aggregate version cannot be 0 for a payment update")
    }

    update := bson.M{
        "version":     paymentUpdate.AggregateVersion,
        "data.status": paymentUpdate.Status,
    }

    if paymentUpdate.ExternalId.IsSet() {
        update["data.externalId"] = paymentUpdate.ExternalId
    }

    coll := p.client.Database(p.dbName).Collection(p.collectionName)
    updateResult, err := coll.UpdateOne(ctx, bson.M{
        "_id":     paymentUpdate.PaymentId,
        "version": paymentUpdate.AggregateVersion - 1,
    },
        bson.M{
            "$set": update,
        })

    if err != nil {
        return werrors.NewRetryableInternalError("failed to update payment: %s", err.Error())
    }

    if updateResult.MatchedCount == 0 {
        return checkVersion(ctx, coll, paymentUpdate)
    }

    return nil
}

func (p *PaymentsRepository) SearchPayments(ctx context.Context, query string) (payments.QueryResult, werrors.WError) {
    //TODO implement me
    panic("implement me")
}

func checkVersion(ctx context.Context, coll *mongo.Collection, paymentUpdate payments.PaymentUpdate) werrors.WError {
    result := coll.FindOne(
        ctx,
        bson.M{
            "_id": paymentUpdate.PaymentId,
        },
    )
    if resultErr := result.Err(); resultErr != nil {
        return werrors.NewRetryableInternalError("failed finding payment with id: %s", paymentUpdate.PaymentId)
    }
    var retrievedPayment PaymentBSON
    decodeErr := result.Decode(&retrievedPayment)
    if decodeErr != nil {
        return werrors.NewNonRetryableInternalError("failed decoding mongodb result: %s", decodeErr.Error())
    }
    expectedUpdateVersion := retrievedPayment.AggregateVersion + 1
    if paymentUpdate.AggregateVersion < expectedUpdateVersion {
        return werrors.NewNonRetryableInternalError("version mismatch: update version %d, expected version %d - sending event to dlq", expectedUpdateVersion, paymentUpdate.AggregateVersion)
    } else { // paymentUpdate.AggregateVersion >= expectedUpdateVersion
        return werrors.NewRetryableInternalError("gap detected between update version %d and expected version %d, retrying...", retrievedPayment.AggregateVersion, expectedUpdateVersion)
    }
}
