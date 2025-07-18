package logattr

import "log/slog"

func ServiceName(serviceName string) slog.Attr {
    return slog.String("service_name", serviceName)
}

func Component(component string) slog.Attr {
    return slog.String("component", component)
}

func PaymentId(paymentId string) slog.Attr {
    return slog.String("payment_id", paymentId)
}

func BindOperationId(bindOperationId string) slog.Attr {
    return slog.String("bind_operation_id", bindOperationId)
}

func BindStatus(bindStatus int) slog.Attr {
    return slog.Int("bind_status", bindStatus)
}

func EventType(eventType string) slog.Attr {
    return slog.String("event_type", eventType)
}

func Error(err string) slog.Attr {
    return slog.String("error", err)
}

func CorrelationId(correlationId string) slog.Attr {
    return slog.String("correlation_id", correlationId)
}

func StreamName(streamName string) slog.Attr {
    return slog.String("stream_name", streamName)
}
