package errors

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"event-saga/internal/domain/events"
	"event-saga/internal/infrastructure/dlq"
)

const (
	insertErrorLogQuery = `
		INSERT INTO error_logs (
			error_id, dlq_event_id, payment_id, saga_id, error_type, error_reason,
			original_event, failure_details, retry_history,
			first_occurred_at, last_occurred_at, resolved, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
	`

	selectUnresolvedErrorsQuery = `
		SELECT 
			error_id, dlq_event_id, payment_id, saga_id, error_type, error_reason,
			original_event, failure_details, retry_history,
			first_occurred_at, last_occurred_at, resolved, created_at
		FROM error_logs
		WHERE resolved = FALSE
		ORDER BY created_at DESC
		LIMIT $1
	`

	updateErrorResolvedQuery = `
		UPDATE error_logs
		SET resolved = TRUE, resolved_at = NOW()
		WHERE error_id = $1
	`
)

type DBErrors struct {
	db *sql.DB
}

func NewDBErrors(db *sql.DB) *DBErrors {
	return &DBErrors{
		db: db,
	}
}

func (dbe *DBErrors) PersistDLQEvent(ctx context.Context, dlqEvent dlq.DLQEvent) error {
	paymentID, sagaID := extractPaymentAndSagaID(dlqEvent.OriginalEvent)
	errorType := classifyErrorType(dlqEvent.FailureReason)

	originalEventJSON, err := serializeEvent(dlqEvent.OriginalEvent)
	if err != nil {
		return fmt.Errorf("failed to serialize original event: %w", err)
	}

	errorDetailsJSON, err := json.Marshal(dlqEvent.ErrorDetails)
	if err != nil {
		return fmt.Errorf("failed to serialize error details: %w", err)
	}

	errorID := uuid.New()

	retryHistoryJSON := []byte("[]")
	if dlqEvent.ErrorDetails != nil {
		if retryHistory, ok := dlqEvent.ErrorDetails["retry_history"].([]interface{}); ok {
			retryHistoryJSON, _ = json.Marshal(retryHistory)
		}
	}

	_, err = dbe.db.ExecContext(ctx, insertErrorLogQuery,
		errorID,
		dlqEvent.DLQEventID,
		paymentID,
		sagaID,
		errorType,
		dlqEvent.FailureReason,
		originalEventJSON,
		string(errorDetailsJSON),
		string(retryHistoryJSON),
		dlqEvent.FirstFailureAt,
		dlqEvent.LastAttemptAt,
		false,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to persist DLQ event to error_logs: %w", err)
	}

	return nil
}

func (dbe *DBErrors) GetUnresolvedErrors(ctx context.Context, limit int) ([]ErrorLog, error) {
	rows, err := dbe.db.QueryContext(ctx, selectUnresolvedErrorsQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query unresolved errors: %w", err)
	}
	defer rows.Close()

	var errors []ErrorLog
	for rows.Next() {
		var el ErrorLog
		var originalEventJSON, failureDetailsJSON, retryHistoryJSON sql.NullString

		err := rows.Scan(
			&el.ErrorID,
			&el.DLQEventID,
			&el.PaymentID,
			&el.SagaID,
			&el.ErrorType,
			&el.ErrorReason,
			&originalEventJSON,
			&failureDetailsJSON,
			&retryHistoryJSON,
			&el.FirstOccurredAt,
			&el.LastOccurredAt,
			&el.Resolved,
			&el.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan error log: %w", err)
		}

		if originalEventJSON.Valid {
			el.OriginalEvent = []byte(originalEventJSON.String)
		}
		if failureDetailsJSON.Valid {
			el.FailureDetails = []byte(failureDetailsJSON.String)
		}
		if retryHistoryJSON.Valid {
			el.RetryHistory = []byte(retryHistoryJSON.String)
		}

		errors = append(errors, el)
	}

	return errors, nil
}

func (dbe *DBErrors) MarkAsResolved(ctx context.Context, errorID uuid.UUID) error {
	_, err := dbe.db.ExecContext(ctx, updateErrorResolvedQuery, errorID)
	if err != nil {
		return fmt.Errorf("failed to mark error as resolved: %w", err)
	}

	return nil
}

type ErrorLog struct {
	ErrorID         uuid.UUID
	DLQEventID      string
	PaymentID       sql.NullString
	SagaID          sql.NullString
	ErrorType       string
	ErrorReason     string
	OriginalEvent   []byte
	FailureDetails  []byte
	RetryHistory    []byte
	FirstOccurredAt time.Time
	LastOccurredAt  time.Time
	Resolved        bool
	CreatedAt       time.Time
}

func extractPaymentAndSagaID(event events.Event) (paymentID, sagaID string) {
	switch data := event.Data().(type) {
	case events.WalletPaymentRequestedData:
		return data.PaymentID, data.SagaID
	case events.ExternalPaymentRequestedData:
		return data.PaymentID, data.SagaID
	case events.WalletPaymentFailedData:
		return data.PaymentID, data.SagaID
	case events.ExternalPaymentFailedData:
		return data.PaymentID, data.SagaID
	case events.PaymentGatewayResponseData:
		return data.PaymentID, data.SagaID
	case events.FundsCreditedData:
		return data.PaymentID, ""
	default:
		return event.AggregateID(), ""
	}
}

func classifyErrorType(failureReason string) string {
	switch failureReason {
	case "MAX_RETRIES_EXCEEDED":
		return "TIMEOUT_MAX_RETRIES"
	case "GATEWAY_TIMEOUT":
		return "GATEWAY_TIMEOUT"
	case "GATEWAY_REJECTED":
		return "GATEWAY_REJECTED"
	case "INSUFFICIENT_FUNDS":
		return "INSUFFICIENT_FUNDS"
	case "SCHEMA_VALIDATION_FAILED":
		return "SCHEMA_VALIDATION"
	default:
		return "UNKNOWN"
	}
}

func serializeEvent(event events.Event) ([]byte, error) {
	eventData := map[string]interface{}{
		"event_id":       event.ID(),
		"event_type":     event.Type(),
		"aggregate_id":   event.AggregateID(),
		"aggregate_type": event.AggregateType(),
		"event_version":  event.Version(),
		"data":           event.Data(),
		"metadata":       event.Metadata(),
		"timestamp":      event.Timestamp(),
	}

	return json.Marshal(eventData)
}
