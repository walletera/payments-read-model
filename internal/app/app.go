package app

import (
    "context"
    "errors"
    "fmt"
    "log/slog"
    "net/http"
    "time"

    "github.com/walletera/payments-read-model/internal/adapters/input/http/public"
    "github.com/walletera/payments-read-model/internal/adapters/mongodb"
    "github.com/walletera/payments-read-model/internal/domain/payments"
    "github.com/walletera/payments-read-model/pkg/logattr"

    "github.com/walletera/eventskit/messages"
    "github.com/walletera/eventskit/rabbitmq"
    paymentsevents "github.com/walletera/payments-types/events"
    "github.com/walletera/payments-types/publicapi"
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
    rabbitmqHost      string
    rabbitmqPort      int
    rabbitmqUser      string
    rabbitmqPassword  string
    mongodbURL        string
    mongoClient       *mongo.Client
    publicAPIConfig   Optional[PublicAPIConfig]
    logHandler        slog.Handler
    logger            *slog.Logger
    httpServersToStop []*http.Server
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

    var httpServersToStop []*http.Server

    var publicApiHttpServer *http.Server
    if app.publicAPIConfig.Set {
        publicApiHttpServer, err = app.startPublicAPIHTTPServer(app.logger)
        if err != nil {
            return fmt.Errorf("failed starting public api http server: %w", err)
        }

        httpServersToStop = append(httpServersToStop, publicApiHttpServer)
    }
    app.httpServersToStop = httpServersToStop

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
    for _, httpServer := range app.httpServersToStop {
        err := httpServer.Shutdown(ctx)
        if err != nil {
            app.logger.Error("error stopping http server", logattr.Error(err.Error()))
        }
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

    // Use the SetServerAPIOptions() method to set the Stable API version to 1
    serverAPI := options.ServerAPI(options.ServerAPIVersion1)
    opts := options.Client().ApplyURI(app.mongodbURL).SetServerAPIOptions(serverAPI)

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

func (app *App) startPublicAPIHTTPServer(appLogger *slog.Logger) (*http.Server, error) {
    repository := mongodb.NewPaymentsRepository(app.mongoClient, "payments", "payments")

    server, err := publicapi.NewServer(
        public.NewHandler(
            repository,
            appLogger.With(logattr.Component("http.PublicAPIHandler")),
        ),
        &public.SecurityHandler{},
    )
    if err != nil {
        panic(err)
    }
    httpServer := &http.Server{
        Addr:    fmt.Sprintf("0.0.0.0:%d", app.publicAPIConfig.Value.PublicAPIHttpServerPort),
        Handler: server,
    }

    go func() {
        defer appLogger.Info("http server stopped")
        if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            appLogger.Error("http server error", logattr.Error(err.Error()))
        }
    }()

    appLogger.Info("http server started")

    return httpServer, nil
}
