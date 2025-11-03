package metrics

import (
	"context"

	"event-saga/internal/common/logger"
	commonmetrics "event-saga/internal/common/metrics"
	"event-saga/internal/domain/events"
	"event-saga/internal/infrastructure/dlq"
	"event-saga/internal/infrastructure/errors"
	"event-saga/internal/infrastructure/eventbus"
)

// Service handles metrics collection and DLQ processing
type Service struct {
	eventBus eventbus.EventBus
	dlq      dlq.DLQ
	dbErrors *errors.DBErrors
	metrics  commonmetrics.Collector
	logger   logger.Logger
}

func NewService(eb eventbus.EventBus, d dlq.DLQ, dbErrors *errors.DBErrors, m commonmetrics.Collector, l logger.Logger) *Service {
	return &Service{
		eventBus: eb,
		dlq:      d,
		dbErrors: dbErrors,
		metrics:  m,
		logger:   l,
	}
}

func (s *Service) HandleWalletPaymentCompleted(ctx context.Context, event events.Event) error {
	s.metrics.IncrementCounter("wallet_payments_completed_total")
	s.logger.Info("Wallet payment completed", logger.Field{Key: "event_type", Value: event.Type()})
	return nil
}

func (s *Service) HandleWalletPaymentFailed(ctx context.Context, event events.Event) error {
	s.metrics.IncrementCounter("wallet_payments_failed_total")
	s.logger.Info("Wallet payment failed", logger.Field{Key: "event_type", Value: event.Type()})
	return nil
}

func (s *Service) HandleExternalPaymentCompleted(ctx context.Context, event events.Event) error {
	s.metrics.IncrementCounter("external_payments_completed_total")
	s.logger.Info("External payment completed", logger.Field{Key: "event_type", Value: event.Type()})
	return nil
}

func (s *Service) HandleExternalPaymentFailed(ctx context.Context, event events.Event) error {
	s.metrics.IncrementCounter("external_payments_failed_total")
	s.logger.Info("External payment failed", logger.Field{Key: "event_type", Value: event.Type()})
	return nil
}

func (s *Service) HandleWalletPaymentRequested(ctx context.Context, event events.Event) error {
	s.metrics.IncrementCounter("wallet_payments_created_total")
	return nil
}

func (s *Service) HandleExternalPaymentRequested(ctx context.Context, event events.Event) error {
	s.metrics.IncrementCounter("external_payments_created_total")
	return nil
}

func (s *Service) HandleDLQEvent(ctx context.Context, dlqEvent dlq.DLQEvent) error {
	s.metrics.IncrementCounter("dlq_events_total")
	s.logger.Warn("Processing DLQ event", logger.Field{Key: "dlq_event_id", Value: dlqEvent.DLQEventID}, logger.Field{Key: "failure_reason", Value: dlqEvent.FailureReason})

	if s.dbErrors != nil {
		if err := s.dbErrors.PersistDLQEvent(ctx, dlqEvent); err != nil {
			s.logger.Error("Failed to persist DLQ event to DB Errors", logger.Field{Key: "error", Value: err}, logger.Field{Key: "dlq_event_id", Value: dlqEvent.DLQEventID})
		} else {
			s.logger.Info("DLQ event persisted to error_logs", logger.Field{Key: "dlq_event_id", Value: dlqEvent.DLQEventID}, logger.Field{Key: "payment_id", Value: extractPaymentID(dlqEvent.OriginalEvent)})
		}
	}

	return nil
}

func extractPaymentID(event events.Event) string {
	switch data := event.Data().(type) {
	case events.WalletPaymentRequestedData:
		return data.PaymentID
	case events.ExternalPaymentRequestedData:
		return data.PaymentID
	case events.WalletPaymentFailedData:
		return data.PaymentID
	case events.ExternalPaymentFailedData:
		return data.PaymentID
	default:
		return event.AggregateID()
	}
}
