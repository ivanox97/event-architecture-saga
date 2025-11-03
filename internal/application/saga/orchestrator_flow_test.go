package saga

import (
	"context"
	"testing"
	"time"

	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOrchestrator_WalletPayment_HappyPath(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockLogger := logger.NewMockLogger()

	orchestrator := NewOrchestrator(mockEventStore, mockEventBus, mockLogger)

	ctx := context.Background()
	userID := "user_123"
	paymentID := "pay_xyz789"
	sagaID := "saga_123"
	amount := 1500.0

	// Step 1: Create wallet payment (already tested in TestOrchestrator_CreateWalletPayment)
	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}
	walletRequestEvent := events.NewWalletPaymentRequested(
		paymentID,
		sagaID,
		userID,
		"svc_456",
		amount,
		"USD",
		metadata,
		1001,
	)

	// Step 2: FundsDebited event arrives (simulating Wallet Service processing)
	fundsDebitedEvent := events.NewFundsDebited(
		paymentID,
		userID,
		amount,
		5000.0, // previous balance
		3500.0, // new balance
		"wallet",
		metadata,
		1002,
	)

	// Step 3: Mock LoadEvents to return WalletPaymentRequested (for saga reconstruction)
	mockEventStore.On("LoadEvents", ctx, paymentID).Return([]events.Event{
		walletRequestEvent,
		fundsDebitedEvent,
	}, nil)

	// Step 4: Mock SaveEvent for WalletPaymentCompleted
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "WalletPaymentCompleted"
	})).Return(nil)

	// Step 5: Mock Publish for WalletPaymentCompleted
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "WalletPaymentCompleted"
	})).Return(nil)

	// Execute: Process FundsDebited event
	err := orchestrator.ProcessEvent(ctx, fundsDebitedEvent)

	// Assertions
	assert.NoError(t, err)
	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)

	// Verify WalletPaymentCompleted was published
	mockEventBus.AssertCalled(t, "Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		if e.Type() != "WalletPaymentCompleted" {
			return false
		}
		data, ok := e.Data().(events.WalletPaymentCompletedData)
		if !ok {
			return false
		}
		assert.Equal(t, paymentID, data.PaymentID)
		assert.Equal(t, sagaID, data.SagaID)
		assert.Equal(t, userID, data.UserID)
		assert.Equal(t, amount, data.Amount)
		return true
	}))
}

func TestOrchestrator_WalletPayment_InsufficientBalance(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockLogger := logger.NewMockLogger()

	orchestrator := NewOrchestrator(mockEventStore, mockEventBus, mockLogger)

	ctx := context.Background()
	userID := "user_456"
	paymentID := "pay_abc999"
	sagaID := "saga_456"
	requestedAmount := 1000.0
	availableBalance := 500.0

	// Step 1: Create wallet payment request
	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}
	walletRequestEvent := events.NewWalletPaymentRequested(
		paymentID,
		sagaID,
		userID,
		"svc_789",
		requestedAmount,
		"USD",
		metadata,
		2001,
	)

	// Step 2: FundsInsufficient event arrives (from Wallet Service)
	fundsInsufficientEvent := events.NewFundsInsufficient(
		paymentID,
		userID,
		requestedAmount,
		availableBalance,
		"wallet",
		metadata,
		2002,
	)

	// Step 3: Mock LoadEvents to return WalletPaymentRequested (for saga reconstruction)
	mockEventStore.On("LoadEvents", ctx, paymentID).Return([]events.Event{
		walletRequestEvent,
		fundsInsufficientEvent,
	}, nil)

	// Step 4: Mock SaveEvent for WalletPaymentFailed
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "WalletPaymentFailed"
	})).Return(nil)

	// Step 5: Mock Publish for WalletPaymentFailed
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "WalletPaymentFailed"
	})).Return(nil)

	// Execute: Process FundsInsufficient event
	err := orchestrator.ProcessEvent(ctx, fundsInsufficientEvent)

	// Assertions
	assert.NoError(t, err)
	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)

	// Verify WalletPaymentFailed was published with correct reason
	mockEventBus.AssertCalled(t, "Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		if e.Type() != "WalletPaymentFailed" {
			return false
		}
		data, ok := e.Data().(events.WalletPaymentFailedData)
		if !ok {
			return false
		}
		assert.Equal(t, paymentID, data.PaymentID)
		assert.Equal(t, sagaID, data.SagaID)
		assert.Equal(t, "insufficient_funds", data.Reason)
		return true
	}))
}

func TestOrchestrator_ExternalPayment_HappyPath(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockLogger := logger.NewMockLogger()

	orchestrator := NewOrchestrator(mockEventStore, mockEventBus, mockLogger)

	ctx := context.Background()
	userID := "user_123"
	paymentID := "pay_abc999"
	sagaID := "saga_456"
	amount := 2000.0

	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	// Step 1: ExternalPaymentRequested event (initial)
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

	// Step 2: PaymentSentToGateway event (from External Payment Service)
	paymentSentEvent := events.NewPaymentSentToGateway(
		paymentID,
		sagaID,
		"external",
		"ext_txn_123",
		metadata,
		1005,
	)

	// Step 3: PaymentGatewayResponse event (SUCCESS from gateway)
	gatewayResponseEvent := events.NewPaymentGatewayResponse(
		paymentID,
		sagaID,
		"external",
		"SUCCESS",
		"txn_ext_456",
		map[string]interface{}{},
		metadata,
		1006,
	)

	// Mock LoadEvents for saga reconstruction (returns all events so far)
	mockEventStore.On("LoadEvents", ctx, paymentID).Return([]events.Event{
		externalRequestEvent,
		paymentSentEvent,
		gatewayResponseEvent,
	}, nil)

	// Mock SaveEvent for ExternalPaymentCompleted
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "ExternalPaymentCompleted"
	})).Return(nil)

	// Mock Publish for ExternalPaymentCompleted
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "ExternalPaymentCompleted"
	})).Return(nil)

	// Execute: Process PaymentGatewayResponse with SUCCESS status
	err := orchestrator.ProcessEvent(ctx, gatewayResponseEvent)

	// Assertions
	assert.NoError(t, err)
	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)

	// Verify ExternalPaymentCompleted was published
	mockEventBus.AssertCalled(t, "Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		if e.Type() != "ExternalPaymentCompleted" {
			return false
		}
		data, ok := e.Data().(events.ExternalPaymentCompletedData)
		if !ok {
			return false
		}
		assert.Equal(t, paymentID, data.PaymentID)
		assert.Equal(t, sagaID, data.SagaID)
		assert.Equal(t, "external", data.GatewayProvider)
		return true
	}))
}

func TestOrchestrator_ExternalPayment_GatewayFailure(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockLogger := logger.NewMockLogger()

	orchestrator := NewOrchestrator(mockEventStore, mockEventBus, mockLogger)

	ctx := context.Background()
	userID := "user_123"
	paymentID := "pay_abc999"
	sagaID := "saga_456"
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

	// Gateway responds with FAILED
	gatewayResponseEvent := events.NewPaymentGatewayResponse(
		paymentID,
		sagaID,
		"external",
		"FAILED",
		"",
		map[string]interface{}{},
		metadata,
		1006,
	)

	// Mock LoadEvents for saga reconstruction
	mockEventStore.On("LoadEvents", ctx, paymentID).Return([]events.Event{
		externalRequestEvent,
		gatewayResponseEvent,
	}, nil)

	// Mock SaveEvent for ExternalPaymentFailed
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "ExternalPaymentFailed"
	})).Return(nil)

	// Mock Publish for ExternalPaymentFailed
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "ExternalPaymentFailed"
	})).Return(nil)

	// Execute: Process PaymentGatewayResponse with FAILED status
	err := orchestrator.ProcessEvent(ctx, gatewayResponseEvent)

	// Assertions
	assert.NoError(t, err)
	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)

	// Verify ExternalPaymentFailed was published
	mockEventBus.AssertCalled(t, "Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		if e.Type() != "ExternalPaymentFailed" {
			return false
		}
		data, ok := e.Data().(events.ExternalPaymentFailedData)
		if !ok {
			return false
		}
		assert.Equal(t, paymentID, data.PaymentID)
		assert.Equal(t, "FAILED", data.Reason)
		return true
	}))
}
