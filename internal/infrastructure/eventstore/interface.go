package eventstore

import (
	"context"
	"event-saga/internal/domain/events"
)

// EventStore defines the interface for event storage
type EventStore interface {
	// SaveEvent persists an event to the store
	SaveEvent(ctx context.Context, event events.Event) error
	// LoadEvents loads all events for a given aggregate
	LoadEvents(ctx context.Context, aggregateID string) ([]events.Event, error)
}
