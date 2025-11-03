package wallet

import (
	"context"
	"testing"
	"time"

	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"
	"event-saga/internal/infrastructure/eventbus"

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

func TestWalletService_HandleWalletPaymentRequested_HappyPath(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockLogger := logger.NewMockLogger()

	service := NewService(mockEventStore, mockEventBus, mockLogger)

	ctx := context.Background()
	userID := "user_123"
	paymentID := "pay_xyz789"
	sagaID := "saga_123"
	amount := 1500.0
	previousBalance := 5000.0
	newBalance := previousBalance - amount

	// Create WalletPaymentRequested event
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

	// Setup: Wallet has previous balance from events (simulating previous FundsDebited event that added balance)
	// For test purposes, we'll create a FundsDebited event with the initial balance
	// In a real scenario, there would be multiple events that built up the balance
	initialDebitEvent := createInitialBalanceEvent(userID, previousBalance, metadata, 1000)

	// Mock LoadEvents to return existing balance event
	mockEventStore.On("LoadEvents", ctx, userID).Return([]events.Event{initialDebitEvent}, nil)

	// Mock SaveEvent for FundsDebited event
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "FundsDebited"
	})).Return(nil)

	// Mock Publish for FundsDebited event
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "FundsDebited"
	})).Return(nil)

	// Execute: Handle the wallet payment request
	err := service.HandleWalletPaymentRequested(ctx, walletRequestEvent)

	// Assertions
	assert.NoError(t, err)
	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)

	// Verify that FundsDebited event was published
	mockEventBus.AssertCalled(t, "Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		if e.Type() != "FundsDebited" {
			return false
		}
		data, ok := e.Data().(events.FundsDebitedData)
		if !ok {
			return false
		}
		assert.Equal(t, paymentID, data.PaymentID)
		assert.Equal(t, userID, data.UserID)
		assert.Equal(t, amount, data.Amount)
		assert.Equal(t, previousBalance, data.PreviousBalance)
		assert.Equal(t, newBalance, data.NewBalance)
		return true
	}))
}

func TestWalletService_HandleWalletPaymentRequested_InsufficientBalance(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockLogger := logger.NewMockLogger()

	service := NewService(mockEventStore, mockEventBus, mockLogger)

	ctx := context.Background()
	userID := "user_456"
	paymentID := "pay_abc999"
	sagaID := "saga_456"
	requestedAmount := 1000.0
	availableBalance := 500.0 // Less than requested

	// Create WalletPaymentRequested event
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

	// Setup: Wallet has insufficient balance (500.0) from previous events
	initialDebitEvent := createInitialBalanceEvent(userID, availableBalance, metadata, 2000)

	// Mock LoadEvents to return balance event showing insufficient funds
	mockEventStore.On("LoadEvents", ctx, userID).Return([]events.Event{initialDebitEvent}, nil)

	// Mock SaveEvent for FundsInsufficient event
	mockEventStore.On("SaveEvent", ctx, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "FundsInsufficient"
	})).Return(nil)

	// Mock Publish for FundsInsufficient event
	mockEventBus.On("Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "FundsInsufficient"
	})).Return(nil)

	// Execute: Handle the wallet payment request
	err := service.HandleWalletPaymentRequested(ctx, walletRequestEvent)

	// Assertions
	assert.NoError(t, err)
	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)

	// Verify that FundsInsufficient event was published (not FundsDebited)
	mockEventBus.AssertCalled(t, "Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		if e.Type() != "FundsInsufficient" {
			return false
		}
		data, ok := e.Data().(events.FundsInsufficientData)
		if !ok {
			return false
		}
		assert.Equal(t, paymentID, data.PaymentID)
		assert.Equal(t, userID, data.UserID)
		assert.Equal(t, requestedAmount, data.RequestedAmount)
		assert.Equal(t, availableBalance, data.AvailableBalance)
		assert.Equal(t, "wallet", data.PaymentType)
		return true
	}))

	// Verify that FundsDebited was NOT called
	mockEventBus.AssertNotCalled(t, "Publish", ctx, configs.TopicPayments, mock.MatchedBy(func(e events.Event) bool {
		return e.Type() == "FundsDebited"
	}))
}

func createInitialBalanceEvent(userID string, balance float64, metadata events.EventMetadata, sequence int64) events.Event {
	return events.NewFundsDebited(
		"initial_payment",
		userID,
		balance,
		0,
		balance,
		"wallet",
		metadata,
		sequence,
	)
}
