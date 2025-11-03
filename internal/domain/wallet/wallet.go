package wallet

import (
	"errors"
	"event-saga/internal/domain/events"
)

var (
	// ErrInsufficientFunds indicates insufficient funds for debit
	ErrInsufficientFunds = errors.New("insufficient funds")
)

// Wallet represents a wallet aggregate
type Wallet struct {
	userID           string
	balance          float64
	availableBalance float64
	version          int
}

// NewWallet creates a new wallet instance
func NewWallet(userID string) *Wallet {
	return &Wallet{
		userID:           userID,
		balance:          0,
		availableBalance: 0,
		version:          0,
	}
}

// UserID returns the user identifier
func (w *Wallet) UserID() string {
	return w.userID
}

// Balance returns the total balance
func (w *Wallet) Balance() float64 {
	return w.balance
}

// AvailableBalance returns the available balance
func (w *Wallet) AvailableBalance() float64 {
	return w.availableBalance
}

// Version returns the aggregate version for optimistic locking
func (w *Wallet) Version() int {
	return w.version
}

// ApplyEvent applies an event to reconstruct the wallet state
func (w *Wallet) ApplyEvent(event events.Event) error {
	switch event.Type() {
	case "FundsDebited":
		data, ok := event.Data().(events.FundsDebitedData)
		if !ok {
			return nil
		}
		w.balance = data.NewBalance
		w.availableBalance = data.NewBalance
		w.version++
		return nil
	case "FundsCredited":
		data, ok := event.Data().(events.FundsCreditedData)
		if !ok {
			return nil
		}
		w.balance = data.NewBalance
		w.availableBalance = data.NewBalance
		w.version++
		return nil
	default:
		return nil // Ignore unknown events
	}
}

// CanDebit checks if the wallet has sufficient funds for a debit
func (w *Wallet) CanDebit(amount float64) bool {
	return w.availableBalance >= amount
}

// ValidateDebit validates if a debit operation is allowed
func (w *Wallet) ValidateDebit(amount float64) error {
	if amount <= 0 {
		return errors.New("debit amount must be positive")
	}
	if !w.CanDebit(amount) {
		return ErrInsufficientFunds
	}
	return nil
}
