package tests

import (
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
)

var mongodbClient *mongo.Client

func getMongodbClient() (*mongo.Client, error) {
    if mongodbClient != nil {
        return mongodbClient, nil
    }

    MongodbUri := "mongodb://localhost:27017/?retryWrites=true&w=majority"

    // Use the SetServerAPIOptions() method to set the Stable API version to 1
    serverAPI := options.ServerAPI(options.ServerAPIVersion1)
    opts := options.Client().ApplyURI(MongodbUri).SetServerAPIOptions(serverAPI)

    // Create a new client and connect to the server
    mongodbClient, err := mongo.Connect(opts)
    if err != nil {
        return nil, err
    }

    return mongodbClient, nil
}
