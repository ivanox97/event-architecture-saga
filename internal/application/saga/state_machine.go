package saga

import (
	"event-saga/internal/domain/events"
	"event-saga/internal/domain/saga"
	"fmt"
)

const (
	_fundsDebited           = "FundsDebited"
	_fundsInsufficient      = "FundsInsufficient"
	_paymentSentToGateway   = "PaymentSentToGateway"
	_paymentGatewayResponse = "PaymentGatewayResponse"
)

// StateMachine handles state transitions for sagas
type StateMachine struct {
	orchestrator *Orchestrator
}

// NewStateMachine creates a new state machine
func NewStateMachine(o *Orchestrator) *StateMachine {
	return &StateMachine{orchestrator: o}
}

// GetNextState determines the next state based on current state and event
func (sm *StateMachine) GetNextState(s *saga.Saga, event events.Event) (saga.SagaState, error) {
	switch s.CurrentState() {
	case saga.SagaValidatingBalance:
		return fundsDebited(event)
	case saga.SagaSendingToGateway:
		return paymentSentToGateway(event)
	case saga.SagaSentToGateway:
		return paymentGatewayResponse(event)
	}

	return s.CurrentState(), nil
}

// Transition handles state transitions
func (sm *StateMachine) Transition(s *saga.Saga, event events.Event) error {
	nextState, err := sm.GetNextState(s, event)
	if err != nil {
		return err
	}

	if nextState != s.CurrentState() {
		return s.TransitionTo(nextState)
	}

	return nil
}

func fundsDebited(event events.Event) (saga.SagaState, error) {
	if event.Type() == _fundsDebited {
		return saga.SagaCompleted, nil
	}

	return saga.SagaState(""), fmt.Errorf("invalid event type: %s", event.Type())
}

func paymentSentToGateway(event events.Event) (saga.SagaState, error) {
	if event.Type() == _paymentSentToGateway {
		return saga.SagaAwaitingResponse, nil
	}

	return saga.SagaState(""), fmt.Errorf("invalid event type: %s", event.Type())
}

func paymentGatewayResponse(event events.Event) (saga.SagaState, error) {
	if event.Type() == _paymentGatewayResponse {
		return saga.SagaCompleted, nil
	}

	return saga.SagaState(""), fmt.Errorf("invalid event type: %s", event.Type())
}
