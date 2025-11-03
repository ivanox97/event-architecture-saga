package externalpayment

import (
	"context"
	"testing"
	"time"

	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"
	"event-saga/internal/infrastructure/dlq"
	"event-saga/internal/infrastructure/eventbus"
	gatewaymock "event-saga/internal/infrastructure/mock"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockEventStore struct {
	mock.Mock
}

func (m *MockEventStore) SaveEvent(ctx context.Context, event events.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockEventStore) LoadEvents(ctx context.Context, aggregateID string) ([]events.Event, error) {
	args := m.Called(ctx, aggregateID)
	return args.Get(0).([]events.Event), args.Error(1)
}

func (m *MockEventStore) Close() error {
	return nil
}

type MockEventBus struct {
	mock.Mock
}

func (m *MockEventBus) Publish(ctx context.Context, topic string, event events.Event) error {
	args := m.Called(ctx, topic, event)
	return args.Error(0)
}

func (m *MockEventBus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler) error {
	return nil
}

func (m *MockEventBus) SubscribeWithGroupID(ctx context.Context, topic, groupID string, handler eventbus.EventHandler) error {
	return nil
}

func (m *MockEventBus) Close() error {
	return nil
}

type MockDLQ struct {
	mock.Mock
}

func (m *MockDLQ) Publish(ctx context.Context, originalEvent events.Event, failureReason string, consumerGroup, originalTopic string, originalPartition int, originalOffset int64) error {
	args := m.Called(ctx, originalEvent, failureReason, consumerGroup, originalTopic, originalPartition, originalOffset)
	return args.Error(0)
}

func (m *MockDLQ) Subscribe(ctx context.Context, handler dlq.DLQHandler) error {
	return nil
}
func (m *MockDLQ) GetEvents() []dlq.DLQEvent {
	return nil
}
func (m *MockDLQ) Close() error {
	return nil
}

type MockExternalGatewayWrapper struct {
	gatewaymock.ExternalGateway
	timeoutSimulation    bool
	successAfterAttempts int
	currentAttempt       int
	shouldTimeout        bool
}

func (m *MockExternalGatewayWrapper) ProcessPayment(ctx context.Context, req gatewaymock.PaymentRequest) (*gatewaymock.GatewayResponse, error) {
	m.currentAttempt++

	if m.successAfterAttempts > 0 && m.currentAttempt >= m.successAfterAttempts {
		return &gatewaymock.GatewayResponse{
			GatewayPaymentID: "gateway_" + req.PaymentID,
			Status:           "SUCCESS",
			TransactionID:    "txn_" + uuid.New().String(),
		}, nil
	}

	if m.shouldTimeout || m.timeoutSimulation {
		// Wait for context to timeout
		<-ctx.Done()
		return nil, ctx.Err()
	}

	return nil, context.DeadlineExceeded
}

func TestExternalPaymentService_HandleExternalPaymentRequested_HappyPath(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockDLQ := new(MockDLQ)
	mockLogger := logger.NewMockLogger()

	// Create gateway that succeeds immediately
	mockGateway := &MockExternalGatewayWrapper{
		successAfterAttempts: 1,
		shouldTimeout:        false,
	}

	service := NewService(mockEventStore, mockEventBus, mockDLQ, mockGateway, mockLogger)

	ctx := context.Background()
	paymentID := "pay_abc999"
	sagaID := "saga_456"
	userID := "user_123"
	amount := 2000.0

	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	externalRequestEvent := events.NewExternalPaymentRequested(
		paymentID,
		sagaID,
		userID,
		"svc_789",
		amount,
		"USD",
		"card_token_xyz",
		metadata,
		1004,
	)

	// Mock SaveEvent for PaymentSentToGateway
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentSentToGateway"
	})).Return(nil).Once()

	// Mock Publish for PaymentSentToGateway
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentSentToGateway"
	})).Return(nil).Once()

	// Mock SaveEvent for PaymentGatewayResponse (simulated webhook - happens asynchronously)
	// Note: This happens in a goroutine, so we might need to wait a bit
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentGatewayResponse" && e.Data().(events.PaymentGatewayResponseData).Status == "SUCCESS"
	})).Return(nil).Maybe()

	// Mock Publish for PaymentGatewayResponse
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentGatewayResponse" && e.Data().(events.PaymentGatewayResponseData).Status == "SUCCESS"
	})).Return(nil).Maybe()

	// Execute
	err := service.HandleExternalPaymentRequested(ctx, externalRequestEvent)

	// Wait a bit for async webhook simulation
	time.Sleep(200 * time.Millisecond)

	// Assertions
	assert.NoError(t, err)
	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestExternalPaymentService_HandleExternalPaymentRequested_TimeoutWithRetries(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockDLQ := new(MockDLQ)
	mockLogger := logger.NewMockLogger()

	// Create gateway that simulates timeouts
	mockGateway := &MockExternalGatewayWrapper{
		timeoutSimulation: true,
		shouldTimeout:     true,
	}

	service := NewService(mockEventStore, mockEventBus, mockDLQ, mockGateway, mockLogger)
	// Override retry policy for faster testing
	service.retryPolicy = RetryPolicy{
		MaxAttempts:  3, // Reduced for testing
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}
	service.timeout = 50 * time.Millisecond // Short timeout for testing

	ctx := context.Background()
	paymentID := "pay_timeout_001"
	sagaID := "saga_timeout_001"
	userID := "user_123"
	amount := 2000.0

	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	externalRequestEvent := events.NewExternalPaymentRequested(
		paymentID,
		sagaID,
		userID,
		"svc_789",
		amount,
		"USD",
		"card_token_xyz",
		metadata,
		1004,
	)

	// Gateway will timeout on all attempts (configured via shouldTimeout)

	// Mock SaveEvent for PaymentGatewayTimeout events (should be called for each timeout)
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentGatewayTimeout"
	})).Return(nil).Times(3)

	// Mock Publish for PaymentGatewayTimeout events
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentGatewayTimeout"
	})).Return(nil).Times(3)

	// Mock SaveEvent for PaymentRetryRequested events
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentRetryRequested"
	})).Return(nil).Times(2) // 2 retries (after attempts 1 and 2)

	// Mock Publish for PaymentRetryRequested events
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentRetryRequested"
	})).Return(nil).Times(2)

	// Mock SaveEvent for ExternalPaymentFailed (after max retries)
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "ExternalPaymentFailed"
	})).Return(nil).Once()

	// Mock Publish for ExternalPaymentFailed
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "ExternalPaymentFailed"
	})).Return(nil).Once()

	// Mock DLQ Publish (should be called when max retries exceeded)
	mockDLQ.On("Publish", ctx, mock.Anything, "MAX_RETRIES_EXCEEDED", configs.ServiceNameExternalPaymentService, configs.TopicPayments, 0, 0).Return(nil).Once()

	// Execute
	err := service.HandleExternalPaymentRequested(ctx, externalRequestEvent)

	// Assertions - should complete without error (failure is handled via events)
	assert.NoError(t, err)

	// Verify DLQ was called
	mockDLQ.AssertCalled(t, "Publish", ctx, mock.Anything, "MAX_RETRIES_EXCEEDED", configs.ServiceNameExternalPaymentService, configs.TopicPayments, 0, 0)

	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}

func TestExternalPaymentService_HandleExternalPaymentRequested_SuccessAfterRetry(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockDLQ := new(MockDLQ)
	mockLogger := logger.NewMockLogger()

	// Create gateway that succeeds after 2 attempts
	mockGateway := &MockExternalGatewayWrapper{
		timeoutSimulation:    false,
		successAfterAttempts: 2,
		shouldTimeout:        true, // First attempt times out
	}

	service := NewService(mockEventStore, mockEventBus, mockDLQ, mockGateway, mockLogger)
	// Override retry policy for faster testing
	service.retryPolicy = RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 10 * time.Millisecond,
		MaxDelay:     100 * time.Millisecond,
		Multiplier:   2.0,
		Jitter:       false,
	}
	service.timeout = 50 * time.Millisecond

	ctx := context.Background()
	paymentID := "pay_retry_success"
	sagaID := "saga_retry_success"
	userID := "user_123"
	amount := 2000.0

	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	externalRequestEvent := events.NewExternalPaymentRequested(
		paymentID,
		sagaID,
		userID,
		"svc_789",
		amount,
		"USD",
		"card_token_xyz",
		metadata,
		1004,
	)

	// Gateway configured to succeed after 2 attempts (first times out, second succeeds)

	// Mock SaveEvent for PaymentGatewayTimeout (attempt 1)
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentGatewayTimeout"
	})).Return(nil).Once()

	// Mock Publish for PaymentGatewayTimeout
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentGatewayTimeout"
	})).Return(nil).Once()

	// Mock SaveEvent for PaymentRetryRequested
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentRetryRequested"
	})).Return(nil).Once()

	// Mock Publish for PaymentRetryRequested
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentRetryRequested"
	})).Return(nil).Once()

	// Mock SaveEvent for PaymentSentToGateway (success)
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentSentToGateway"
	})).Return(nil).Once()

	// Mock Publish for PaymentSentToGateway
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentSentToGateway"
	})).Return(nil).Once()

	// Mock SaveEvent for PaymentGatewayResponse (SUCCESS)
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentGatewayResponse" && e.Data().(events.PaymentGatewayResponseData).Status == "SUCCESS"
	})).Return(nil).Once()

	// Mock Publish for PaymentGatewayResponse
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "PaymentGatewayResponse" && e.Data().(events.PaymentGatewayResponseData).Status == "SUCCESS"
	})).Return(nil).Once()

	// Execute
	err := service.HandleExternalPaymentRequested(ctx, externalRequestEvent)

	// Wait for async webhook
	time.Sleep(200 * time.Millisecond)

	// Assertions
	assert.NoError(t, err)
	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)

	// Verify DLQ was NOT called (payment succeeded)
	mockDLQ.AssertNotCalled(t, "Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}
