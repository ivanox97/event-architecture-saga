package saga

import (
	"testing"
	"time"

	"event-saga/internal/domain/events"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSaga_TransitionTo(t *testing.T) {
	tests := []struct {
		name         string
		currentState SagaState
		targetState  SagaState
		wantErr      bool
	}{
		{
			name:         "valid transition INITIALIZED to VALIDATING_BALANCE",
			currentState: SagaInitialized,
			targetState:  SagaValidatingBalance,
			wantErr:      false,
		},
		{
			name:         "valid transition VALIDATING_BALANCE to COMPLETED",
			currentState: SagaValidatingBalance,
			targetState:  SagaCompleted,
			wantErr:      false,
		},
		{
			name:         "valid transition VALIDATING_BALANCE to FAILED",
			currentState: SagaValidatingBalance,
			targetState:  SagaFailed,
			wantErr:      false,
		},
		{
			name:         "invalid transition COMPLETED to FAILED",
			currentState: SagaCompleted,
			targetState:  SagaFailed,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSaga(uuid.New().String(), uuid.New().String(), uuid.New().String(), "wallet")

			// Set initial state
			if tt.currentState != SagaInitialized {
				s.currentState = tt.currentState
			}

			err := s.TransitionTo(tt.targetState)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.targetState, s.CurrentState())
			}
		})
	}
}

func TestSaga_ApplyEvent(t *testing.T) {
	s := NewSaga(uuid.New().String(), uuid.New().String(), uuid.New().String(), "wallet")

	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	// Apply FundsDebited event should transition to COMPLETED
	debitData := events.FundsDebitedData{
		PaymentID:       s.PaymentID(),
		UserID:          s.UserID(),
		Amount:          100.0,
		PreviousBalance: 200.0,
		NewBalance:      100.0,
		PaymentType:     "wallet",
		DebitedAt:       time.Now(),
	}

	baseEvent := events.NewBaseEvent(
		uuid.New().String(),
		"FundsDebited",
		s.PaymentID(),
		"Payment",
		1,
		debitData,
		metadata,
		1,
	)

	err := s.ApplyEvent(baseEvent)
	assert.NoError(t, err)
	assert.Equal(t, SagaCompleted, s.CurrentState())
}

func TestSaga_IsTerminal(t *testing.T) {
	s := NewSaga(uuid.New().String(), uuid.New().String(), uuid.New().String(), "wallet")
	assert.False(t, s.IsTerminal())

	s.TransitionTo(SagaCompleted)
	assert.True(t, s.IsTerminal())

	s = NewSaga(uuid.New().String(), uuid.New().String(), uuid.New().String(), "wallet")
	s.TransitionTo(SagaFailed)
	assert.True(t, s.IsTerminal())
}
