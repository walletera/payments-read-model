package app

import (
    "context"
    "fmt"
    "log/slog"
    "time"

    "payments-read-model/internal/adapters/mongodb"
    "payments-read-model/internal/domain/payments"
    "payments-read-model/pkg/logattr"

    "github.com/walletera/eventskit/messages"
    "github.com/walletera/eventskit/rabbitmq"
    paymentsevents "github.com/walletera/payments-types/events"
    "github.com/walletera/werrors"
    "go.mongodb.org/mongo-driver/v2/mongo"
    "go.mongodb.org/mongo-driver/v2/mongo/options"
    "go.uber.org/zap"
    "go.uber.org/zap/exp/zapslog"
    "go.uber.org/zap/zapcore"
)

const (
    RabbitMQPaymentsExchangeName     = "payments.events"
    RabbitMQExchangeType             = "topic"
    RabbitMQPaymentCreatedRoutingKey = "payment.created"
    RabbitMQQueueName                = "payments-read-model"
)

type App struct {
    rabbitmqHost     string
    rabbitmqPort     int
    rabbitmqUser     string
    rabbitmqPassword string
    mongoClient      *mongo.Client
    logHandler       slog.Handler
    logger           *slog.Logger
}

func NewApp(opts ...Option) (*App, error) {
    app := &App{}
    err := setDefaultOpts(app)
    if err != nil {
        return nil, fmt.Errorf("failed setting default options: %w", err)
    }
    for _, opt := range opts {
        opt(app)
    }
    return app, nil
}

func (app *App) Run(ctx context.Context) error {
    app.logger = slog.
        New(app.logHandler).
        With(logattr.ServiceName("payments-read-model"))

    app.logger.Info("payments-read-model started")

    processor, err := createPaymentsMessageProcessor(app)
    if err != nil {
        return fmt.Errorf("error creating payments message processor: %w", err)
    }

    err = processor.Start(ctx)
    if err != nil {
        return fmt.Errorf("error starting payments message processor: %w", err)
    }

    return nil
}

func (app *App) Stop(ctx context.Context) {
    // TODO implement processor gracefully shutdown
    err := app.mongoClient.Disconnect(context.TODO())
    if err != nil {
        app.logger.Error("error disconnecting from mongo", logattr.Error(err.Error()))
    }
    app.logger.Info("payments-read-model stopped")
}

func setDefaultOpts(app *App) error {
    zapLogger, err := newZapLogger()
    if err != nil {
        return err
    }
    app.logHandler = zapslog.NewHandler(zapLogger.Core())
    return nil
}

func newZapLogger() (*zap.Logger, error) {
    encoderConfig := zap.NewProductionEncoderConfig()
    encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.RFC3339)
    zapConfig := zap.Config{
        Level:             zap.NewAtomicLevelAt(zap.DebugLevel),
        Development:       false,
        DisableStacktrace: true,
        Sampling: &zap.SamplingConfig{
            Initial:    100,
            Thereafter: 100,
        },
        Encoding:         "json",
        EncoderConfig:    encoderConfig,
        OutputPaths:      []string{"stderr"},
        ErrorOutputPaths: []string{"stderr"},
    }
    return zapConfig.Build()
}

func createPaymentsMessageProcessor(app *App) (*messages.Processor[paymentsevents.Handler], error) {
    queueName := fmt.Sprintf(RabbitMQQueueName)

    rabbitMQClient, err := rabbitmq.NewClient(
        rabbitmq.WithHost(app.rabbitmqHost),
        rabbitmq.WithPort(uint(app.rabbitmqPort)),
        rabbitmq.WithUser(app.rabbitmqUser),
        rabbitmq.WithPassword(app.rabbitmqPassword),
        rabbitmq.WithExchangeName(RabbitMQPaymentsExchangeName),
        rabbitmq.WithExchangeType(RabbitMQExchangeType),
        rabbitmq.WithConsumerRoutingKeys(RabbitMQPaymentCreatedRoutingKey),
        rabbitmq.WithQueueName(queueName),
    )
    if err != nil {
        return nil, fmt.Errorf("creating rabbitmq client: %w", err)
    }

    MongodbUri := "mongodb://localhost:27017/?retryWrites=true&w=majority"

    // Use the SetServerAPIOptions() method to set the Stable API version to 1
    serverAPI := options.ServerAPI(options.ServerAPIVersion1)
    opts := options.Client().ApplyURI(MongodbUri).SetServerAPIOptions(serverAPI)

    // Create a new client and connect to the server
    client, err := mongo.Connect(opts)
    if err != nil {
        return nil, fmt.Errorf("error connecting to mongodb: %w", err)
    }
    app.mongoClient = client

    repository := mongodb.NewPaymentsRepository(client, "payments", "payments")
    paymentEventsHandler := payments.NewEventsHandler(repository, app.logger.With(logattr.Component("payments.events.Handler")))

    paymentsMessageProcessor := messages.NewProcessor[paymentsevents.Handler](
        rabbitMQClient,
        paymentsevents.NewDeserializer(app.logger),
        paymentEventsHandler,
        withErrorCallback(
            app.logger.With(
                logattr.Component("payments.rabbitmq.MessageProcessor")),
        ),
    )

    return paymentsMessageProcessor, nil
}

func withErrorCallback(logger *slog.Logger) messages.ProcessorOpt {
    return messages.WithErrorCallback(func(wError werrors.WError) {
        logger.Error(
            "failed processing message",
            logattr.Error(wError.Message()))
    })
}
