package eventbus

import (
	"context"
	"event-saga/internal/domain/events"
)

type EventHandler func(ctx context.Context, event events.Event) error

// EventBus defines the interface for event publishing and subscription
type EventBus interface {
	// Publish publishes an event to a topic
	Publish(ctx context.Context, topic string, event events.Event) error
	// Subscribe subscribes to events from a topic (auto-generates consumer group ID)
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
	// SubscribeWithGroupID subscribes to events from a topic with a specific consumer group ID
	SubscribeWithGroupID(ctx context.Context, topic, groupID string, handler EventHandler) error
	// Close closes the event bus
	Close() error
}
