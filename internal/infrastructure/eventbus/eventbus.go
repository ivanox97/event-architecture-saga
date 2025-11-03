package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"event-saga/internal/domain/events"

	"github.com/segmentio/kafka-go"
)

const (
	defaultBrokerAddress = "localhost:19092"
	defaultNumPartitions = 12
	readTimeout          = 10 * time.Second
	writeTimeout         = 10 * time.Second
)

// eventBusImpl implements the EventBus interface
type eventBusImpl struct {
	brokers       []string
	numPartitions int
	writers       map[string]*kafka.Writer
	readers       map[string]*kafka.Reader
	writersMu     sync.RWMutex
	readersMu     sync.RWMutex
	consumers     map[string][]EventHandler
	consumersMu   sync.RWMutex
	running       bool
	mu            sync.RWMutex
}

// newEventBusImpl creates a new EventBus instance (internal function)
func newEventBusImpl(brokers []string) (EventBus, error) {
	if len(brokers) == 0 {
		brokers = []string{defaultBrokerAddress}
	}

	bus := &eventBusImpl{
		brokers:       brokers,
		numPartitions: defaultNumPartitions,
		writers:       make(map[string]*kafka.Writer),
		readers:       make(map[string]*kafka.Reader),
		consumers:     make(map[string][]EventHandler),
		running:       true,
	}

	return bus, nil
}

// Publish publishes an event to a topic
func (r *eventBusImpl) Publish(ctx context.Context, topicName string, event events.Event) error {
	r.mu.RLock()
	if !r.running {
		r.mu.RUnlock()
		return fmt.Errorf("event bus is closed")
	}
	r.mu.RUnlock()

	writer := r.getOrCreateWriter(topicName)

	partitionID, err := GetPartition(event, r.numPartitions)
	if err != nil {
		return fmt.Errorf("failed to calculate partition: %w", err)
	}

	// Serialize the complete event structure (similar to how PostgresEventStore saves events)
	eventJSON, err := r.marshalEvent(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	message := kafka.Message{
		Key:   []byte(event.AggregateID()),
		Value: eventJSON,
		Headers: []kafka.Header{
			{Key: "event_type", Value: []byte(event.Type())},
			{Key: "event_id", Value: []byte(event.ID())},
		},
		Partition: partitionID,
		Time:      event.Timestamp(),
	}

	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()

	if err := writer.WriteMessages(writeCtx, message); err != nil {
		return fmt.Errorf("failed to write message to topic %s: %w", topicName, err)
	}

	return nil
}

// Subscribe subscribes to events from a topic
// groupID is required for Kafka consumer groups to enable message commits
func (r *eventBusImpl) Subscribe(ctx context.Context, topicName string, handler EventHandler) error {
	return r.SubscribeWithGroupID(ctx, topicName, "", handler)
}

// SubscribeWithGroupID subscribes to events from a topic with a specific consumer group ID
func (r *eventBusImpl) SubscribeWithGroupID(ctx context.Context, topicName, groupID string, handler EventHandler) error {
	r.consumersMu.Lock()
	if r.consumers[topicName] == nil {
		r.consumers[topicName] = make([]EventHandler, 0)
	}
	r.consumers[topicName] = append(r.consumers[topicName], handler)
	r.consumersMu.Unlock()

	reader := r.getOrCreateReader(topicName, groupID)

	go r.consumeEvents(ctx, reader, handler)

	return nil
}

// consumeEvents consumes events from a reader and calls the handler
func (r *eventBusImpl) consumeEvents(ctx context.Context, reader *kafka.Reader, handler EventHandler) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			r.mu.RLock()
			if !r.running {
				r.mu.RUnlock()
				return
			}
			r.mu.RUnlock()

			readCtx, cancel := context.WithTimeout(ctx, readTimeout)

			message, err := reader.FetchMessage(readCtx)
			cancel()

			if err != nil {
				if err == context.DeadlineExceeded || err == context.Canceled {
					continue
				}
				fmt.Printf("Error fetching message: %v\n", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			event, err := r.unmarshalEvent(message)
			if err != nil {
				fmt.Printf("Error unmarshaling event: %v\n", err)
				if err := reader.CommitMessages(ctx, message); err != nil {
					fmt.Printf("Error committing invalid message: %v\n", err)
				}
				continue
			}

			if err := handler(ctx, event); err != nil {
				fmt.Printf("Error handling event %s: %v\n", event.Type(), err)
			}

			if err := reader.CommitMessages(ctx, message); err != nil {
				fmt.Printf("Error committing message: %v\n", err)
			}
		}
	}
}

// marshalEvent serializes an event to JSON
func (r *eventBusImpl) marshalEvent(event events.Event) ([]byte, error) {
	eventData, err := json.Marshal(event.Data())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event data: %w", err)
	}

	eventJSON := struct {
		ID             string               `json:"id"`
		Type           string               `json:"type"`
		AggregateID    string               `json:"aggregate_id"`
		AggregateType  string               `json:"aggregate_type"`
		Version        int                  `json:"version"`
		Data           json.RawMessage      `json:"data"`
		Metadata       events.EventMetadata `json:"metadata"`
		Timestamp      time.Time            `json:"timestamp"`
		SequenceNumber int64                `json:"sequence_number"`
	}{
		ID:             event.ID(),
		Type:           event.Type(),
		AggregateID:    event.AggregateID(),
		AggregateType:  event.AggregateType(),
		Version:        event.Version(),
		Data:           eventData,
		Metadata:       event.Metadata(),
		Timestamp:      event.Timestamp(),
		SequenceNumber: event.SequenceNumber(),
	}

	return json.Marshal(eventJSON)
}

// unmarshalEvent unmarshals a message into an Event
// Uses the same pattern as PostgresEventStore for consistency
func (r *eventBusImpl) unmarshalEvent(msg kafka.Message) (events.Event, error) {
	var eventData struct {
		ID             string               `json:"id"`
		Type           string               `json:"type"`
		AggregateID    string               `json:"aggregate_id"`
		AggregateType  string               `json:"aggregate_type"`
		Version        int                  `json:"version"`
		Data           json.RawMessage      `json:"data"`
		Metadata       events.EventMetadata `json:"metadata"`
		Timestamp      time.Time            `json:"timestamp"`
		SequenceNumber int64                `json:"sequence_number"`
	}

	if err := json.Unmarshal(msg.Value, &eventData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// Deserialize event data based on event type (same as PostgresEventStore.reconstructEvent)
	var deserializedData interface{}
	switch eventData.Type {
	case "WalletPaymentRequested":
		var data events.WalletPaymentRequestedData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal WalletPaymentRequested data: %w", err)
		}
		deserializedData = data
	case "ExternalPaymentRequested":
		var data events.ExternalPaymentRequestedData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ExternalPaymentRequested data: %w", err)
		}
		deserializedData = data
	case "FundsDebited":
		var data events.FundsDebitedData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal FundsDebited data: %w", err)
		}
		deserializedData = data
	case "FundsInsufficient":
		var data events.FundsInsufficientData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal FundsInsufficient data: %w", err)
		}
		deserializedData = data
	case "FundsCredited":
		var data events.FundsCreditedData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal FundsCredited data: %w", err)
		}
		deserializedData = data
	case "PaymentGatewayResponse":
		var data events.PaymentGatewayResponseData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal PaymentGatewayResponse data: %w", err)
		}
		deserializedData = data
	case "WalletPaymentCompleted":
		var data events.WalletPaymentCompletedData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal WalletPaymentCompleted data: %w", err)
		}
		deserializedData = data
	case "WalletPaymentFailed":
		var data events.WalletPaymentFailedData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal WalletPaymentFailed data: %w", err)
		}
		deserializedData = data
	case "ExternalPaymentCompleted":
		var data events.ExternalPaymentCompletedData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ExternalPaymentCompleted data: %w", err)
		}
		deserializedData = data
	case "ExternalPaymentFailed":
		var data events.ExternalPaymentFailedData
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ExternalPaymentFailed data: %w", err)
		}
		deserializedData = data
	default:
		// For unknown event types, try to unmarshal as generic map
		var data map[string]interface{}
		if err := json.Unmarshal(eventData.Data, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
		}
		deserializedData = data
	}

	baseEvent := events.NewBaseEventWithTimestamp(
		eventData.ID,
		eventData.Type,
		eventData.AggregateID,
		eventData.AggregateType,
		eventData.Version,
		deserializedData,
		eventData.Metadata,
		eventData.SequenceNumber,
		eventData.Timestamp,
	)

	return baseEvent, nil
}

// getOrCreateWriter gets or creates a writer for a topic
func (r *eventBusImpl) getOrCreateWriter(topicName string) *kafka.Writer {
	r.writersMu.RLock()
	if writer, exists := r.writers[topicName]; exists {
		r.writersMu.RUnlock()
		return writer
	}
	r.writersMu.RUnlock()

	r.writersMu.Lock()
	defer r.writersMu.Unlock()

	if writer, exists := r.writers[topicName]; exists {
		return writer
	}

	writer := &kafka.Writer{
		Addr:         kafka.TCP(r.brokers...),
		Topic:        topicName,
		Balancer:     &kafka.LeastBytes{},
		WriteTimeout: writeTimeout,
		RequiredAcks: kafka.RequireOne,
	}

	r.writers[topicName] = writer
	return writer
}

// getOrCreateReader gets or creates a reader for a topic with a consumer group ID
func (r *eventBusImpl) getOrCreateReader(topicName, groupID string) *kafka.Reader {
	// Create unique key for reader (topic + groupID)
	readerKey := topicName
	if groupID != "" {
		readerKey = topicName + ":" + groupID
	}

	r.readersMu.RLock()
	if reader, exists := r.readers[readerKey]; exists {
		r.readersMu.RUnlock()
		return reader
	}
	r.readersMu.RUnlock()

	r.readersMu.Lock()
	defer r.readersMu.Unlock()

	if reader, exists := r.readers[readerKey]; exists {
		return reader
	}

	// If no groupID provided, generate one based on topic name
	if groupID == "" {
		groupID = fmt.Sprintf("consumer-%s-%d", topicName, time.Now().Unix())
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     r.brokers,
		Topic:       topicName,
		GroupID:     groupID,
		MinBytes:    10e3,
		MaxBytes:    10e6,
		MaxWait:     1 * time.Second,
		StartOffset: kafka.FirstOffset, // Start from beginning to catch all messages
	})

	r.readers[readerKey] = reader
	return reader
}

// Close closes all connections
func (r *eventBusImpl) Close() error {
	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	var errors []error

	r.writersMu.Lock()
	for topic, writer := range r.writers {
		if err := writer.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close writer for topic %s: %w", topic, err))
		}
	}
	r.writersMu.Unlock()

	r.readersMu.Lock()
	for topic, reader := range r.readers {
		if err := reader.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close reader for topic %s: %w", topic, err))
		}
	}
	r.readersMu.Unlock()

	if len(errors) > 0 {
		return fmt.Errorf("errors closing event bus: %v", errors)
	}

	return nil
}
