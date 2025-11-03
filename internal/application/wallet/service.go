package wallet

import (
	"context"
	"fmt"
	"time"

	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"
	"event-saga/internal/domain/wallet"
	"event-saga/internal/infrastructure/eventbus"
	"event-saga/internal/infrastructure/eventstore"

	"github.com/google/uuid"
)

type Service struct {
	eventStore eventstore.EventStore
	eventBus   eventbus.EventBus
	logger     logger.Logger
	sequence   int64
}

func NewService(es eventstore.EventStore, eb eventbus.EventBus, l logger.Logger) *Service {
	return &Service{
		eventStore: es,
		eventBus:   eb,
		logger:     l,
		sequence:   0,
	}
}

func (s *Service) RebuildWalletState(ctx context.Context, userID string) (*wallet.Wallet, error) {
	events, err := s.eventStore.LoadEvents(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to load events: %w", err)
	}

	w := wallet.NewWallet(userID)

	for _, event := range events {
		if err := w.ApplyEvent(event); err != nil {
			return nil, fmt.Errorf("failed to apply event: %w", err)
		}
	}

	return w, nil
}

type ProcessRefundRequest struct {
	PaymentID string  `json:"payment_id"`
	UserID    string  `json:"user_id"`
	Amount    float64 `json:"amount"`
	Reason    string  `json:"reason"`
}

func (s *Service) ProcessRefund(ctx context.Context, req ProcessRefundRequest) error {
	if req.Amount <= 0 {
		return fmt.Errorf("refund amount must be positive")
	}

	w, err := s.RebuildWalletState(ctx, req.UserID)
	if err != nil {
		return fmt.Errorf("failed to rebuild wallet state: %w", err)
	}

	previousBalance := w.Balance()
	newBalance := previousBalance + req.Amount

	refundID := uuid.New().String()
	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	s.sequence++
	creditEvent := events.NewFundsCredited(
		refundID,
		req.PaymentID,
		req.UserID,
		req.Amount,
		previousBalance,
		newBalance,
		req.Reason,
		metadata,
		s.sequence,
	)

	if err := s.eventStore.SaveEvent(ctx, creditEvent); err != nil {
		return fmt.Errorf("failed to save credit event: %w", err)
	}

	if err := s.eventBus.Publish(ctx, configs.TopicPayments, creditEvent); err != nil {
		return fmt.Errorf("failed to publish credit event: %w", err)
	}

	s.logger.Info("Funds credited", logger.Field{Key: "user_id", Value: req.UserID}, logger.Field{Key: "amount", Value: req.Amount}, logger.Field{Key: "refund_id", Value: refundID})
	return nil
}
