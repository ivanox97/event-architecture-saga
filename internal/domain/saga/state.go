package saga

// SagaState represents the current state of a saga
type SagaState string

const (
	// SagaInitialized indicates the saga has been created
	SagaInitialized SagaState = "INITIALIZED"
	// SagaValidatingBalance indicates balance validation is in progress (wallet payment)
	SagaValidatingBalance SagaState = "VALIDATING_BALANCE"
	// SagaSendingToGateway indicates sending to external gateway (external payment)
	SagaSendingToGateway SagaState = "SENDING_TO_GATEWAY"
	// SagaSentToGateway indicates payment sent to gateway (external payment)
	SagaSentToGateway SagaState = "SENT_TO_GATEWAY"
	// SagaAwaitingResponse indicates waiting for gateway response (external payment)
	SagaAwaitingResponse SagaState = "AWAITING_RESPONSE"
	// SagaCompleted indicates the saga completed successfully
	SagaCompleted SagaState = "COMPLETED"
	// SagaFailed indicates the saga failed
	SagaFailed SagaState = "FAILED"
)

// CanTransitionTo checks if a state transition is valid
func (s SagaState) CanTransitionTo(target SagaState) bool {
	validTransitions := map[SagaState][]SagaState{
		// Initial state can transition to either wallet or external payment flow
		SagaInitialized: {SagaValidatingBalance, SagaSendingToGateway},
		// Wallet payment transitions
		SagaValidatingBalance: {SagaCompleted, SagaFailed},
		// External payment transitions
		SagaSendingToGateway: {SagaSentToGateway},
		SagaSentToGateway:    {SagaAwaitingResponse},
		SagaAwaitingResponse: {SagaCompleted, SagaFailed},
		// Terminal states
		SagaCompleted: {}, // Terminal state
		SagaFailed:    {}, // Terminal state
	}

	allowed, exists := validTransitions[s]
	if !exists {
		return false
	}

	for _, state := range allowed {
		if state == target {
			return true
		}
	}

	return false
}
