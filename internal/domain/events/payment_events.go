package events

import (
	"time"

	"github.com/google/uuid"
)

type WalletPaymentRequestedData struct {
	PaymentID      string
	SagaID         string
	UserID         string
	ServiceID      string
	Amount         float64
	Currency       string
	IdempotencyKey string
	Metadata       map[string]string
}

type WalletPaymentRequested struct {
	*BaseEvent
}

func NewWalletPaymentRequested(paymentID, sagaID, userID, serviceID string, amount float64, currency string, metadata EventMetadata, sequenceNumber int64) *WalletPaymentRequested {
	data := WalletPaymentRequestedData{
		PaymentID:      paymentID,
		SagaID:         sagaID,
		UserID:         userID,
		ServiceID:      serviceID,
		Amount:         amount,
		Currency:       currency,
		IdempotencyKey: uuid.New().String(),
		Metadata:       make(map[string]string),
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"WalletPaymentRequested",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &WalletPaymentRequested{BaseEvent: base}
}

type WalletPaymentCompletedData struct {
	PaymentID       string
	SagaID          string
	UserID          string
	Amount          float64
	Currency        string
	CompletedAt     time.Time
	GatewayProvider string
}

type WalletPaymentCompleted struct {
	*BaseEvent
}

func NewWalletPaymentCompleted(paymentID, sagaID, userID string, amount float64, currency string, metadata EventMetadata, sequenceNumber int64) *WalletPaymentCompleted {
	data := WalletPaymentCompletedData{
		PaymentID:       paymentID,
		SagaID:          sagaID,
		UserID:          userID,
		Amount:          amount,
		Currency:        currency,
		CompletedAt:     time.Now(),
		GatewayProvider: "wallet",
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"WalletPaymentCompleted",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &WalletPaymentCompleted{BaseEvent: base}
}

type WalletPaymentFailedData struct {
	PaymentID string
	SagaID    string
	UserID    string
	Amount    float64
	Currency  string
	Reason    string
	FailedAt  time.Time
}

type WalletPaymentFailed struct {
	*BaseEvent
}

func NewWalletPaymentFailed(paymentID, sagaID, userID string, amount float64, currency, reason string, metadata EventMetadata, sequenceNumber int64) *WalletPaymentFailed {
	data := WalletPaymentFailedData{
		PaymentID: paymentID,
		SagaID:    sagaID,
		UserID:    userID,
		Amount:    amount,
		Currency:  currency,
		Reason:    reason,
		FailedAt:  time.Now(),
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"WalletPaymentFailed",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &WalletPaymentFailed{BaseEvent: base}
}

type ExternalPaymentRequestedData struct {
	PaymentID      string
	SagaID         string
	UserID         string
	ServiceID      string
	Amount         float64
	Currency       string
	CardToken      string
	IdempotencyKey string
	Metadata       map[string]string
}

type ExternalPaymentRequested struct {
	*BaseEvent
}

func NewExternalPaymentRequested(paymentID, sagaID, userID, serviceID string, amount float64, currency, cardToken string, metadata EventMetadata, sequenceNumber int64) *ExternalPaymentRequested {
	data := ExternalPaymentRequestedData{
		PaymentID:      paymentID,
		SagaID:         sagaID,
		UserID:         userID,
		ServiceID:      serviceID,
		Amount:         amount,
		Currency:       currency,
		CardToken:      cardToken,
		IdempotencyKey: uuid.New().String(),
		Metadata:       make(map[string]string),
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"ExternalPaymentRequested",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &ExternalPaymentRequested{BaseEvent: base}
}

type PaymentSentToGatewayData struct {
	PaymentID        string
	SagaID           string
	GatewayProvider  string
	GatewayPaymentID string
	SentAt           time.Time
}

type PaymentSentToGateway struct {
	*BaseEvent
}

func NewPaymentSentToGateway(paymentID, sagaID, gatewayProvider, gatewayPaymentID string, metadata EventMetadata, sequenceNumber int64) *PaymentSentToGateway {
	data := PaymentSentToGatewayData{
		PaymentID:        paymentID,
		SagaID:           sagaID,
		GatewayProvider:  gatewayProvider,
		GatewayPaymentID: gatewayPaymentID,
		SentAt:           time.Now(),
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"PaymentSentToGateway",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &PaymentSentToGateway{BaseEvent: base}
}

type PaymentGatewayResponseData struct {
	PaymentID       string
	SagaID          string
	GatewayProvider string
	Status          string // "SUCCESS", "FAILED", "INSUFFICIENT_FUNDS", "TIMEOUT"
	TransactionID   string
	ResponseData    map[string]interface{}
	RespondedAt     time.Time
}

type PaymentGatewayResponse struct {
	*BaseEvent
}

func NewPaymentGatewayResponse(paymentID, sagaID, gatewayProvider, status, transactionID string, responseData map[string]interface{}, metadata EventMetadata, sequenceNumber int64) *PaymentGatewayResponse {
	data := PaymentGatewayResponseData{
		PaymentID:       paymentID,
		SagaID:          sagaID,
		GatewayProvider: gatewayProvider,
		Status:          status,
		TransactionID:   transactionID,
		ResponseData:    responseData,
		RespondedAt:     time.Now(),
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"PaymentGatewayResponse",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &PaymentGatewayResponse{BaseEvent: base}
}

type ExternalPaymentCompletedData struct {
	PaymentID       string
	SagaID          string
	UserID          string
	Amount          float64
	Currency        string
	CompletedAt     time.Time
	GatewayProvider string
	TransactionID   string
}

type ExternalPaymentCompleted struct {
	*BaseEvent
}

func NewExternalPaymentCompleted(paymentID, sagaID, userID string, amount float64, currency, gatewayProvider, transactionID string, metadata EventMetadata, sequenceNumber int64) *ExternalPaymentCompleted {
	data := ExternalPaymentCompletedData{
		PaymentID:       paymentID,
		SagaID:          sagaID,
		UserID:          userID,
		Amount:          amount,
		Currency:        currency,
		CompletedAt:     time.Now(),
		GatewayProvider: gatewayProvider,
		TransactionID:   transactionID,
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"ExternalPaymentCompleted",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &ExternalPaymentCompleted{BaseEvent: base}
}

type ExternalPaymentFailedData struct {
	PaymentID       string
	SagaID          string
	UserID          string
	Amount          float64
	Currency        string
	Reason          string
	FailedAt        time.Time
	GatewayProvider string
}

type ExternalPaymentFailed struct {
	*BaseEvent
}

func NewExternalPaymentFailed(paymentID, sagaID, userID string, amount float64, currency, reason, gatewayProvider string, metadata EventMetadata, sequenceNumber int64) *ExternalPaymentFailed {
	data := ExternalPaymentFailedData{
		PaymentID:       paymentID,
		SagaID:          sagaID,
		UserID:          userID,
		Amount:          amount,
		Currency:        currency,
		Reason:          reason,
		FailedAt:        time.Now(),
		GatewayProvider: gatewayProvider,
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"ExternalPaymentFailed",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &ExternalPaymentFailed{BaseEvent: base}
}

type PaymentGatewayTimeoutData struct {
	PaymentID              string
	SagaID                 string
	GatewayProvider        string
	Attempt                int
	MaxAttempts            int
	TimeoutDurationSeconds int
	TimeoutAt              time.Time
}

type PaymentGatewayTimeout struct {
	*BaseEvent
}

func NewPaymentGatewayTimeout(paymentID, sagaID, gatewayProvider string, attempt, maxAttempts, timeoutDurationSeconds int, metadata EventMetadata, sequenceNumber int64) *PaymentGatewayTimeout {
	data := PaymentGatewayTimeoutData{
		PaymentID:              paymentID,
		SagaID:                 sagaID,
		GatewayProvider:        gatewayProvider,
		Attempt:                attempt,
		MaxAttempts:            maxAttempts,
		TimeoutDurationSeconds: timeoutDurationSeconds,
		TimeoutAt:              time.Now(),
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"PaymentGatewayTimeout",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &PaymentGatewayTimeout{BaseEvent: base}
}

type PaymentRetryRequestedData struct {
	PaymentID       string
	SagaID          string
	Attempt         int
	PreviousAttempt int
	PreviousError   string
	NextRetryAt     time.Time
}

type PaymentRetryRequested struct {
	*BaseEvent
}

func NewPaymentRetryRequested(paymentID, sagaID string, attempt, previousAttempt int, previousError string, nextRetryAt time.Time, metadata EventMetadata, sequenceNumber int64) *PaymentRetryRequested {
	data := PaymentRetryRequestedData{
		PaymentID:       paymentID,
		SagaID:          sagaID,
		Attempt:         attempt,
		PreviousAttempt: previousAttempt,
		PreviousError:   previousError,
		NextRetryAt:     nextRetryAt,
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"PaymentRetryRequested",
		paymentID,
		"Payment",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &PaymentRetryRequested{BaseEvent: base}
}
