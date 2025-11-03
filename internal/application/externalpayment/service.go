package externalpayment

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"
	"event-saga/internal/infrastructure/dlq"
	"event-saga/internal/infrastructure/eventbus"
	"event-saga/internal/infrastructure/eventstore"
	"event-saga/internal/infrastructure/mock"
)

// RetryPolicy defines the retry policy for gateway calls
type RetryPolicy struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	Jitter       bool
}

// DefaultRetryPolicy returns the default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:  5,
		InitialDelay: 5 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   2.0,
		Jitter:       true,
	}
}

type Service struct {
	eventStore  eventstore.EventStore
	eventBus    eventbus.EventBus
	dlq         dlq.DLQ
	gateway     mock.ExternalGateway
	logger      logger.Logger
	sequence    int64
	retryPolicy RetryPolicy
	timeout     time.Duration
}

func NewService(es eventstore.EventStore, eb eventbus.EventBus, d dlq.DLQ, g mock.ExternalGateway, l logger.Logger) *Service {
	return &Service{
		eventStore:  es,
		eventBus:    eb,
		dlq:         d,
		gateway:     g,
		logger:      l,
		sequence:    0,
		retryPolicy: DefaultRetryPolicy(),
		timeout:     30 * time.Second,
	}
}

func (s *Service) HandleExternalPaymentRequested(ctx context.Context, event events.Event) error {
	paymentData, ok := event.Data().(events.ExternalPaymentRequestedData)
	if !ok {
		return fmt.Errorf("invalid event data type, expected ExternalPaymentRequestedData")
	}

	return s.processPaymentWithRetry(ctx, paymentData, event.Metadata())
}

func (s *Service) processPaymentWithRetry(ctx context.Context, paymentData events.ExternalPaymentRequestedData, metadata events.EventMetadata) error {
	attempt := 0
	delay := s.retryPolicy.InitialDelay

	gatewayReq := mock.PaymentRequest{
		PaymentID: paymentData.PaymentID,
		Amount:    paymentData.Amount,
		Currency:  paymentData.Currency,
		CardToken: paymentData.CardToken,
	}

	for attempt < s.retryPolicy.MaxAttempts {
		attempt++

		attemptCtx, cancel := context.WithTimeout(ctx, s.timeout)
		gatewayResp, err := s.gateway.ProcessPayment(attemptCtx, gatewayReq)
		cancel()

		if err == nil {
			return s.handleSuccess(ctx, paymentData, gatewayResp, metadata)
		}

		isTimeoutErr := err == context.DeadlineExceeded || err == context.Canceled
		if isTimeoutErr {
			if err := s.publishTimeoutEvent(ctx, paymentData, attempt, metadata); err != nil {
				s.logger.Error("Failed to publish timeout event", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "error", Value: err})
			}
		} else {
			return s.handlePermanentFailure(ctx, paymentData, err.Error(), metadata)
		}

		if attempt < s.retryPolicy.MaxAttempts {
			if err := s.publishRetryRequestedEvent(ctx, paymentData, attempt, err.Error(), delay, metadata); err != nil {
				s.logger.Error("Failed to publish retry event", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "error", Value: err})
			}

			time.Sleep(delay)

			delay = time.Duration(float64(delay) * s.retryPolicy.Multiplier)
			if delay > s.retryPolicy.MaxDelay {
				delay = s.retryPolicy.MaxDelay
			}

			if s.retryPolicy.Jitter {
				jitter := time.Duration(rand.Intn(int(delay / 10)))
				delay += jitter
			}
		}
	}

	return s.handleMaxRetriesExceeded(ctx, paymentData, metadata)
}

func (s *Service) handleSuccess(ctx context.Context, paymentData events.ExternalPaymentRequestedData, gatewayResp *mock.GatewayResponse, metadata events.EventMetadata) error {
	s.sequence++
	sentEvent := events.NewPaymentSentToGateway(
		paymentData.PaymentID,
		paymentData.SagaID,
		"external",
		gatewayResp.GatewayPaymentID,
		metadata,
		s.sequence,
	)

	if err := s.eventStore.SaveEvent(ctx, sentEvent); err != nil {
		return fmt.Errorf("failed to save sent to gateway event: %w", err)
	}

	if err := s.eventBus.Publish(ctx, configs.TopicPayments, sentEvent); err != nil {
		return fmt.Errorf("failed to publish sent to gateway event: %w", err)
	}

	s.logger.Info("Payment sent to gateway", logger.Field{Key: "payment_id", Value: paymentData.PaymentID})

	go s.simulateWebhookResponse(ctx, paymentData, gatewayResp, metadata)

	return nil
}

func (s *Service) handlePermanentFailure(ctx context.Context, paymentData events.ExternalPaymentRequestedData, reason string, metadata events.EventMetadata) error {
	s.sequence++
	failedEvent := events.NewExternalPaymentFailed(
		paymentData.PaymentID,
		paymentData.SagaID,
		paymentData.UserID,
		paymentData.Amount,
		paymentData.Currency,
		reason,
		"external",
		metadata,
		s.sequence,
	)

	if err := s.eventStore.SaveEvent(ctx, failedEvent); err != nil {
		return fmt.Errorf("failed to save failure event: %w", err)
	}

	if err := s.eventBus.Publish(ctx, configs.TopicPayments, failedEvent); err != nil {
		return fmt.Errorf("failed to publish failure event: %w", err)
	}

	s.logger.Error("Payment failed permanently", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "reason", Value: reason})
	return nil
}

func (s *Service) handleMaxRetriesExceeded(ctx context.Context, paymentData events.ExternalPaymentRequestedData, metadata events.EventMetadata) error {
	reason := "MAX_RETRIES_EXCEEDED"

	s.sequence++
	failedEvent := events.NewExternalPaymentFailed(
		paymentData.PaymentID,
		paymentData.SagaID,
		paymentData.UserID,
		paymentData.Amount,
		paymentData.Currency,
		reason,
		"external",
		metadata,
		s.sequence,
	)

	if err := s.eventStore.SaveEvent(ctx, failedEvent); err != nil {
		return fmt.Errorf("failed to save max retries exceeded event: %w", err)
	}

	if err := s.eventBus.Publish(ctx, configs.TopicPayments, failedEvent); err != nil {
		return fmt.Errorf("failed to publish max retries exceeded event: %w", err)
	}

	s.logger.Error("Payment failed after max retries", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "max_attempts", Value: s.retryPolicy.MaxAttempts})

	if s.dlq != nil {
		if err := s.dlq.Publish(ctx, failedEvent, reason, configs.ServiceNameExternalPaymentService, configs.TopicPayments, 0, 0); err != nil {
			s.logger.Error("Failed to publish to DLQ", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "error", Value: err})
		} else {
			s.logger.Info("Failed payment routed to DLQ", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "reason", Value: reason})
		}
	}

	return nil
}

func (s *Service) publishTimeoutEvent(ctx context.Context, paymentData events.ExternalPaymentRequestedData, attempt int, metadata events.EventMetadata) error {
	s.sequence++
	timeoutEvent := events.NewPaymentGatewayTimeout(
		paymentData.PaymentID,
		paymentData.SagaID,
		"external",
		attempt,
		s.retryPolicy.MaxAttempts,
		int(s.timeout.Seconds()),
		metadata,
		s.sequence,
	)

	if err := s.eventStore.SaveEvent(ctx, timeoutEvent); err != nil {
		return fmt.Errorf("failed to save timeout event: %w", err)
	}

	if err := s.eventBus.Publish(ctx, configs.TopicPayments, timeoutEvent); err != nil {
		return fmt.Errorf("failed to publish timeout event: %w", err)
	}

	s.logger.Warn("Gateway timeout", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "attempt", Value: attempt})
	return nil
}

func (s *Service) publishRetryRequestedEvent(ctx context.Context, paymentData events.ExternalPaymentRequestedData, attempt int, previousError string, delay time.Duration, metadata events.EventMetadata) error {
	s.sequence++
	nextRetryAt := time.Now().Add(delay)
	retryEvent := events.NewPaymentRetryRequested(
		paymentData.PaymentID,
		paymentData.SagaID,
		attempt,
		attempt-1,
		previousError,
		nextRetryAt,
		metadata,
		s.sequence,
	)

	if err := s.eventStore.SaveEvent(ctx, retryEvent); err != nil {
		return fmt.Errorf("failed to save retry event: %w", err)
	}

	if err := s.eventBus.Publish(ctx, configs.TopicPayments, retryEvent); err != nil {
		return fmt.Errorf("failed to publish retry event: %w", err)
	}

	s.logger.Info("Retry requested", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "attempt", Value: attempt}, logger.Field{Key: "delay", Value: delay})
	return nil
}

func (s *Service) simulateWebhookResponse(ctx context.Context, paymentData events.ExternalPaymentRequestedData, gatewayResp *mock.GatewayResponse, metadata events.EventMetadata) {
	time.Sleep(200 * time.Millisecond)

	s.sequence++
	responseEvent := events.NewPaymentGatewayResponse(
		paymentData.PaymentID,
		paymentData.SagaID,
		"external",
		gatewayResp.Status,
		gatewayResp.TransactionID,
		make(map[string]interface{}),
		metadata,
		s.sequence,
	)

	if err := s.eventStore.SaveEvent(ctx, responseEvent); err != nil {
		s.logger.Error("Failed to save gateway response event", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "error", Value: err})
		return
	}

	topic := configs.TopicPayments
	if err := s.eventBus.Publish(ctx, topic, responseEvent); err != nil {
		s.logger.Error("Failed to publish gateway response event", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "error", Value: err})
		return
	}

	s.logger.Info("Webhook response received", logger.Field{Key: "payment_id", Value: paymentData.PaymentID}, logger.Field{Key: "status", Value: gatewayResp.Status})
}
