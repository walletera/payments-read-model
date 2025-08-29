package app

import "log/slog"

type Option func(app *App)

func WithPublicAPIConfig(config PublicAPIConfig) func(a *App) {
    return func(a *App) {
        a.publicAPIConfig = NewOptional[PublicAPIConfig](config)
    }
}

func WithRabbitmqHost(host string) func(a *App) { return func(a *App) { a.rabbitmqHost = host } }

func WithRabbitmqPort(port int) func(a *App) { return func(a *App) { a.rabbitmqPort = port } }

func WithRabbitmqUser(user string) func(a *App) { return func(a *App) { a.rabbitmqUser = user } }

func WithRabbitmqPassword(password string) func(a *App) {
    return func(a *App) { a.rabbitmqPassword = password }
}

func WithMongoDBURL(url string) func(a *App) { return func(a *App) { a.mongodbURL = url } }

func WithLogHandler(handler slog.Handler) func(app *App) {
    return func(app *App) { app.logHandler = handler }
}
