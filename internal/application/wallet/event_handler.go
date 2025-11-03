package wallet

import (
	"context"
	"fmt"

	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func (s *Service) HandleWalletPaymentRequested(ctx context.Context, event events.Event) error {
	fmt.Printf("[HandleWalletPaymentRequested] Received event - Type: %s, Data type: %T, Data: %+v\n",
		event.Type(), event.Data(), event.Data())

	paymentData, ok := event.Data().(events.WalletPaymentRequestedData)
	if !ok {
		fmt.Printf("[HandleWalletPaymentRequested] ERROR: Invalid event data type - expected WalletPaymentRequestedData, got: %T, value: %+v\n",
			event.Data(), event.Data())
		return fmt.Errorf("invalid event data type, expected WalletPaymentRequestedData")
	}

	fmt.Printf("[HandleWalletPaymentRequested] Processing payment - PaymentID: %s, UserID: %s, Amount: %f\n",
		paymentData.PaymentID, paymentData.UserID, paymentData.Amount)

	userID := paymentData.UserID

	w, err := s.RebuildWalletState(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to rebuild wallet state: %w", err)
	}

	if err := w.ValidateDebit(paymentData.Amount); err != nil {
		s.sequence++
		metadata := event.Metadata()
		insufficientEvent := events.NewFundsInsufficient(
			paymentData.PaymentID,
			userID,
			paymentData.Amount,
			w.AvailableBalance(),
			"wallet",
			metadata,
			s.sequence,
		)

		if err := s.eventStore.SaveEvent(ctx, insufficientEvent); err != nil {
			return fmt.Errorf("failed to save insufficient funds event: %w", err)
		}

		if err := s.eventBus.Publish(ctx, configs.TopicPayments, insufficientEvent); err != nil {
			return fmt.Errorf("failed to publish insufficient funds event: %w", err)
		}

		s.logger.Warn("Insufficient funds", logger.Field{Key: "user_id", Value: userID}, logger.Field{Key: "amount", Value: paymentData.Amount})
		return nil
	}

	previousBalance := w.Balance()
	newBalance := previousBalance - paymentData.Amount

	s.sequence++
	metadata := event.Metadata()
	debitEvent := events.NewFundsDebited(
		paymentData.PaymentID,
		userID,
		paymentData.Amount,
		previousBalance,
		newBalance,
		"wallet",
		metadata,
		s.sequence,
	)

	fmt.Printf("[HandleWalletPaymentRequested] Saving FundsDebited event - PaymentID: %s, PreviousBalance: %f, NewBalance: %f\n",
		paymentData.PaymentID, previousBalance, newBalance)

	if err := s.eventStore.SaveEvent(ctx, debitEvent); err != nil {
		fmt.Printf("[HandleWalletPaymentRequested] ERROR saving debit event: %v\n", err)
		return fmt.Errorf("failed to save debit event: %w", err)
	}

	fmt.Printf("[HandleWalletPaymentRequested] Publishing FundsDebited event to Event Bus\n")
	if err := s.eventBus.Publish(ctx, configs.TopicPayments, debitEvent); err != nil {
		fmt.Printf("[HandleWalletPaymentRequested] ERROR publishing debit event: %v\n", err)
		return fmt.Errorf("failed to publish debit event: %w", err)
	}

	fmt.Printf("[HandleWalletPaymentRequested] Successfully processed - PaymentID: %s, NewBalance: %f\n",
		paymentData.PaymentID, newBalance)

	s.logger.Info("Funds debited", logger.Field{Key: "user_id", Value: userID}, logger.Field{Key: "amount", Value: paymentData.Amount})
	return nil
}
