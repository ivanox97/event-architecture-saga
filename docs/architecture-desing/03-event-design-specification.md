# Especificación de Diseño de Eventos

## 2. Catálogo Completo de Eventos

### 2.1 Eventos de SAGA Orchestrator

#### PaymentRequested Event

```json
{
  "event_id": "evt_saga_req_001",
  "event_type": "PaymentRequested",
  "aggregate_id": "pay_xyz789",
  "aggregate_type": "Payment",
  "event_version": 1,
  "timestamp": "2024-01-15T10:30:00Z",
  "sequence_number": 1001,
  "kafka_partition": 2,
  "kafka_offset": 12345,
  "data": {
    "saga_id": "saga_123",
    "payment_id": "pay_xyz789",
    "user_id": "user_123",
    "service_id": "svc_456",
    "amount": 1500.0,
    "currency": "USD",
    "idempotency_key": "idemp_key_abc123",
    "metadata": {
      "source": "mobile_app"
    }
  },
  "metadata": {
    "correlation_id": "corr_123",
    "trace_id": "trace_456",
    "span_id": "span_789"
  }
}
```

#### PaymentProcessing Event

```json
{
  "event_id": "evt_processing_001",
  "event_type": "PaymentProcessing",
  "aggregate_id": "pay_xyz789",
  "aggregate_type": "Payment",
  "event_version": 1,
  "timestamp": "2024-01-15T10:30:06Z",
  "sequence_number": 1003,
  "data": {
    "saga_id": "saga_123",
    "payment_id": "pay_xyz789",
    "status": "PROCESSING"
  }
}
```

#### PaymentCompleted Event

```json
{
  "event_id": "evt_completed_001",
  "event_type": "PaymentCompleted",
  "aggregate_id": "pay_xyz789",
  "aggregate_type": "Payment",
  "event_version": 1,
  "timestamp": "2024-01-15T10:31:21Z",
  "sequence_number": 1006,
  "data": {
    "saga_id": "saga_123",
    "payment_id": "pay_xyz789",
    "transaction_id": "txn_ext_456",
    "completed_at": "2024-01-15T10:31:21Z"
  }
}
```

#### PaymentFailed Event

```json
{
  "event_id": "evt_failed_001",
  "event_type": "PaymentFailed",
  "aggregate_id": "pay_xyz789",
  "aggregate_type": "Payment",
  "event_version": 1,
  "timestamp": "2024-01-15T10:31:25Z",
  "sequence_number": 1007,
  "data": {
    "saga_id": "saga_123",
    "payment_id": "pay_xyz789",
    "reason": "GATEWAY_REJECTED",
    "failed_at": "2024-01-15T10:31:25Z",
    "compensation_required": true
  }
}
```

#### CompensationInitiated Event

```json
{
  "event_id": "evt_comp_001",
  "event_type": "CompensationInitiated",
  "aggregate_id": "saga_123",
  "aggregate_type": "Saga",
  "event_version": 1,
  "timestamp": "2024-01-15T10:31:26Z",
  "sequence_number": 1008,
  "data": {
    "saga_id": "saga_123",
    "payment_id": "pay_xyz789",
    "reason": "GATEWAY_REJECTED",
    "compensation_steps": [
      {
        "step": "DebitFunds",
        "compensation_action": "CreditFunds",
        "status": "PENDING"
      }
    ],
    "initiated_at": "2024-01-15T10:31:26Z"
  }
}
```

---

### 2.2 Eventos de Wallet Service

#### FundsDebited Event (Wallet Payment)

```json
{
  "event_id": "evt_debit_001",
  "event_type": "FundsDebited",
  "aggregate_id": "user_123",
  "aggregate_type": "Wallet",
  "event_version": 1,
  "timestamp": "2024-01-15T10:30:05Z",
  "sequence_number": 1002,
  "data": {
    "payment_id": "pay_xyz789",
    "user_id": "user_123",
    "amount": 1500.0,
    "previous_balance": 5000.0,
    "new_balance": 3500.0,
    "new_available_balance": 3500.0,
    "payment_type": "wallet",
    "debited_at": "2024-01-15T10:30:05Z"
  }
}
```

#### FundsCredited Event (Refunds, Top-ups)

```json
{
  "event_id": "evt_credit_001",
  "event_type": "FundsCredited",
  "aggregate_id": "user_123",
  "aggregate_type": "Wallet",
  "event_version": 1,
  "timestamp": "2024-01-15T10:31:26Z",
  "sequence_number": 1009,
  "data": {
    "user_id": "user_123",
    "amount": 500.0,
    "previous_balance": 3500.0,
    "new_balance": 4000.0,
    "reason": "REFUND", // o "TOP_UP"
    "credited_at": "2024-01-15T10:31:26Z"
  }
}
```

#### FundsInsufficient Event

```json
{
  "event_id": "evt_insufficient_001",
  "event_type": "FundsInsufficient",
  "aggregate_id": "user_456",
  "aggregate_type": "Wallet",
  "event_version": 1,
  "timestamp": "2024-01-15T10:30:02Z",
  "sequence_number": 2002,
  "data": {
    "payment_id": "pay_abc999",
    "user_id": "user_456",
    "requested_amount": 1000.0,
    "available_balance": 500.0,
    "total_balance": 500.0,
    "deficit": 500.0,
    "payment_type": "wallet"
  }
}
```

---

### 2.3 Eventos de External Payment Service

#### PaymentSentToGateway Event

```json
{
  "event_id": "evt_sent_001",
  "event_type": "PaymentSentToGateway",
  "aggregate_id": "pay_xyz789",
  "aggregate_type": "Payment",
  "event_version": 1,
  "timestamp": "2024-01-15T10:30:10Z",
  "sequence_number": 1004,
  "data": {
    "payment_id": "pay_xyz789",
    "gateway_provider": "external",
    "gateway_payment_id": "ext_txn_123",
    "payment_type": "external",
    "sent_at": "2024-01-15T10:30:10Z"
  }
}
```

#### PaymentGatewayResponse Event

```json
{
  "event_id": "evt_response_001",
  "event_type": "PaymentGatewayResponse",
  "aggregate_id": "pay_xyz789",
  "aggregate_type": "Payment",
  "event_version": 1,
  "timestamp": "2024-01-15T10:31:20Z",
  "sequence_number": 1005,
  "data": {
    "payment_id": "pay_xyz789",
    "gateway_provider": "external",
    "status": "SUCCESS",
    "transaction_id": "txn_ext_456",
    "payment_type": "external",
    "response_data": {
      "currency": "usd",
      "amount": 150000,
      "fee": 4500
    },
    "received_at": "2024-01-15T10:31:20Z"
  }
}
```

#### PaymentGatewayTimeout Event

```json
{
  "event_id": "evt_timeout_001",
  "event_type": "PaymentGatewayTimeout",
  "aggregate_id": "pay_xyz789",
  "aggregate_type": "Payment",
  "event_version": 1,
  "timestamp": "2024-01-15T10:30:45Z",
  "sequence_number": 1005,
  "data": {
    "payment_id": "pay_xyz789",
    "saga_id": "saga_123",
    "gateway_provider": "external",
    "attempt": 1,
    "max_attempts": 5,
    "timeout_duration_seconds": 30,
    "timeout_at": "2024-01-15T10:30:45Z"
  }
}
```

---

### 2.4 Command Events (SAGA Orchestrator → Services)

#### DebitFunds Command (Para Wallet Payments)

```json
{
  "event_id": "cmd_debit_001",
  "event_type": "DebitFunds",
  "aggregate_id": "user_123",
  "aggregate_type": "Wallet",
  "event_version": 1,
  "timestamp": "2024-01-15T10:30:03Z",
  "sequence_number": 1002,
  "data": {
    "command_id": "cmd_001",
    "payment_id": "pay_xyz789",
    "user_id": "user_123",
    "amount": 1500.0,
    "payment_type": "wallet",
    "saga_id": "saga_123"
  }
}
```

**Nota**: Para wallet payments, el débito es directo (ACID transaction). No se usan comandos separados de Lock/Unlock.

#### SendToGateway Command

```json
{
  "event_id": "cmd_send_001",
  "event_type": "SendToGateway",
  "aggregate_id": "pay_xyz789",
  "aggregate_type": "Payment",
  "event_version": 1,
  "timestamp": "2024-01-15T10:30:08Z",
  "sequence_number": 1004,
  "data": {
    "command_id": "cmd_003",
    "payment_id": "pay_xyz789",
    "saga_id": "saga_123",
    "gateway_provider": "external",
    "amount": 1500.0,
    "currency": "USD"
  }
}
```

---

## 3. Estructura de Topics (Redpanda/Kafka)

### 3.1 Estrategia Actual: Topic Único (Implementación MVP)

**IMPORTANTE:** Esta sección describe la **implementación actual del código**. La sección 3.2 describe la estrategia de múltiples topics para **escalabilidad futura**.

**Topics Implementados:**

```
payment_events    # Topic único con 12 particiones (todos los eventos de pagos)
events.dlq.v1     # Dead Letter Queue
```

**Estrategia de Diferenciación:**

Los eventos dentro del topic `payment_events` se diferencian por el campo `event_type`:

- `WalletPaymentRequested`, `FundsDebited`, `FundsInsufficient`, `WalletPaymentCompleted`, `WalletPaymentFailed`
- `ExternalPaymentRequested`, `PaymentSentToGateway`, `PaymentGatewayResponse`, `ExternalPaymentCompleted`, `ExternalPaymentFailed`

**Particionamiento:**

- **12 particiones totales**: 6 para wallet events (pares) + 6 para external events (impares)
- **Wallet Events**: Partition key = `user_id` (garantiza orden por usuario)
- **External Events**: Partition key = `payment_id` (garantiza orden por pago)

---

## 4. Orden de Eventos y Garantías de Entrega

### 4.1 Ordenamiento por Partición

**Particionamiento (Redpanda/Kafka):**

- Mismo `partition_key` → Misma partición → Orden garantizado
- Diferentes `partition_key` → Diferentes particiones → Procesamiento paralelo

**Ejemplo:**

```
Events para user_id="user_123":
  → Todos van a Partition 2
  → Procesados secuencialmente en orden

Events para user_id="user_456":
  → Todos van a Partition 4
  → Procesados secuencialmente en orden

Paralelismo: Partition 2 y Partition 4 procesan simultáneamente
```

### 4.2 Garantías de Entrega

#### Exactly-Once Semantics

**Configuración:**

```yaml
# Configuración para Redpanda/Kafka
producer:
  acks: all
  enable_idempotence: true
  max_in_flight_requests: 5

consumer:
  enable_auto_commit: false
  isolation_level: read_committed
```

**Implementación:**

```go
func (es *EventStore) PublishEvent(event Event) error {
    // 1. Store in PostgreSQL first (transactional)
    tx := es.postgres.Begin()
    if err := tx.Insert(event); err != nil {
        tx.Rollback()
        return err
    }

    // 2. Publish to Redpanda/Kafka via EventBus
    if err := es.eventBus.Publish(ctx, "events.payments.v1", event); err != nil {
        tx.Rollback()
        return err
    }

    // 3. Commit both
    tx.Commit()
    return nil
}
```

#### At-Least-Once Delivery

**Uso**: Eventos idempotentes (métricas, logging)

**Configuración:**

```yaml
acks: all
enable_idempotence: true
```

#### Idempotencia en Consumers

```go
func (ws *WalletService) ProcessEvent(event Event) error {
    // Check if already processed
    if ws.isEventProcessed(event.ID) {
        return nil // Idempotent - skip
    }

    // Process event
    err := ws.handleEvent(event)
    if err != nil {
        return err
    }

    // Mark as processed
    ws.markEventProcessed(event.ID)
    return nil
}
```

---

## 5. Event Sourcing - Almacenamiento Permanente

Implementamos dentro de nuestro Event Store una base de datos SQL para guardar todos los eventos que se reciban, de esta manera podemos extender el TTL de Redpanda (Kafka-compatible) y se puede lograr un servicio auditable perdurable en el tiempo.

### Garantías de Event Sourcing:

#### Durabilidad

- ✅ Eventos almacenados en PostgreSQL
- ✅ Replicación en Redpanda (Kafka-compatible)
- ✅ Backups automáticos de PostgreSQL Event Store

#### Consistencia

- ✅ Orden garantizado por partición
- ✅ Reconstrucción determinística desde eventos
- ✅ Estado derivado siempre consistente con eventos

### Auditoría

- ✅ Historial completo inmutable
- ✅ Time-travel debugging
- ✅ Trazabilidad completa de todas las operaciones

---

## 6. Implementación de SAGA Orchestration

### 6.1 Arquitectura del SAGA Orchestrator

Nuestra aplicación implementa un **SAGA Orchestrator** que coordina workflows de pago entre múltiples servicios usando **Event Sourcing puro**. El estado de las SAGAs se reconstruye completamente desde eventos almacenados en el Event Store, sin necesidad de una base de datos de estado de SAGAs separada.

**Componente Principal:**

```go
// internal/platform/saga/orchestrator.go
type Orchestrator struct {
    eventStore eventstore.EventStore  // Persistencia de eventos
    eventBus   eventbus.EventBus      // Comunicación asíncrona
    logger     logger.Logger
    sequence   int64                   // Secuencia de eventos
}
```

**Responsabilidades:**

- ✅ Crear nuevas SAGAs de pago (wallet y external)
- ✅ Procesar eventos de servicios downstream
- ✅ Reconstruir estado de SAGAs desde eventos cuando se necesita
- ✅ Coordinar transiciones de estado basadas en eventos recibidos
- ✅ Publicar eventos de finalización o fallo

### 6.2 Iniciación de SAGAs

#### Wallet Payment

```go
// POST /api/payments/wallet
func (o *Orchestrator) CreateWalletPayment(ctx context.Context, req CreateWalletPaymentRequest) (*PaymentResponse, error) {
    // 1. Generar IDs
    paymentID := uuid.New().String()
    sagaID := uuid.New().String()

    // 2. Crear evento inicial
    event := events.NewWalletPaymentRequested(
        paymentID, sagaID, req.UserID, req.ServiceID,
        req.Amount, req.Currency, metadata, sequence,
    )

    // 3. Guardar en Event Store
    o.eventStore.SaveEvent(ctx, event)

    // 4. Publicar en Event Bus (topic: payment_events)
    o.eventBus.Publish(ctx, "payment_events", event)

    return &PaymentResponse{
        PaymentID: paymentID,
        SagaID:    sagaID,
        Status:    "INITIALIZED",
    }, nil
}
```

**Flujo Resultante:**

```
1. HTTP POST → CreateWalletPayment()
2. Event Store: WalletPaymentRequested guardado
3. Event Bus: WalletPaymentRequested publicado a payment_events (partición par basada en user_id)
4. Wallet Service consume y procesa
```

### 6.3 Reconstrucción de Estado desde Eventos

**Característica Clave:** El estado de la SAGA se reconstruye desde eventos cada vez que se necesita, no se persiste en una tabla separada.

**Implementación:**

```go
// internal/platform/saga/orchestrator.go
func (o *Orchestrator) rebuildSagaFromEvents(ctx context.Context, sagaID, paymentID string) (*saga.Saga, error) {
    // 1. Cargar todos los eventos para este paymentID (aggregateID)
    eventsList, err := o.eventStore.LoadEvents(ctx, paymentID)
    if err != nil {
        return nil, err
    }

    // 2. Filtrar eventos por sagaID y extraer información inicial
    var userID, paymentType string
    var sagaEvents []events.Event

    for _, event := range eventsList {
        switch e := event.Data().(type) {
        case events.WalletPaymentRequestedData:
            if e.SagaID == sagaID {
                userID = e.UserID
                paymentType = "wallet"
                sagaEvents = append(sagaEvents, event)
            }
        case events.ExternalPaymentRequestedData:
            if e.SagaID == sagaID {
                userID = e.UserID
                paymentType = "external"
                sagaEvents = append(sagaEvents, event)
            }
        default:
            // Incluir otros eventos del mismo pago
            sagaEvents = append(sagaEvents, event)
        }
    }

    // 3. Crear saga y aplicar todos los eventos
    s := saga.NewSaga(sagaID, paymentID, userID, paymentType)
    for _, event := range sagaEvents {
        s.ApplyEvent(event) // Aplica evento y actualiza estado interno
    }

    return s, nil
}
```

### 6.4 Procesamiento de Eventos

El Orchestrator consume eventos del topic `payment_events` y procesa cada tipo de evento:

```go
// internal/platform/saga/orchestrator.go
func (o *Orchestrator) ProcessEvent(ctx context.Context, event events.Event) error {
    switch event.Type() {
    case "WalletPaymentRequested":
        return o.handleWalletPaymentRequested(ctx, event)

    case "ExternalPaymentRequested":
        return o.handleExternalPaymentRequested(ctx, event)

    case "FundsDebited":
        return o.handleFundsDebited(ctx, event)

    case "FundsInsufficient":
        return o.handleFundsInsufficient(ctx, event)

    case "PaymentSentToGateway":
        return o.handlePaymentSentToGateway(ctx, event)

    case "PaymentGatewayResponse":
        return o.handlePaymentGatewayResponse(ctx, event)
    }
    return nil
}
```

**Ejemplo: Procesamiento de Wallet Payment Completado**

### 6.5 Flujos de Pago Implementados

#### Flujo de Wallet Payment (Happy Path)

```
1. HTTP POST /api/payments/wallet
   ↓
2. Orchestrator.CreateWalletPayment()
   ├─ Genera paymentID y sagaID
   ├─ Crea WalletPaymentRequested event
   ├─ Guarda en Event Store
   └─ Publica en payment_events (partición par por user_id)

3. Wallet Service consume WalletPaymentRequested
   ├─ Reconstruye wallet state desde eventos
   ├─ Valida balance
   ├─ Debita fondos (ACID transaction)
   ├─ Guarda FundsDebited event
   └─ Publica FundsDebited en payment_events (misma partición)

4. Orchestrator consume FundsDebited
   ├─ Reconstruye saga desde eventos (rebuildSagaFromEvents)
   ├─ Aplica evento → Transición: COMPLETED
   ├─ Guarda WalletPaymentCompleted event
   └─ Publica WalletPaymentCompleted en payment_events

5. Metrics Service consume WalletPaymentCompleted
   └─ Registra métricas
```

#### Flujo de Wallet Payment (Insufficient Balance)

```
1-2. (Igual que happy path)

3. Wallet Service consume WalletPaymentRequested
   ├─ Reconstruye wallet state
   ├─ Valida balance → INSUFFICIENT
   ├─ Guarda FundsInsufficient event
   └─ Publica FundsInsufficient en payment_events

4. Orchestrator consume FundsInsufficient
   ├─ Reconstruye saga desde eventos
   ├─ Aplica evento → Transición: FAILED
   ├─ Guarda WalletPaymentFailed event
   └─ Publica WalletPaymentFailed en payment_events
```

#### Flujo de External Payment

```
1. HTTP POST /api/payments/creditcard
   ↓
2. Orchestrator.CreateExternalPayment()
   └─ Publica ExternalPaymentRequested en payment_events (partición impar por payment_id)

3. External Payment Service consume ExternalPaymentRequested
   ├─ Llama External Gateway (mock)
   ├─ Guarda PaymentSentToGateway event
   └─ Publica PaymentSentToGateway en payment_events (misma partición)

4. Orchestrator consume PaymentSentToGateway
   ├─ Reconstruye saga
   ├─ Aplica evento → Transición: SENT_TO_GATEWAY
   └─ (Espera PaymentGatewayResponse)

5. External Service simula webhook (goroutine)
   ├─ Guarda PaymentGatewayResponse event
   └─ Publica PaymentGatewayResponse en payment_events

6. Orchestrator consume PaymentGatewayResponse
   ├─ Reconstruye saga
   ├─ Evalúa response.Status:
   │  ├─ SUCCESS → Publica ExternalPaymentCompleted
   │  └─ FAILED → Publica ExternalPaymentFailed
   └─ Transición: COMPLETED o FAILED
```

### 6.6 Estados de SAGA (State Machine)

**Estados Definidos:**

```go
// internal/domain/saga/state.go
type SagaState string

const (
    SagaInitialized      SagaState = "INITIALIZED"
    SagaValidatingBalance SagaState = "VALIDATING_BALANCE"  // Wallet
    SagaSendingToGateway SagaState = "SENDING_TO_GATEWAY"   // External
    SagaSentToGateway    SagaState = "SENT_TO_GATEWAY"       // External
    SagaAwaitingResponse SagaState = "AWAITING_RESPONSE"     // External
    SagaCompleted        SagaState = "COMPLETED"
    SagaFailed           SagaState = "FAILED"
)
```

**Transiciones Válidas:**

```go
// Wallet Payment:
INITIALIZED → VALIDATING_BALANCE → [COMPLETED | FAILED]

// External Payment:
INITIALIZED → SENDING_TO_GATEWAY → SENT_TO_GATEWAY → AWAITING_RESPONSE → [COMPLETED | FAILED]
```

**Validación de Transiciones:**

```go
func (s SagaState) CanTransitionTo(target SagaState) bool {
    validTransitions := map[SagaState][]SagaState{
        SagaInitialized:       {SagaValidatingBalance},
        SagaValidatingBalance: {SagaCompleted, SagaFailed},
        SagaSendingToGateway:  {SagaSentToGateway},
        SagaSentToGateway:    {SagaAwaitingResponse},
        SagaAwaitingResponse:  {SagaCompleted, SagaFailed},
        // Estados terminales no tienen transiciones
        SagaCompleted: {},
        SagaFailed:    {},
    }
    // Validación de transición...
}
```

### 6.7 Consulta de Estado de Pago

**Endpoint:** `GET /api/v1/payments/:id`

**Implementación:**

```go
func (o *Orchestrator) GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentStatus, error) {
    // 1. Cargar eventos para paymentID
    eventsList, err := o.eventStore.LoadEvents(ctx, paymentID)
    if err != nil {
        return nil, err
    }

    // 2. Extraer sagaID del primer evento
    var sagaID string
    if len(eventsList) > 0 {
        switch e := eventsList[0].Data().(type) {
        case events.WalletPaymentRequestedData:
            sagaID = e.SagaID
        case events.ExternalPaymentRequestedData:
            sagaID = e.SagaID
        }
    }

    // 3. Reconstruir saga desde eventos
    s, err := o.rebuildSagaFromEvents(ctx, sagaID, paymentID)
    if err != nil {
        return nil, err
    }

    // 4. Retornar estado actual
    return &PaymentStatus{
        PaymentID: paymentID,
        SagaID:    sagaID,
        Status:    string(s.CurrentState()),
    }, nil
}
```

### 6.8 Ventajas

1. **Simplicidad Arquitectónica**

   - No requiere SAGA State Store separado
   - Event Store es fuente única de verdad
   - Menos componentes = menos complejidad

2. **Trazabilidad Completa**

   - Cada paso registrado como evento inmutable
   - Historial completo de cada SAGA
   - Capacidad de replay y debugging

3. **Escalabilidad**

   - Cada servicio escala independientemente
   - Particionamiento permite paralelismo
   - Event sourcing permite reconstrucción bajo demanda

4. **Resiliencia**

   - Recuperación automática desde Event Store
   - Sin pérdida de estado en fallos
   - Idempotencia garantizada por event_id

5. **Flexibilidad**
   - Agregar nuevos pasos es solo agregar nuevos eventos
   - No requiere cambios en servicios existentes
   - Fácil agregar nuevos tipos de pago
