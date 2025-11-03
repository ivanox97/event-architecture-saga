package events

import (
	"time"

	"github.com/google/uuid"
)

type FundsDebitedData struct {
	PaymentID       string
	UserID          string
	Amount          float64
	PreviousBalance float64
	NewBalance      float64
	PaymentType     string
	DebitedAt       time.Time
}

type FundsDebited struct {
	*BaseEvent
}

func NewFundsDebited(paymentID, userID string, amount, previousBalance, newBalance float64, paymentType string, metadata EventMetadata, sequenceNumber int64) *FundsDebited {
	data := FundsDebitedData{
		PaymentID:       paymentID,
		UserID:          userID,
		Amount:          amount,
		PreviousBalance: previousBalance,
		NewBalance:      newBalance,
		PaymentType:     paymentType,
		DebitedAt:       time.Now(),
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"FundsDebited",
		userID,
		"Wallet",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &FundsDebited{BaseEvent: base}
}

type FundsInsufficientData struct {
	PaymentID        string
	UserID           string
	RequestedAmount  float64
	AvailableBalance float64
	PaymentType      string
}

type FundsInsufficient struct {
	*BaseEvent
}

func NewFundsInsufficient(paymentID, userID string, requestedAmount, availableBalance float64, paymentType string, metadata EventMetadata, sequenceNumber int64) *FundsInsufficient {
	data := FundsInsufficientData{
		PaymentID:        paymentID,
		UserID:           userID,
		RequestedAmount:  requestedAmount,
		AvailableBalance: availableBalance,
		PaymentType:      paymentType,
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"FundsInsufficient",
		userID,
		"Wallet",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &FundsInsufficient{BaseEvent: base}
}

type FundsCreditedData struct {
	RefundID        string
	PaymentID       string
	UserID          string
	Amount          float64
	PreviousBalance float64
	NewBalance      float64
	Reason          string
	CreditedAt      time.Time
}

type FundsCredited struct {
	*BaseEvent
}

func NewFundsCredited(refundID, paymentID, userID string, amount, previousBalance, newBalance float64, reason string, metadata EventMetadata, sequenceNumber int64) *FundsCredited {
	data := FundsCreditedData{
		RefundID:        refundID,
		PaymentID:       paymentID,
		UserID:          userID,
		Amount:          amount,
		PreviousBalance: previousBalance,
		NewBalance:      newBalance,
		Reason:          reason,
		CreditedAt:      time.Now(),
	}

	base := NewBaseEvent(
		uuid.New().String(),
		"FundsCredited",
		userID,
		"Wallet",
		1,
		data,
		metadata,
		sequenceNumber,
	)

	return &FundsCredited{BaseEvent: base}
}
