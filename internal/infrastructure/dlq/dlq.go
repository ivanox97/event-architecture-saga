package dlq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"event-saga/internal/domain/events"
)

type DLQEvent struct {
	DLQEventID        string
	OriginalEvent     events.Event
	FailureReason     string
	FailureCount      int
	FirstFailureAt    time.Time
	LastAttemptAt     time.Time
	ConsumerGroup     string
	OriginalTopic     string
	OriginalPartition int
	OriginalOffset    int64
	ErrorDetails      map[string]interface{}
}

type DLQHandler func(ctx context.Context, event DLQEvent) error

type DLQSimulator struct {
	mu        sync.RWMutex
	events    []DLQEvent
	consumers []DLQHandler
	running   bool
	maxEvents int // Maximum events to store (for memory management)
}

func NewDLQSimulator() *DLQSimulator {
	return &DLQSimulator{
		events:    make([]DLQEvent, 0),
		consumers: make([]DLQHandler, 0),
		running:   true,
		maxEvents: 10000,
	}
}

func (d *DLQSimulator) Publish(ctx context.Context, originalEvent events.Event, failureReason string, consumerGroup, originalTopic string, originalPartition int, originalOffset int64) error {
	if !d.running {
		return fmt.Errorf("DLQ is closed")
	}

	dlqEvent := DLQEvent{
		DLQEventID:        fmt.Sprintf("dlq_%d_%s", time.Now().UnixNano(), originalEvent.ID()),
		OriginalEvent:     originalEvent,
		FailureReason:     failureReason,
		FailureCount:      1,
		FirstFailureAt:    time.Now(),
		LastAttemptAt:     time.Now(),
		ConsumerGroup:     consumerGroup,
		OriginalTopic:     originalTopic,
		OriginalPartition: originalPartition,
		OriginalOffset:    originalOffset,
		ErrorDetails:      make(map[string]interface{}),
	}

	d.mu.Lock()
	if len(d.events) >= d.maxEvents {
		d.events = d.events[1000:]
	}
	d.events = append(d.events, dlqEvent)
	consumers := make([]DLQHandler, len(d.consumers))
	copy(consumers, d.consumers)
	d.mu.Unlock()

	for _, handler := range consumers {
		go func(h DLQHandler) {
			if err := h(ctx, dlqEvent); err != nil {
				fmt.Printf("Error handling DLQ event %s: %v\n", dlqEvent.DLQEventID, err)
			}
		}(handler)
	}

	return nil
}

func (d *DLQSimulator) Subscribe(ctx context.Context, handler DLQHandler) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.consumers == nil {
		d.consumers = make([]DLQHandler, 0)
	}

	d.consumers = append(d.consumers, handler)

	go d.consumeEvents(ctx, handler)

	return nil
}

func (d *DLQSimulator) consumeEvents(ctx context.Context, handler DLQHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			d.mu.RLock()
			if len(d.events) == 0 {
				d.mu.RUnlock()
				time.Sleep(100 * time.Millisecond)
				continue
			}

			eventsCopy := make([]DLQEvent, len(d.events))
			copy(eventsCopy, d.events)
			d.mu.RUnlock()

			for _, event := range eventsCopy {
				if err := handler(ctx, event); err != nil {
					fmt.Printf("Error processing DLQ event %s: %v\n", event.DLQEventID, err)
				}
			}

			time.Sleep(1 * time.Second)
		}
	}
}

func (d *DLQSimulator) GetEvents() []DLQEvent {
	d.mu.RLock()
	defer d.mu.RUnlock()

	eventsCopy := make([]DLQEvent, len(d.events))
	copy(eventsCopy, d.events)
	return eventsCopy
}

func (d *DLQSimulator) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.running = false
	return nil
}

type DLQ interface {
	Publish(ctx context.Context, originalEvent events.Event, failureReason string, consumerGroup, originalTopic string, originalPartition int, originalOffset int64) error
	Subscribe(ctx context.Context, handler DLQHandler) error
	GetEvents() []DLQEvent
	Close() error
}
