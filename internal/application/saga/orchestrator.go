package saga

import (
	"context"
	"fmt"
	"time"

	"event-saga/internal/common/configs"
	"event-saga/internal/common/logger"
	"event-saga/internal/domain/events"
	"event-saga/internal/domain/saga"
	"event-saga/internal/infrastructure/eventbus"
	"event-saga/internal/infrastructure/eventstore"

	"github.com/google/uuid"
)

type CreateWalletPaymentRequest struct {
	UserID    string            `json:"user_id"`
	ServiceID string            `json:"service_id"`
	Amount    float64           `json:"amount"`
	Currency  string            `json:"currency"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type CreateExternalPaymentRequest struct {
	UserID    string            `json:"user_id"`
	ServiceID string            `json:"service_id"`
	Amount    float64           `json:"amount"`
	Currency  string            `json:"currency"`
	CardToken string            `json:"card_token"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type PaymentResponse struct {
	PaymentID string `json:"payment_id"`
	SagaID    string `json:"saga_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type PaymentStatus struct {
	PaymentID string  `json:"payment_id"`
	SagaID    string  `json:"saga_id"`
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
	Currency  string  `json:"currency"`
}

type Orchestrator struct {
	eventStore eventstore.EventStore
	eventBus   eventbus.EventBus
	logger     logger.Logger
	sequence   int64
}

func NewOrchestrator(es eventstore.EventStore, eb eventbus.EventBus, l logger.Logger) *Orchestrator {
	return &Orchestrator{
		eventStore: es,
		eventBus:   eb,
		logger:     l,
		sequence:   0,
	}
}

// RebuildSagaFromEvents reconstructs a saga from events in the event store
// Since events are stored with paymentID as aggregateID, paymentID is required to load events
// All events for a paymentID belong to the same saga, so we process all events without filtering by sagaID
func (o *Orchestrator) rebuildSagaFromEvents(ctx context.Context, sagaID, paymentID string) (*saga.Saga, error) {
	if paymentID == "" {
		return nil, fmt.Errorf("paymentID required to load events for saga: %s", sagaID)
	}

	eventsList, err := o.eventStore.LoadEvents(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load events for payment: %w", err)
	}

	if len(eventsList) == 0 {
		return nil, fmt.Errorf("no events found for payment: %s", paymentID)
	}

	var userID, paymentType, detectedSagaID string
	var sagaEvents []events.Event

	// Process all events for this paymentID (they all belong to the same saga)
	for i, event := range eventsList {
		fmt.Printf("[rebuildSagaFromEvents] Processing event %d/%d - Type: %s, Data type: %T\n",
			i+1, len(eventsList), event.Type(), event.Data())

		// Determine payment type from the first request event
		switch e := event.Data().(type) {
		case events.WalletPaymentRequestedData:
			fmt.Printf("[rebuildSagaFromEvents] Matched WalletPaymentRequestedData - UserID: %s, SagaID: %s\n",
				e.UserID, e.SagaID)
			if paymentType == "" {
				userID = e.UserID
				paymentType = "wallet"
				detectedSagaID = e.SagaID // Use sagaID from the event
				fmt.Printf("[rebuildSagaFromEvents] Set paymentType=wallet, userID=%s, sagaID=%s\n",
					userID, detectedSagaID)
			}
			sagaEvents = append(sagaEvents, event)
		case events.ExternalPaymentRequestedData:
			fmt.Printf("[rebuildSagaFromEvents] Matched ExternalPaymentRequestedData - UserID: %s, SagaID: %s\n",
				e.UserID, e.SagaID)
			if paymentType == "" {
				userID = e.UserID
				paymentType = "external"
				detectedSagaID = e.SagaID // Use sagaID from the event
				fmt.Printf("[rebuildSagaFromEvents] Set paymentType=external, userID=%s, sagaID=%s\n",
					userID, detectedSagaID)
			}
			sagaEvents = append(sagaEvents, event)
		default:
			fmt.Printf("[rebuildSagaFromEvents] Unknown event data type %T, including in saga events\n", event.Data())
			// Include all other events related to this payment
			sagaEvents = append(sagaEvents, event)
		}
	}

	if paymentType == "" {
		return nil, fmt.Errorf("could not determine payment type for payment: %s (no WalletPaymentRequested or ExternalPaymentRequested event found)", paymentID)
	}

	if userID == "" {
		return nil, fmt.Errorf("could not determine userID for payment: %s", paymentID)
	}

	// Use detected sagaID from the event, or fallback to provided one
	finalSagaID := detectedSagaID
	if finalSagaID == "" {
		finalSagaID = sagaID
	}

	s := saga.NewSaga(finalSagaID, paymentID, userID, paymentType)
	for _, event := range sagaEvents {
		if err := s.ApplyEvent(event); err != nil {
			o.logger.Info("Failed to apply event to saga", logger.Field{Key: "event_type", Value: event.Type()}, logger.Field{Key: "error", Value: err})
		}
	}

	return s, nil
}

// CreateWalletPayment creates a new wallet payment and initiates a saga
func (o *Orchestrator) CreateWalletPayment(ctx context.Context, req CreateWalletPaymentRequest) (*PaymentResponse, error) {
	paymentID := uuid.New().String()
	sagaID := uuid.New().String()

	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	o.sequence++
	event := events.NewWalletPaymentRequested(
		paymentID,
		sagaID,
		req.UserID,
		req.ServiceID,
		req.Amount,
		req.Currency,
		metadata,
		o.sequence,
	)

	fmt.Printf("[CreateWalletPayment] Created event - Type: %s, PaymentID: %s, SagaID: %s, AggregateID: %s\n",
		event.Type(), paymentID, sagaID, event.AggregateID())
	fmt.Printf("[CreateWalletPayment] Event Data type: %T, Event Data: %+v\n", event.Data(), event.Data())

	if err := o.eventStore.SaveEvent(ctx, event); err != nil {
		fmt.Printf("[CreateWalletPayment] ERROR saving event: %v\n", err)
		return nil, fmt.Errorf("failed to save event: %w", err)
	}
	fmt.Printf("[CreateWalletPayment] Event saved successfully\n")

	if err := o.eventBus.Publish(ctx, configs.TopicPayments, event); err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	o.logger.Info("Wallet payment created", logger.Field{Key: "payment_id", Value: paymentID}, logger.Field{Key: "saga_id", Value: sagaID})

	return &PaymentResponse{
		PaymentID: paymentID,
		SagaID:    sagaID,
		Status:    string(saga.SagaInitialized),
		CreatedAt: event.Timestamp().Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

// CreateExternalPayment creates a new external payment and initiates a saga
func (o *Orchestrator) CreateExternalPayment(ctx context.Context, req CreateExternalPaymentRequest) (*PaymentResponse, error) {
	paymentID := uuid.New().String()
	sagaID := uuid.New().String()

	metadata := events.EventMetadata{
		CorrelationID: uuid.New().String(),
		TraceID:       uuid.New().String(),
		Timestamp:     time.Now(),
	}

	o.sequence++
	event := events.NewExternalPaymentRequested(
		paymentID,
		sagaID,
		req.UserID,
		req.ServiceID,
		req.Amount,
		req.Currency,
		req.CardToken,
		metadata,
		o.sequence,
	)

	if err := o.eventStore.SaveEvent(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to save event: %w", err)
	}

	if err := o.eventBus.Publish(ctx, configs.TopicPayments, event); err != nil {
		return nil, fmt.Errorf("failed to publish event: %w", err)
	}

	o.logger.Info("External payment created", logger.Field{Key: "payment_id", Value: paymentID}, logger.Field{Key: "saga_id", Value: sagaID})

	return &PaymentResponse{
		PaymentID: paymentID,
		SagaID:    sagaID,
		Status:    string(saga.SagaInitialized),
		CreatedAt: event.Timestamp().Format("2006-01-02T15:04:05Z07:00"),
	}, nil
}

func (o *Orchestrator) ProcessEvent(ctx context.Context, event events.Event) error {
	switch event.Type() {
	case "FundsDebited":
		return o.handleFundsDebited(ctx, event)
	case "FundsInsufficient":
		return o.handleFundsInsufficient(ctx, event)
	case "PaymentGatewayResponse":
		return o.handlePaymentGatewayResponse(ctx, event)
	default:
		return nil
	}
}

func (o *Orchestrator) handleFundsDebited(ctx context.Context, event events.Event) error {
	fmt.Printf("[handleFundsDebited] Received FundsDebited event\n")

	data, ok := event.Data().(events.FundsDebitedData)
	if !ok {
		fmt.Printf("[handleFundsDebited] ERROR: Invalid event data type - expected FundsDebitedData, got: %T\n", event.Data())
		return fmt.Errorf("invalid event data type, expected FundsDebitedData")
	}

	fmt.Printf("[handleFundsDebited] Processing - PaymentID: %s, UserID: %s, NewBalance: %f\n",
		data.PaymentID, data.UserID, data.NewBalance)

	eventsList, err := o.eventStore.LoadEvents(ctx, data.PaymentID)
	if err != nil {
		return fmt.Errorf("failed to load events for payment: %w", err)
	}

	var sagaID string
	for _, e := range eventsList {
		if walletReq, ok := e.Data().(events.WalletPaymentRequestedData); ok {
			sagaID = walletReq.SagaID
			break
		}
	}

	if sagaID == "" {
		return fmt.Errorf("sagaID not found for payment: %s", data.PaymentID)
	}

	s, err := o.rebuildSagaFromEvents(ctx, sagaID, data.PaymentID)
	if err != nil {
		return fmt.Errorf("failed to rebuild saga: %w", err)
	}

	fmt.Printf("[handleFundsDebited] Applying event to saga - Current state: %s\n", s.CurrentState())

	if err := s.ApplyEvent(event); err != nil {
		fmt.Printf("[handleFundsDebited] ERROR applying event: %v\n", err)
		return fmt.Errorf("failed to apply event: %w", err)
	}

	fmt.Printf("[handleFundsDebited] Event applied - New state: %s\n", s.CurrentState())
	fmt.Printf("[handleFundsDebited] Publishing WalletPaymentCompleted\n")

	return o.publishWalletPaymentCompleted(ctx, s, event)
}

func (o *Orchestrator) handleFundsInsufficient(ctx context.Context, event events.Event) error {
	data := event.Data().(events.FundsInsufficientData)

	eventsList, err := o.eventStore.LoadEvents(ctx, data.PaymentID)
	if err != nil {
		return fmt.Errorf("failed to load events for payment: %w", err)
	}

	var sagaID string
	for _, e := range eventsList {
		if walletReq, ok := e.Data().(events.WalletPaymentRequestedData); ok {
			sagaID = walletReq.SagaID
			break
		}
	}

	if sagaID == "" {
		return fmt.Errorf("sagaID not found for payment: %s", data.PaymentID)
	}

	s, err := o.rebuildSagaFromEvents(ctx, sagaID, data.PaymentID)
	if err != nil {
		return fmt.Errorf("failed to rebuild saga: %w", err)
	}

	if err := s.ApplyEvent(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	return o.publishWalletPaymentFailed(ctx, s, event, "insufficient_funds")
}

func (o *Orchestrator) handlePaymentGatewayResponse(ctx context.Context, event events.Event) error {
	data := event.Data().(events.PaymentGatewayResponseData)

	s, err := o.rebuildSagaFromEvents(ctx, data.SagaID, data.PaymentID)
	if err != nil {
		return fmt.Errorf("failed to rebuild saga: %w", err)
	}

	if err := s.ApplyEvent(event); err != nil {
		return fmt.Errorf("failed to apply event: %w", err)
	}

	if data.Status == "SUCCESS" {
		return o.publishExternalPaymentCompleted(ctx, s, event, data)
	} else {
		return o.publishExternalPaymentFailed(ctx, s, event, data.Status)
	}
}

// publishWalletPaymentCompleted publishes a WalletPaymentCompleted event
func (o *Orchestrator) publishWalletPaymentCompleted(ctx context.Context, s *saga.Saga, originalEvent events.Event) error {
	metadata := events.EventMetadata{
		CorrelationID: originalEvent.Metadata().CorrelationID,
		TraceID:       originalEvent.Metadata().TraceID,
		Timestamp:     originalEvent.Timestamp(),
	}

	var amount float64
	var currency string

	switch data := originalEvent.Data().(type) {
	case events.FundsDebitedData:
		amount = data.Amount
		currency = "USD"
	case events.WalletPaymentRequestedData:
		amount = data.Amount
		currency = data.Currency
	}

	o.sequence++
	completedEvent := events.NewWalletPaymentCompleted(
		s.PaymentID(),
		s.SagaID(),
		s.UserID(),
		amount,
		currency,
		metadata,
		o.sequence,
	)

	if err := o.eventStore.SaveEvent(ctx, completedEvent); err != nil {
		return fmt.Errorf("failed to save completion event: %w", err)
	}

	return o.eventBus.Publish(ctx, configs.TopicPayments, completedEvent)
}

// publishWalletPaymentFailed publishes a WalletPaymentFailed event
func (o *Orchestrator) publishWalletPaymentFailed(ctx context.Context, s *saga.Saga, originalEvent events.Event, reason string) error {
	metadata := events.EventMetadata{
		CorrelationID: originalEvent.Metadata().CorrelationID,
		TraceID:       originalEvent.Metadata().TraceID,
		Timestamp:     originalEvent.Timestamp(),
	}

	var amount float64
	var currency string

	switch data := originalEvent.Data().(type) {
	case events.WalletPaymentRequestedData:
		amount = data.Amount
		currency = data.Currency
	case events.FundsInsufficientData:
		amount = data.RequestedAmount
		currency = "USD"
	}

	o.sequence++
	failedEvent := events.NewWalletPaymentFailed(
		s.PaymentID(),
		s.SagaID(),
		s.UserID(),
		amount,
		currency,
		reason,
		metadata,
		o.sequence,
	)

	if err := o.eventStore.SaveEvent(ctx, failedEvent); err != nil {
		return fmt.Errorf("failed to save failure event: %w", err)
	}

	return o.eventBus.Publish(ctx, configs.TopicPayments, failedEvent)
}

// publishExternalPaymentCompleted publishes an ExternalPaymentCompleted event
func (o *Orchestrator) publishExternalPaymentCompleted(ctx context.Context, s *saga.Saga, originalEvent events.Event, responseData events.PaymentGatewayResponseData) error {
	metadata := events.EventMetadata{
		CorrelationID: originalEvent.Metadata().CorrelationID,
		TraceID:       originalEvent.Metadata().TraceID,
		Timestamp:     originalEvent.Timestamp(),
	}

	var amount float64
	var currency string

	if reqData, ok := originalEvent.Data().(events.ExternalPaymentRequestedData); ok {
		amount = reqData.Amount
		currency = reqData.Currency
	}

	o.sequence++
	completedEvent := events.NewExternalPaymentCompleted(
		s.PaymentID(),
		s.SagaID(),
		s.UserID(),
		amount,
		currency,
		responseData.GatewayProvider,
		responseData.TransactionID,
		metadata,
		o.sequence,
	)

	if err := o.eventStore.SaveEvent(ctx, completedEvent); err != nil {
		return fmt.Errorf("failed to save completion event: %w", err)
	}

	return o.eventBus.Publish(ctx, configs.TopicPayments, completedEvent)
}

// publishExternalPaymentFailed publishes an ExternalPaymentFailed event
func (o *Orchestrator) publishExternalPaymentFailed(ctx context.Context, s *saga.Saga, originalEvent events.Event, reason string) error {
	metadata := events.EventMetadata{
		CorrelationID: originalEvent.Metadata().CorrelationID,
		TraceID:       originalEvent.Metadata().TraceID,
		Timestamp:     originalEvent.Timestamp(),
	}

	var amount float64
	var currency string
	var gatewayProvider string

	if reqData, ok := originalEvent.Data().(events.ExternalPaymentRequestedData); ok {
		amount = reqData.Amount
		currency = reqData.Currency
		gatewayProvider = "external"
	} else if respData, ok := originalEvent.Data().(events.PaymentGatewayResponseData); ok {
		gatewayProvider = respData.GatewayProvider
		amount = 0
		currency = "USD"
	}

	o.sequence++
	failedEvent := events.NewExternalPaymentFailed(
		s.PaymentID(),
		s.SagaID(),
		s.UserID(),
		amount,
		currency,
		reason,
		gatewayProvider,
		metadata,
		o.sequence,
	)

	if err := o.eventStore.SaveEvent(ctx, failedEvent); err != nil {
		return fmt.Errorf("failed to save failure event: %w", err)
	}

	return o.eventBus.Publish(ctx, configs.TopicPayments, failedEvent)
}

func (o *Orchestrator) GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentStatus, error) {
	fmt.Printf("[GetPaymentStatus] Getting status for paymentID: %s\n", paymentID)

	eventsList, err := o.eventStore.LoadEvents(ctx, paymentID)
	if err != nil || len(eventsList) == 0 {
		fmt.Printf("[GetPaymentStatus] ERROR: No events found - err: %v, count: %d\n", err, len(eventsList))
		return nil, fmt.Errorf("payment not found: %s", paymentID)
	}

	fmt.Printf("[GetPaymentStatus] Loaded %d events\n", len(eventsList))

	var sagaID string
	var amount float64
	var currency string

	firstEvent := eventsList[0]
	fmt.Printf("[GetPaymentStatus] First event - Type: %s, Data type: %T, Data: %+v\n",
		firstEvent.Type(), firstEvent.Data(), firstEvent.Data())

	switch e := firstEvent.Data().(type) {
	case events.WalletPaymentRequestedData:
		fmt.Printf("[GetPaymentStatus] Matched WalletPaymentRequestedData - SagaID: %s\n", e.SagaID)
		sagaID = e.SagaID
		amount = e.Amount
		currency = e.Currency
	case events.ExternalPaymentRequestedData:
		fmt.Printf("[GetPaymentStatus] Matched ExternalPaymentRequestedData - SagaID: %s\n", e.SagaID)
		sagaID = e.SagaID
		amount = e.Amount
		currency = e.Currency
	default:
		fmt.Printf("[GetPaymentStatus] WARNING: Unknown event data type %T, using default\n", firstEvent.Data())
		sagaID = firstEvent.AggregateID()
		amount = 0
		currency = "USD"
	}

	fmt.Printf("[GetPaymentStatus] Calling rebuildSagaFromEvents with sagaID: %s, paymentID: %s\n", sagaID, paymentID)
	s, err := o.rebuildSagaFromEvents(ctx, sagaID, paymentID)
	if err != nil {
		fmt.Printf("[GetPaymentStatus] ERROR rebuilding saga: %v\n", err)
		return nil, fmt.Errorf("failed to rebuild saga: %w", err)
	}
	fmt.Printf("[GetPaymentStatus] Saga rebuilt successfully\n")

	return &PaymentStatus{
		PaymentID: s.PaymentID(),
		SagaID:    s.SagaID(),
		Status:    string(s.CurrentState()),
		Amount:    amount,
		Currency:  currency,
	}, nil
}
