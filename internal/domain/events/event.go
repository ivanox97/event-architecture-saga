package events

import "time"

type Event interface {
	ID() string
	Type() string
	AggregateID() string
	AggregateType() string
	Version() int
	Data() interface{}
	Metadata() EventMetadata
	SequenceNumber() int64
	Timestamp() time.Time
}

type EventMetadata struct {
	CorrelationID string
	TraceID       string
	Timestamp     time.Time
}

type BaseEvent struct {
	eventID        string
	eventType      string
	aggregateID    string
	aggregateType  string
	version        int
	data           interface{}
	metadata       EventMetadata
	sequenceNumber int64
	timestamp      time.Time
}

func (e *BaseEvent) ID() string {
	return e.eventID
}

func (e *BaseEvent) Type() string {
	return e.eventType
}

func (e *BaseEvent) AggregateID() string {
	return e.aggregateID
}

func (e *BaseEvent) AggregateType() string {
	return e.aggregateType
}

func (e *BaseEvent) Version() int {
	return e.version
}

func (e *BaseEvent) Data() interface{} {
	return e.data
}

func (e *BaseEvent) Metadata() EventMetadata {
	return e.metadata
}

func (e *BaseEvent) SequenceNumber() int64 {
	return e.sequenceNumber
}

func (e *BaseEvent) Timestamp() time.Time {
	return e.timestamp
}

func NewBaseEvent(eventID, eventType, aggregateID, aggregateType string, version int, data interface{}, metadata EventMetadata, sequenceNumber int64) *BaseEvent {
	return NewBaseEventWithTimestamp(eventID, eventType, aggregateID, aggregateType, version, data, metadata, sequenceNumber, time.Now())
}

func NewBaseEventWithTimestamp(eventID, eventType, aggregateID, aggregateType string, version int, data interface{}, metadata EventMetadata, sequenceNumber int64, timestamp time.Time) *BaseEvent {
	return &BaseEvent{
		eventID:        eventID,
		eventType:      eventType,
		aggregateID:    aggregateID,
		aggregateType:  aggregateType,
		version:        version,
		data:           data,
		metadata:       metadata,
		sequenceNumber: sequenceNumber,
		timestamp:      timestamp,
	}
}
