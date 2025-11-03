package wallet

import (
	"testing"
	"time"

	"event-saga/internal/domain/events"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestWallet_CanDebit(t *testing.T) {
	tests := []struct {
		name           string
		balance        float64
		debitAmount    float64
		expectedResult bool
	}{
		{
			name:           "sufficient balance",
			balance:        100.0,
			debitAmount:    50.0,
			expectedResult: true,
		},
		{
			name:           "insufficient balance",
			balance:        50.0,
			debitAmount:    100.0,
			expectedResult: false,
		},
		{
			name:           "exact balance",
			balance:        100.0,
			debitAmount:    100.0,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := NewWallet(uuid.New().String())
			w.balance = tt.balance
			w.availableBalance = tt.balance

			result := w.CanDebit(tt.debitAmount)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestWallet_ValidateDebit(t *testing.T) {
	w := NewWallet(uuid.New().String())
	w.balance = 100.0
	w.availableBalance = 100.0

	// Valid debit
	err := w.ValidateDebit(50.0)
	assert.NoError(t, err)

	// Invalid debit - insufficient funds
	err = w.ValidateDebit(150.0)
	assert.Error(t, err)
	assert.Equal(t, ErrInsufficientFunds, err)

	// Invalid debit - negative amount
	err = w.ValidateDebit(-10.0)
	assert.Error(t, err)
}

func TestWallet_ApplyEvent(t *testing.T) {
	w := NewWallet(uuid.New().String())

	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	debitData := events.FundsDebitedData{
		PaymentID:       uuid.New().String(),
		UserID:          w.UserID(),
		Amount:          50.0,
		PreviousBalance: 100.0,
		NewBalance:      50.0,
		PaymentType:     "wallet",
		DebitedAt:       time.Now(),
	}

	baseEvent := events.NewBaseEvent(
		uuid.New().String(),
		"FundsDebited",
		w.UserID(),
		"Wallet",
		1,
		debitData,
		metadata,
		1,
	)

	err := w.ApplyEvent(baseEvent)
	assert.NoError(t, err)
	assert.Equal(t, 50.0, w.Balance())
	assert.Equal(t, 50.0, w.AvailableBalance())
}

func TestWallet_ApplyEvent_FundsCredited(t *testing.T) {
	w := NewWallet(uuid.New().String())
	w.balance = 100.0
	w.availableBalance = 100.0

	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	creditData := events.FundsCreditedData{
		RefundID:        uuid.New().String(),
		PaymentID:       uuid.New().String(),
		UserID:          w.UserID(),
		Amount:          50.0,
		PreviousBalance: 100.0,
		NewBalance:      150.0,
		Reason:          "refund",
		CreditedAt:      time.Now(),
	}

	baseEvent := events.NewBaseEvent(
		uuid.New().String(),
		"FundsCredited",
		w.UserID(),
		"Wallet",
		1,
		creditData,
		metadata,
		2,
	)

	err := w.ApplyEvent(baseEvent)
	assert.NoError(t, err)
	assert.Equal(t, 150.0, w.Balance())
	assert.Equal(t, 150.0, w.AvailableBalance())
}
