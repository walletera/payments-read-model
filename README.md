# Payments Read Model
[![Go](https://github.com/walletera/payments-read-model/actions/workflows/go.yml/badge.svg)](https://github.com/walletera/eventskit/actions/workflows/go.yml)

## Overview
**Payments Read Model** is a Go application designed to build and maintain a MongoDB-based read-model for payment data. It consumes events from a RabbitMQ event stream, processes event-based updates using event sourcing patterns, and persists the current state of payments in a form optimized for querying.
The payment events consumed by this service are produced by the [Payments Service](https://github.com/walletera/payments). This decouples state reading from write-side complexity and leverages event sourcing to serve accurate, real-time payment projections.
The application is structured using clean architecture principles, separating event ingestion, domain logic, and persistence. This empowers services or APIs to serve payment data efficiently from an up-to-date projection, decoupled from the core event stream.
## Features
- **Event-driven**: Consumes payment-related events (such as creation and update) from RabbitMQ, produced by the [Payments Service](https://github.com/walletera/payments).
- **MongoDB-backed**: Builds and maintains a robust MongoDB-based projection of payment entities.
- **Idempotent Updates & Consistency**: Uses optimistic concurrency control to handle versioning and ensure consistency.
- **Structured Logging**: Thread-safe, structured logging for operational clarity, using zap and slog.
- **Extensible & Modular**: Components are loosely coupled for testability and ease of extension.

## High-Level Architecture
1. **Event Processing**: Listens to a RabbitMQ topic exchange for payment events produced by the [Payments Service](https://github.com/walletera/payments).
2. **Domain Handling**: Decodes and processes these events, applying domain rules and constructing up-to-date payment states.
3. **Persistence Layer**: Writes or updates the current payment state into MongoDB, using version checks for correctness.
4. **Logging & Error Handling**: Comprehensive logging to observe healthiness and troubleshoot issues.

## Getting Started
### Prerequisites
- Go 1.23+
- MongoDB (with access to a suitable user/database/collection)
- RabbitMQ (with access to the relevant exchanges and queues)

### Environment Variables / Configuration
Set your connection details and other settings (e.g., host, port, credentials) through environment variables, configuration files, or command-line flags as supported by your deployment environment.
- `RABBITMQ_HOST`
- `RABBITMQ_PORT`
- `RABBITMQ_USER`
- `RABBITMQ_PASSWORD`
- `MONGODB_URI` _(usually defaults to in code)`mongodb://localhost:27017/?retryWrites=true&w=majority`_

(The precise configuration mechanism and environment integration may depend on your deployment; consult configuration code or add your own flag/env parsing if needed.)
### Running the Service
1. **Dependency setup:** Ensure RabbitMQ and MongoDB are running and accessible.
2. **Start the application:**
``` bash
   go run ./cmd/your-main-entry.go
```
1. **Shutdown:**
   The application is designed to handle shutdown signals gracefully and clean up resources (MongoDB connections, etc.).

## Extending
- **Supporting more events**: Extend the event handler logic for new event types.
- **Read APIs**: Build REST/gRPC endpoints atop the read-model MongoDB collections for consumption by other systems or UIs.
- **Configuration**: Add a configuration loader or environment parser as needed for your deployment platform.

## Project Structure
- : Implementation details for external systems (e.g., MongoDB). `internal/adapters/`
- : Domain logic, event handlers. `internal/domain/`
- `pkg/logattr/`: Logging attribute helpers.

## Observability
All important operations are logged. Errors and failures (e.g., version mismatch, persistence failures) are logged at appropriate severity levels and include relevant identifiers for diagnosis.
