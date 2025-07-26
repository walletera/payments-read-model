package mongodb

import (
    "context"

    "payments-read-model/internal/domain/payments"

    "github.com/google/uuid"
    "github.com/walletera/payments-types/privateapi"
    "github.com/walletera/payments-types/publicapi"
    "github.com/walletera/werrors"
    "go.mongodb.org/mongo-driver/v2/bson"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
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

func (p *PaymentsRepository) SearchPayments(ctx context.Context, listPaymentsParams publicapi.ListPaymentsParams) (payments.QueryResult, werrors.WError) {
    filter := bson.M{}

    if listPaymentsParams.ID.IsSet() {
        filter["_id"] = listPaymentsParams.ID.Value
    }
    if listPaymentsParams.CustomerId.IsSet() {
        filter["data.customerId"] = listPaymentsParams.CustomerId.Value
    }
    if listPaymentsParams.Status.IsSet() {
        filter["data.status"] = listPaymentsParams.Status.Value
    }
    if listPaymentsParams.Gateway.IsSet() {
        filter["data.gateway"] = listPaymentsParams.Gateway.Value
    }
    if listPaymentsParams.ExternalId.IsSet() {
        filter["data.externalId"] = listPaymentsParams.ExternalId.Value
    }
    if listPaymentsParams.SchemeId.IsSet() {
        filter["data.schemeId"] = listPaymentsParams.SchemeId.Value
    }
    if listPaymentsParams.DateFrom.IsSet() || listPaymentsParams.DateTo.IsSet() {
        dateFilter := bson.M{}
        if listPaymentsParams.DateFrom.IsSet() {
            dateFilter["$gte"] = listPaymentsParams.DateFrom.Value
        }
        if listPaymentsParams.DateTo.IsSet() {
            dateFilter["$lte"] = listPaymentsParams.DateTo.Value
        }
        filter["data.createdAt"] = dateFilter
    }

    coll := p.client.Database(p.dbName).Collection(p.collectionName)

    total, err := coll.CountDocuments(ctx, filter)
    if err != nil {
        return payments.QueryResult{}, werrors.NewRetryableInternalError("failed to count payments: %s", err.Error())
    }

    sort := bson.D{{"createdAt", -1}, {"_id", -1}}
    findOpts := options.Find().SetSort(sort)

    limit := int64(50)
    if listPaymentsParams.Limit.IsSet() {
        limit = int64(listPaymentsParams.Limit.Value)
        findOpts.SetLimit(limit)
    }

    offset := int64(0)
    if listPaymentsParams.Offset.IsSet() {
        offset = int64(listPaymentsParams.Offset.Value)
        findOpts.SetSkip(offset)
    }

    cursor, err := coll.Find(ctx, filter, findOpts)
    if err != nil {
        return payments.QueryResult{}, werrors.NewRetryableInternalError("failed to find payments: %s", err.Error())
    }

    iterator := &Iterator{cursor: cursor}
    return payments.QueryResult{
        Iterator: iterator,
        Total:    uint64(total),
    }, nil
}
