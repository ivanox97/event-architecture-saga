package saga

import (
	"errors"
	"time"

	"event-saga/internal/domain/events"
)

var (
	ErrInvalidTransition = errors.New("invalid state transition")
)

// Saga represents a payment saga aggregate
type Saga struct {
	sagaID       string
	paymentID    string
	userID       string
	currentState SagaState
	paymentType  string
	version      int
	createdAt    time.Time
	lastActivity time.Time
}

func NewSaga(sagaID, paymentID, userID, paymentType string) *Saga {
	now := time.Now()
	return &Saga{
		sagaID:       sagaID,
		paymentID:    paymentID,
		userID:       userID,
		currentState: SagaInitialized,
		paymentType:  paymentType,
		version:      0,
		createdAt:    now,
		lastActivity: now,
	}
}

func (s *Saga) SagaID() string {
	return s.sagaID
}

func (s *Saga) PaymentID() string {
	return s.paymentID
}

func (s *Saga) UserID() string {
	return s.userID
}

func (s *Saga) CurrentState() SagaState {
	return s.currentState
}

func (s *Saga) PaymentType() string {
	return s.paymentType
}

func (s *Saga) Version() int {
	return s.version
}

func (s *Saga) CreatedAt() time.Time {
	return s.createdAt
}

func (s *Saga) LastActivity() time.Time {
	return s.lastActivity
}

// ApplyEvent applies an event to reconstruct the saga state
func (s *Saga) ApplyEvent(event events.Event) error {
	switch event.Type() {
	case "WalletPaymentRequested":
		// Transition to VALIDATING_BALANCE for wallet payments
		return s.TransitionTo(SagaValidatingBalance)
	case "ExternalPaymentRequested":
		// Transition to SENDING_TO_GATEWAY for external payments
		return s.TransitionTo(SagaSendingToGateway)
	case "FundsDebited":
		// Wallet payment completed
		return s.TransitionTo(SagaCompleted)
	case "FundsInsufficient":
		// Wallet payment failed
		return s.TransitionTo(SagaFailed)
	case "PaymentSentToGateway":
		// External payment sent to gateway
		return s.TransitionTo(SagaSentToGateway)
	case "PaymentGatewayResponse":
		// External payment response received - transition to awaiting response state
		// The actual completion/failure will be handled by ExternalPaymentCompleted/ExternalPaymentFailed events
		return s.TransitionTo(SagaAwaitingResponse)
	case "WalletPaymentCompleted", "ExternalPaymentCompleted":
		return s.TransitionTo(SagaCompleted)
	case "WalletPaymentFailed", "ExternalPaymentFailed":
		return s.TransitionTo(SagaFailed)
	default:
		return nil
	}
}

// TransitionTo transitions the saga to a new state
func (s *Saga) TransitionTo(newState SagaState) error {
	if !s.currentState.CanTransitionTo(newState) {
		return ErrInvalidTransition
	}

	s.currentState = newState
	s.version++
	s.lastActivity = time.Now()
	return nil
}

// IsTerminal returns true if the saga is in a terminal state
func (s *Saga) IsTerminal() bool {
	return s.currentState == SagaCompleted || s.currentState == SagaFailed
}
