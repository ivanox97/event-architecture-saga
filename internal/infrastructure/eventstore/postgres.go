package eventstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"event-saga/internal/domain/events"
)

const (
	insertEventQuery = `
		INSERT INTO events (
			event_id, aggregate_id, aggregate_type, event_type,
			event_version, event_data, event_metadata, timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	selectEventsByAggregateQuery = `
		SELECT event_id, aggregate_id, aggregate_type, event_type,
		       event_version, event_data, event_metadata, timestamp, sequence_number
		FROM events
		WHERE aggregate_id = $1
		ORDER BY sequence_number ASC
	`
)

type PostgresEventStore struct {
	db *sql.DB
}

func NewPostgresEventStore(connString string) (*PostgresEventStore, error) {
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresEventStore{db: db}, nil
}

func (es *PostgresEventStore) SaveEvent(ctx context.Context, event events.Event) error {
	fmt.Printf("[SaveEvent] Saving event - Type: %s, ID: %s, AggregateID: %s, AggregateType: %s\n",
		event.Type(), event.ID(), event.AggregateID(), event.AggregateType())

	eventData, err := json.Marshal(event.Data())
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}
	fmt.Printf("[SaveEvent] Event data JSON length: %d bytes\n", len(eventData))

	metadata, err := json.Marshal(event.Metadata())
	if err != nil {
		return fmt.Errorf("failed to marshal event metadata: %w", err)
	}

	_, err = es.db.ExecContext(ctx, insertEventQuery,
		event.ID(),
		event.AggregateID(),
		event.AggregateType(),
		event.Type(),
		event.Version(),
		eventData,
		metadata,
		event.Timestamp(),
	)

	if err != nil {
		fmt.Printf("[SaveEvent] ERROR saving event: %v\n", err)
		return fmt.Errorf("failed to save event: %w", err)
	}

	fmt.Printf("[SaveEvent] Successfully saved event - Type: %s, AggregateID: %s\n", event.Type(), event.AggregateID())
	return nil
}

func (es *PostgresEventStore) LoadEvents(ctx context.Context, aggregateID string) ([]events.Event, error) {
	fmt.Printf("[LoadEvents] Loading events for aggregateID: %s\n", aggregateID)

	rows, err := es.db.QueryContext(ctx, selectEventsByAggregateQuery, aggregateID)
	if err != nil {
		fmt.Printf("[LoadEvents] ERROR querying events: %v\n", err)
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var loadedEvents []events.Event
	eventCount := 0
	for rows.Next() {
		var eventID, aggID, aggType, eventType string
		var version int
		var eventDataJSON, metadataJSON []byte
		var timestamp sql.NullTime
		var sequenceNumber int64

		err := rows.Scan(&eventID, &aggID, &aggType, &eventType, &version, &eventDataJSON, &metadataJSON, &timestamp, &sequenceNumber)
		if err != nil {
			fmt.Printf("[LoadEvents] ERROR scanning row: %v\n", err)
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		fmt.Printf("[LoadEvents] Found event - Type: %s, ID: %s, AggregateID: %s, DataJSON length: %d\n",
			eventType, eventID, aggID, len(eventDataJSON))

		event, err := es.reconstructEvent(eventType, eventID, aggID, aggType, version, eventDataJSON, metadataJSON, timestamp.Time, sequenceNumber)
		if err != nil {
			fmt.Printf("[LoadEvents] ERROR reconstructing event: %v\n", err)
			return nil, fmt.Errorf("failed to reconstruct event: %w", err)
		}

		fmt.Printf("[LoadEvents] Reconstructed event - Type: %s, Data type: %T, Data value: %+v\n",
			event.Type(), event.Data(), event.Data())

		loadedEvents = append(loadedEvents, event)
		eventCount++
	}

	if err := rows.Err(); err != nil {
		fmt.Printf("[LoadEvents] ERROR iterating rows: %v\n", err)
		return nil, fmt.Errorf("error iterating events: %w", err)
	}

	fmt.Printf("[LoadEvents] Loaded %d events for aggregateID: %s\n", eventCount, aggregateID)
	return loadedEvents, nil
}

func (es *PostgresEventStore) reconstructEvent(eventType, eventID, aggID, aggType string, version int, eventDataJSON, metadataJSON []byte, timestamp time.Time, sequenceNumber int64) (events.Event, error) {
	var metadata events.EventMetadata
	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
		}
	}

	var eventData interface{}
	switch eventType {
	case "WalletPaymentRequested":
		fmt.Printf("[reconstructEvent] Unmarshaling WalletPaymentRequested - JSON: %s\n", string(eventDataJSON))
		var data events.WalletPaymentRequestedData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			fmt.Printf("[reconstructEvent] ERROR unmarshaling WalletPaymentRequested: %v\n", err)
			return nil, fmt.Errorf("failed to unmarshal WalletPaymentRequested data: %w", err)
		}
		fmt.Printf("[reconstructEvent] Successfully unmarshaled WalletPaymentRequested - PaymentID: %s, SagaID: %s\n",
			data.PaymentID, data.SagaID)
		eventData = data
	case "ExternalPaymentRequested":
		var data events.ExternalPaymentRequestedData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ExternalPaymentRequested data: %w", err)
		}
		eventData = data
	case "FundsDebited":
		var data events.FundsDebitedData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal FundsDebited data: %w", err)
		}
		eventData = data
	case "FundsInsufficient":
		var data events.FundsInsufficientData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal FundsInsufficient data: %w", err)
		}
		eventData = data
	case "FundsCredited":
		var data events.FundsCreditedData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal FundsCredited data: %w", err)
		}
		eventData = data
	case "PaymentGatewayResponse":
		var data events.PaymentGatewayResponseData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PaymentGatewayResponse data: %w", err)
		}
		eventData = data
	case "WalletPaymentCompleted":
		var data events.WalletPaymentCompletedData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal WalletPaymentCompleted data: %w", err)
		}
		eventData = data
	case "WalletPaymentFailed":
		var data events.WalletPaymentFailedData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal WalletPaymentFailed data: %w", err)
		}
		eventData = data
	case "ExternalPaymentCompleted":
		var data events.ExternalPaymentCompletedData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ExternalPaymentCompleted data: %w", err)
		}
		eventData = data
	case "ExternalPaymentFailed":
		var data events.ExternalPaymentFailedData
		if err := json.Unmarshal(eventDataJSON, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ExternalPaymentFailed data: %w", err)
		}
		eventData = data
	default:
		if err := json.Unmarshal(eventDataJSON, &eventData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
		}
	}

	baseEvent := events.NewBaseEventWithTimestamp(eventID, eventType, aggID, aggType, version, eventData, metadata, sequenceNumber, timestamp)
	return baseEvent, nil
}

func (es *PostgresEventStore) Close() error {
	return es.db.Close()
}
