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
	paymentData, ok := event.Data().(events.WalletPaymentRequestedData)
	if !ok {
		return fmt.Errorf("invalid event data type, expected WalletPaymentRequestedData")
	}

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

	if err := s.eventStore.SaveEvent(ctx, debitEvent); err != nil {
		return fmt.Errorf("failed to save debit event: %w", err)
	}

	if err := s.eventBus.Publish(ctx, configs.TopicPayments, debitEvent); err != nil {
		return fmt.Errorf("failed to publish debit event: %w", err)
	}

	s.logger.Info("Funds debited", logger.Field{Key: "user_id", Value: userID}, logger.Field{Key: "amount", Value: paymentData.Amount})
	return nil
}
