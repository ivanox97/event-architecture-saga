package saga

import (
	"context"
	"testing"

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
	args := m.Called(ctx, topic, handler)
	return args.Error(0)
}

func (m *MockEventBus) SubscribeWithGroupID(ctx context.Context, topic, groupID string, handler eventbus.EventHandler) error {
	args := m.Called(ctx, topic, groupID, handler)
	return args.Error(0)
}

func (m *MockEventBus) Close() error {
	args := m.Called()
	return args.Error(0)
}

func TestOrchestrator_CreateWalletPayment(t *testing.T) {
	mockEventStore := new(MockEventStore)
	mockEventBus := new(MockEventBus)
	mockLogger := logger.NewMockLogger()

	orchestrator := NewOrchestrator(mockEventStore, mockEventBus, mockLogger)

	req := CreateWalletPaymentRequest{
		UserID:    uuid.New().String(),
		ServiceID: "service-123",
		Amount:    100.0,
		Currency:  "USD",
	}

	// No saga repository save expected - using Event Sourcing
	mockEventStore.On("SaveEvent", mock.Anything, mock.AnythingOfType("*events.WalletPaymentRequested")).Return(nil)
	mockEventBus.On("Publish", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("*events.WalletPaymentRequested")).Return(nil)

	resp, err := orchestrator.CreateWalletPayment(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.NotEmpty(t, resp.PaymentID)
	assert.NotEmpty(t, resp.SagaID)
	assert.Equal(t, "INITIALIZED", resp.Status)

	mockEventStore.AssertExpectations(t)
	mockEventBus.AssertExpectations(t)
}
