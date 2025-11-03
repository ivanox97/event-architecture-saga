# Estrategia de Manejo de Errores

## 1. Identificación de Escenarios de Falla

### 1.1 Fallas Durante Ejecución de SAGA

#### Wallet Payment - Wallet Service Failure

**Escenario:**

```
Causa: Wallet Service falla después de recibir WalletPaymentRequested
Flujo Actual:
  1. Wallet Service consume WalletPaymentRequested
  2. Reconstruye wallet state desde eventos (RebuildWalletState)
  3. Valida balance desde estado reconstruido
  4. Si suficiente: Publica FundsDebited event
  5. Si insuficiente: Publica FundsInsufficient event
```

**Impacto según Momento de Falla:**

| Momento de Falla                                     | Impacto                                       | Estado de SAGA              | Recuperación                     |
| ---------------------------------------------------- | --------------------------------------------- | --------------------------- | -------------------------------- |
| **Antes de validar balance**                         | Evento no procesado                           | `VALIDATING_BALANCE`        | Replay evento desde Event Store  |
| **Después de validar pero antes de publicar evento** | Validación realizada pero evento no publicado | `VALIDATING_BALANCE`        | Reprocesar evento (idempotente)  |
| **Después de publicar FundsDebited**                 | Evento publicado exitosamente                 | `COMPLETED` (eventualmente) | Sin acción (evento ya procesado) |

**Recuperación:**

```go
// Al reiniciar Wallet Service:
// 1. Cargar eventos pendientes desde Event Store
// 2. Reprocesar eventos según partition offset
// 3. Eventos son idempotentes (event_id verificación)
```

**No hay compensación necesaria:**

- Si falló antes de publicar FundsDebited: No hubo débito
- Si falló después: Evento ya publicado, SAGA continuará normalmente
- Estado siempre reconstruible desde eventos

#### Wallet Payment - Saldo Insuficiente (No es Falla, es Validación)

**Escenario:**

```
Flujo:
  1. WalletPaymentRequested recibido
  2. Reconstruye wallet state desde eventos
  3. Valida: available_balance < amount
  4. Publica FundsInsufficient event inmediatamente
```

**Características:**

- ✅ **Rechazo inmediato**: No hay débito, solo validación
- ✅ **Sin estado intermedio**: No hay fondos bloqueados
- ✅ **Evento de fallo claro**: `FundsInsufficient` publicado

**No requiere compensación:**

- No se modificó ningún estado
- Solo se publicó evento de rechazo

#### External Payment - External Payment Service Failure

**Escenario:**

```
Causa: External Payment Service falla después de recibir ExternalPaymentRequested
Flujo Actual:
  1. External Payment Service consume ExternalPaymentRequested
  2. Llama External Gateway
  3. Publica PaymentSentToGateway event
  4. Recibe webhook asíncrono (goroutine)
  5. Publica PaymentGatewayResponse event
```

**Impacto según Momento de Falla:**

| Momento de Falla                                            | Impacto                                   | Estado de SAGA         | Recuperación                    |
| ----------------------------------------------------------- | ----------------------------------------- | ---------------------- | ------------------------------- |
| **Antes de llamar gateway**                                 | Pago no enviado                           | `SENDING_TO_GATEWAY`   | Replay evento desde Event Store |
| **Después de PaymentSentToGateway pero antes de respuesta** | Gateway recibió pago, respuesta pendiente | `AWAITING_RESPONSE`    | Esperar webhook o timeout       |
| **Después de PaymentGatewayResponse**                       | Respuesta procesada                       | `COMPLETED` o `FAILED` | Sin acción                      |

**No hay compensación necesaria:**

- Gateway externo maneja su propia reversión si es necesario

#### External Payment - Gateway Timeout

**Escenario:**

```
Causa: External Gateway no responde en tiempo esperado (30s)
Estado Actual: ✅ Timeout detection y retry logic implementados
Comportamiento: Service detecta timeout, reintenta con exponential backoff
```

**Implementación Actual:**

```go
// Timeout por intento: 30 segundos
// Max intentos: 5
// Exponential backoff: 5s, 10s, 20s, 40s, 60s (max)
// Jitter: Habilitado para evitar thundering herd
```

**Flujo Implementado:**

```
1. ExternalPaymentRequested recibido
2. Intentar llamada al gateway con timeout de 30s
   ↓ Timeout
3. Publicar PaymentGatewayTimeout event (attempt 1)
4. Esperar 5s (exponential backoff)
5. Publicar PaymentRetryRequested event (attempt 2)
6. Reintentar llamada al gateway
   ↓ Timeout (si persiste)
7. Repetir hasta max 5 intentos
8. Si todos fallan: Publicar ExternalPaymentFailed {reason: "MAX_RETRIES_EXCEEDED"}
```

**Impacto:**

- ✅ **Timeout detection automático**: Cada intento tiene timeout de 30s
- ✅ **Reintentos automáticos**: Max 5 intentos con exponential backoff
- ✅ **Eventos publicados**: PaymentGatewayTimeout y PaymentRetryRequested para trazabilidad
- ⏳ **DLQ**: No implementado (futuro) - por ahora solo log que iría a DLQ

#### SAGA Orchestrator Failure

**Escenario:**

```
Causa: SAGA Orchestrator cae durante procesamiento de eventos
Estado Actual: Event Sourcing puro - estado se reconstruye desde eventos
```

**Recuperación Automática:**

```go
// Al reiniciar SAGA Orchestrator:
func (o *Orchestrator) Recover() error {
    // 1. SAGA Orchestrator se subscribe a payment_events
    // 2. Procesa eventos desde último offset consumido
    // 3. Para cada evento: rebuildSagaFromEvents() → ApplyEvent() → Continuar
    // 4. Estado siempre actualizado desde eventos
}
```

**Características:**

- ✅ **Recuperación automática**: Estado se reconstruye desde eventos
- ✅ **Sin pérdida de progreso**: Todo registrado en Event Store
- ✅ **No requiere intervención manual**: Event Sourcing permite replay

**Ejemplo de Recuperación:**

```go
// Evento recibido después de restart
func (o *Orchestrator) ProcessEvent(ctx context.Context, event events.Event) error {
    // Reconstruir estado actual desde eventos
    s, err := o.rebuildSagaFromEvents(ctx, sagaID, paymentID)
    if err != nil {
        return err
    }

    // Aplicar nuevo evento
    s.ApplyEvent(event)

    // Continuar procesamiento según estado actual
    return o.continueSagaProcessing(ctx, s, event)
}
```

### 1.2 Fallas de Infraestructura

#### Event Store (PostgreSQL) Failure

**Escenario:**

```
Causa: PostgreSQL Event Store no disponible temporalmente
Impacto: No se pueden guardar nuevos eventos
```

**Comportamiento Actual:**

```go
// Event Store SaveEvent falla
func (o *Orchestrator) CreateWalletPayment(ctx context.Context, req CreateWalletPaymentRequest) error {
    // Save event to Event Store
    if err := o.eventStore.SaveEvent(ctx, event); err != nil {
        // Error inmediato al cliente
        return fmt.Errorf("failed to save event: %w", err)
    }
    // ...
}
```

** Debido a la los reintentos se reintenta guardar los eventos y si se cumple el maximo de reintentos cae a la DLQ para tratar el error manualmente**

#### Partición Kafka Loss

**Escenario:**

```
Causa: Pérdida de partición Kafka
Impacto: Eventos no se procesan desde esa partición
```

**Recuperación:**

- Replay desde PostgreSQL Event Store (fuente de verdad)
- Eventos siempre disponibles en Event Store
- Reconstrucción de estado garantizada

---

## 2. Estrategias de Reintento y Políticas de Retroceso Exponencial

### 2.1 Estado Actual: Retry Logic Implementado para External Payments

**MVP Implementado:**

- ✅ **Retry logic con exponential backoff**: Implementado para External Payment Service
- ✅ **Timeout detection**: 30s por intento
- ✅ **Max 5 intentos**: Configurable mediante `RetryPolicy`
- ✅ **Eventos de timeout y retry**: `PaymentGatewayTimeout` y `PaymentRetryRequested` publicados
- ✅ **Eventos son idempotentes**: Permite replay seguro
- ⏳ **DLQ**: No implementado (futuro) - solo log cuando se exceden max retries

**Comportamiento Actual:**

```go
// External Payment Service - Con retry logic
func (s *Service) HandleExternalPaymentRequested(ctx context.Context, event events.Event) error {
    return s.processPaymentWithRetry(ctx, paymentData, metadata)
}

// processPaymentWithRetry implementa:
// - Timeout de 30s por intento
// - Max 5 intentos
// - Exponential backoff: 5s, 10s, 20s, 40s, 60s (max)
// - Jitter para evitar thundering herd
// - Publica eventos: PaymentGatewayTimeout, PaymentRetryRequested
```

### 2.2 Implementación Actual: Exponential Backoff para Gateway Calls

**Configuración Implementada:**

```yaml
retry_policy:
  external_payment_gateway:
    max_attempts: 5
    initial_delay: 5s
    max_delay: 60s
    multiplier: 2.0
    jitter: true

  event_processing:
    max_attempts: 10
    initial_delay: 500ms
    max_delay: 30s
    multiplier: 1.5
    jitter: true
```

**Código Implementado (Referencia):**

```go
// RetryPolicy define la política de reintentos
type RetryPolicy struct {
    MaxAttempts  int
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
    Jitter       bool
}

// processPaymentWithRetry implementa el retry logic con exponential backoff
// Ubicación: app/internal/platform/externalpayment/service.go
func (s *Service) processPaymentWithRetry(ctx context.Context, paymentData events.ExternalPaymentRequestedData, metadata events.EventMetadata) error {
    // Configuración: MaxAttempts=5, InitialDelay=5s, MaxDelay=60s, Multiplier=2.0
    // Cada intento tiene timeout de 30s
    // Publica eventos: PaymentGatewayTimeout y PaymentRetryRequested
    // Si max retries: ExternalPaymentFailed (reason: "MAX_RETRIES_EXCEEDED")
}
```

**Secuencia de Reintentos Implementada:**

```
Attempt 1: Inmediato (timeout: 30s)
  ↓ Timeout → PaymentGatewayTimeout event (attempt 1)
  ↓ Wait 5s (initial_delay)
Attempt 2: PaymentRetryRequested event (attempt 2)
  ↓ Timeout → PaymentGatewayTimeout event (attempt 2)
  ↓ Wait 10s (5s * 2.0)
Attempt 3: PaymentRetryRequested event (attempt 3)
  ↓ Timeout → PaymentGatewayTimeout event (attempt 3)
  ↓ Wait 20s (10s * 2.0)
Attempt 4: PaymentRetryRequested event (attempt 4)
  ↓ Timeout → PaymentGatewayTimeout event (attempt 4)
  ↓ Wait 40s (20s * 2.0, max 60s)
Attempt 5: PaymentRetryRequested event (attempt 5)
  ↓ Timeout → PaymentGatewayTimeout event (attempt 5)
Max retries exceeded → ExternalPaymentFailed event (reason: "MAX_RETRIES_EXCEEDED")
  ↓ (Futuro: DLQ) - Por ahora solo log
```

### 2.3 Clasificación de Errores para Reintentos

**Errores Transitorios (Retryable):**

```go
func isRetryableError(err error) bool {
    // Timeouts
    if isTimeout(err) {
        return true
    }

    // Network errors
    if isNetworkError(err) {
        return true
    }

    // Gateway temporarily unavailable (5xx)
    if isTemporaryGatewayError(err) {
        return true
    }

    return false
}
```

**Errores Permanentes (No Retryable):**

```go
func isPermanentError(err error) bool {
    // Business rule violations
    if isValidationError(err) {
        return true
    }

    // Gateway rejected (4xx)
    if isGatewayRejection(err) {
        return true
    }

    // Insufficient funds (gateway)
    if isInsufficientFundsError(err) {
        return true
    }

    return false
}
```

### 2.4 Reintentos en Event Processing

**Estrategia para Event Consumers:**

```go
// Futuro: Retry en event processing
func (s *Service) ProcessEventWithRetry(ctx context.Context, event events.Event) error {
    policy := RetryPolicy{
        MaxAttempts:  10,
        InitialDelay: 500 * time.Millisecond,
        MaxDelay:     30 * time.Second,
        Multiplier:   1.5,
        Jitter:       true,
    }

    attempt := 0
    for attempt < policy.MaxAttempts {
        err := s.ProcessEvent(ctx, event)
        if err == nil {
            return nil
        }

        // Verificar si es error recuperable
        if !isRetryableError(err) {
            // Error permanente - enviar a DLQ
            return s.routeToDLQ(ctx, event, "PERMANENT_ERROR")
        }

        attempt++
        if attempt < policy.MaxAttempts {
            delay := calculateDelay(attempt, policy)
            time.Sleep(delay)
        }
    }

    // Max retries exceeded - DLQ
    return s.routeToDLQ(ctx, event, "MAX_RETRIES_EXCEEDED")
}
```

**Beneficios de Exponential Backoff:**

- ✅ Reduce carga en sistema caído durante recuperación
- ✅ Evita thundering herd con jitter
- ✅ Balance entre latencia y recuperación
- ✅ Configurable por tipo de operación

---

## 3. Manejo de Colas de Mensajes Fallidos (Dead Letter Queue)

**Implementación:**

```go
// DLQ Simulator (mock) - app/internal/infrastructure/dlq/dlq.go
// Implementación mock para DLQ - en producción sería un topic de Redpanda/Kafka compartido (events.dlq.v1)
// Nota: El Event Bus principal ya usa Redpanda (Kafka-compatible), pero el DLQ sigue siendo mock en el MVP
type DLQSimulator struct {
    events    []DLQEvent
    consumers []DLQHandler
    running   bool
}

// External Payment Service publica a DLQ cuando max retries exceeded
// Metrics Service consume de DLQ y persiste en DB Errors
```

### 3.2 Estrategia de DLQ Propuesta

**Eventos van a DLQ cuando:**

1. **Max retries exceeded**: Evento excedió máximo de reintentos
2. **Event malformado**: Schema validation failed
3. **Consumer crash repetido**: Evento no puede ser procesado después de múltiples intentos
4. **Event muy antiguo**: Evento expiró (timestamp muy antiguo)

**Topic DLQ:**

```
events.dlq.v1    # Dead Letter Queue topic
```

### 3.3 Estructura de DLQ Event

```json
{
  "dlq_event_id": "dlq_001",
  "original_event": {
    "event_id": "evt_001",
    "event_type": "ExternalPaymentRequested",
    "aggregate_id": "pay_xyz789",
    "data": {
      "payment_id": "pay_xyz789",
      "saga_id": "saga_123",
      "amount": 1500.0
    }
  },
  "failure_reason": "MAX_RETRIES_EXCEEDED",
  "failure_count": 5,
  "first_failure_at": "2024-01-15T10:30:00Z",
  "last_attempt_at": "2024-01-15T10:35:00Z",
  "consumer_group": "external-payment-service-group",
  "original_topic": "payment_events",
  "original_partition": 3,
  "original_offset": 12345,
  "error_details": {
    "error_type": "GATEWAY_TIMEOUT",
    "error_message": "Gateway did not respond within 30s",
    "retry_history": [
      { "attempt": 1, "timestamp": "2024-01-15T10:30:05Z", "error": "TIMEOUT" },
      { "attempt": 2, "timestamp": "2024-01-15T10:30:15Z", "error": "TIMEOUT" },
      { "attempt": 3, "timestamp": "2024-01-15T10:30:30Z", "error": "TIMEOUT" },
      { "attempt": 4, "timestamp": "2024-01-15T10:31:00Z", "error": "TIMEOUT" },
      { "attempt": 5, "timestamp": "2024-01-15T10:32:00Z", "error": "TIMEOUT" }
    ]
  }
}
```

### 3.4 Routing a DLQ

**Implementación Actual:**

```go
func (s *Service) routeToDLQ(ctx context.Context, event events.Event, reason string) error {
    dlqEvent := DLQEvent{
        DLQEventID:    uuid.New().String(),
        OriginalEvent: event,
        FailureReason: reason,
        FirstFailureAt: time.Now(),
        RetryCount:    s.getRetryCount(event.ID()),
        ConsumerGroup: s.consumerGroup,
        OriginalTopic: "payment_events",
        OriginalPartition: event.Partition(),
        OriginalOffset: event.Offset(),
        ErrorDetails: s.getErrorDetails(event.ID()),
    }

    // Publicar a DLQ topic
    return s.eventBus.Publish(ctx, "events.dlq.v1", dlqEvent)
}
```

### 3.5 Procesamiento de DLQ

**DLQ Handler (Metrics Service):**

```go
// Metrics Service consume eventos de DLQ
func (ms *MetricsService) HandleDLQEvent(ctx context.Context, dlqEvent DLQEvent) error {
    // 1. Analizar si es recuperable
    if ms.isRecoverable(dlqEvent) {
        // Republicar a topic original
        return ms.republishToOriginalTopic(ctx, dlqEvent)
    }

    // 2. Persistir permanentemente en DB Errors
    return ms.persistToDBErrors(ctx, dlqEvent)
}
```

**Persistencia en DB Errors (Implementado):**

```sql
CREATE TABLE error_logs (
    error_id UUID PRIMARY KEY,
    dlq_event_id UUID NOT NULL,
    payment_id VARCHAR(255),
    saga_id VARCHAR(255),
    error_type VARCHAR(100),
    error_reason TEXT,
    original_event JSONB NOT NULL,
    failure_details JSONB,
    retry_history JSONB,
    first_occurred_at TIMESTAMP NOT NULL,
    last_occurred_at TIMESTAMP NOT NULL,
    resolved BOOLEAN DEFAULT FALSE,
    resolved_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    INDEX idx_payment_id (payment_id),
    INDEX idx_unresolved (resolved, created_at),
    INDEX idx_error_type (error_type, created_at)
);
```

**Procesamiento de DB Errors:**

```go
// app/internal/infrastructure/dberrors/dberrors.go
func (dbe *DBErrors) PersistDLQEvent(ctx context.Context, dlqEvent dlq.DLQEvent) error {
    errorLog := ErrorLog{
        ErrorID:        uuid.New(),
        DLQEventID:     dlqEvent.DLQEventID,
        PaymentID:      extractPaymentID(dlqEvent.OriginalEvent),
        SagaID:         extractSagaID(dlqEvent.OriginalEvent),
        ErrorType:      classifyError(dlqEvent.FailureReason),
        ErrorReason:    dlqEvent.FailureReason,
        OriginalEvent:  dlqEvent.OriginalEvent,
        FailureDetails: dlqEvent.ErrorDetails,
        RetryHistory:   dlqEvent.ErrorDetails.RetryHistory,
        FirstOccurredAt: dlqEvent.FirstFailureAt,
        LastOccurredAt:  dlqEvent.LastAttemptAt,
        Resolved:       false,
    }

    return dbe.db.Insert("error_logs", errorLog)
}
```

### 3.6 Flujo Completo: Error → DLQ → DB Errors (Implementado)

```
1. External Payment Service intenta procesar pago
   ↓ Timeout
2. Reintentos (max 5) con exponential backoff
   ↓ Todos fallan
3. External Payment Service publica ExternalPaymentFailed
   ↓
4. External Payment Service route to DLQ (dlq.Publish)
   ↓
5. DLQ Simulator almacena evento en memoria
   ↓
6. Metrics Service consume DLQ event (subscribe)
   ↓
7. Metrics Service persiste a DB Errors (error_logs table)
   ↓
8. SAGA Orchestrator consume ExternalPaymentFailed (payment_events)
   ↓
9. SAGA Estado: FAILED
```

**Implementación Actual:**

- ✅ External Payment Service publica a DLQ cuando `handleMaxRetriesExceeded`
- ✅ Metrics Service subscribe a DLQ y consume eventos automáticamente
- ✅ Metrics Service persiste eventos en `error_logs` table usando `DBErrors.PersistDLQEvent`
- ✅ Tabla `error_logs` permite consulta manual, alertas, y análisis

**Nota:** En producción, DLQ sería un topic compartido de Kafka/Redpanda (`events.dlq.v1`). En el MVP, cada servicio tiene su instancia de DLQ Simulator (mock), pero el flujo funciona igual. El Event Bus principal ya usa Redpanda (Kafka-compatible) en lugar de un simulador.

---

## 4. Transacciones Compensatorias

### 4.1 Principio: Sin Compensación Automática para Pagos Normales

**Wallet Payments:**

- ✅ Validación ocurre **antes** de cualquier operación irreversible
- ✅ Si falla validación: `FundsInsufficient` → Rechazo inmediato (sin débito)
- ✅ Si pasa validación: `FundsDebited` → Débito exitoso (no requiere compensación)
- ✅ **No hay bloqueo temporal**: Débito es directo y atómico

**External Payments:**

- ✅ Si falla: `ExternalPaymentFailed` → No hay nada que compensar
- ✅ Gateway externo maneja su propia reversión si es necesario

### 4.2 Compensación Solo para Casos Especiales

**Refunds Manuales:**

```go
// Caso especial: Refund de un pago wallet exitoso
// NO parte del flujo normal de pago
func (o *Orchestrator) ProcessRefundRequest(ctx context.Context, refund RefundRequest) error {
    // Validar que pago original fue exitoso
    saga, err := o.rebuildSagaFromEvents(ctx, refund.OriginalSagaID, refund.OriginalPaymentID)
    if err != nil {
        return err
    }

    if saga.CurrentState() != saga.SagaCompleted {
        return fmt.Errorf("payment not completed, cannot refund")
    }

    // Publicar evento de refund (compensación manual)
    refundEvent := events.NewRefundRequested(
        refund.RefundID,
        refund.OriginalPaymentID,
        refund.UserID,
        refund.Amount,
        refund.Reason,
    )

    o.eventStore.SaveEvent(ctx, refundEvent)
    o.eventBus.Publish(ctx, "payment_events", refundEvent)

    // Wallet Service procesará y publicará FundsCredited
    return nil
}
```

**Flujo de Refund (Compensación Manual):**

```
1. RefundRequested Event
   ↓
2. Wallet Service consume RefundRequested
   ↓
3. Valida pago original fue exitoso (desde eventos)
   ↓
4. Publica FundsCredited Event
   ↓
5. Wallet state reconstruido desde eventos incluye crédito
```

### 4.3 Por Qué No Hay Compensación Automática

**Wallet Payment - Insufficient Balance:**

```
Flujo:
  1. WalletPaymentRequested
   ↓
  2. Rebuild wallet state (desde eventos)
   ↓
  3. Validate: balance < amount → INSUFFICIENT
   ↓
  4. FundsInsufficient event publicado
   ↓
  5. WalletPaymentFailed event publicado
```

**No requiere compensación:**

- ✅ No se debitó nada (validación ocurre antes)
- ✅ Estado no se modificó
- ✅ Solo eventos de rechazo publicados

**External Payment - Gateway Timeout:**

```
Flujo:
  1. ExternalPaymentRequested
   ↓
  2. PaymentSentToGateway
   ↓
  3. Gateway timeout (max retries)
   ↓
  4. ExternalPaymentFailed
```

**No requiere compensación:**

- ✅ No hay fondos bloqueados
- ✅ Gateway externo maneja reversión si es necesario

### 4.4 Eventos de Compensación (Solo Refunds)

**RefundRequested Event:**

```json
{
  "event_id": "evt_refund_001",
  "event_type": "RefundRequested",
  "aggregate_id": "refund_123",
  "aggregate_type": "Refund",
  "event_version": 1,
  "timestamp": "2024-01-15T10:31:26Z",
  "sequence_number": 2001,
  "data": {
    "refund_id": "refund_123",
    "original_payment_id": "pay_xyz789",
    "user_id": "user_123",
    "amount": 1500.0,
    "reason": "MERCHANT_REFUND",
    "payment_type": "wallet",
    "initiated_at": "2024-01-15T10:31:26Z"
  }
}
```

**FundsCredited Event (Resultado de Refund):**

```json
{
  "event_id": "evt_credit_001",
  "event_type": "FundsCredited",
  "aggregate_id": "user_123",
  "aggregate_type": "Wallet",
  "event_version": 1,
  "timestamp": "2024-01-15T10:31:27Z",
  "sequence_number": 2002,
  "data": {
    "payment_id": "refund_123",
    "user_id": "user_123",
    "amount": 1500.0,
    "previous_balance": 3500.0,
    "new_balance": 5000.0,
    "reason": "REFUND",
    "credited_at": "2024-01-15T10:31:27Z"
  }
}
```

**Nota:** Estos eventos son para **refunds manuales**, NO para compensación automática de flujos normales de pago.

---

## 6. Recuperación desde Event Sourcing Puro

### 6.1 Recuperación Automática de SAGAs

**Característica Clave:** Con Event Sourcing puro, el estado siempre se puede reconstruir desde eventos.

**Ejemplo: SAGA Orchestrator se reinicia**

```go
// Al iniciar SAGA Orchestrator:
func (o *Orchestrator) Start() {
    // 1. Subscribe a payment_events
    o.eventBus.Subscribe(ctx, "payment_events", o.ProcessEvent)

    // 2. Procesar eventos desde último offset
    //    (Redpanda/Kafka consumer group maneja esto automáticamente)

    // 3. Para cada evento recibido:
    //    - rebuildSagaFromEvents() obtiene estado actual
    //    - ApplyEvent() actualiza estado
    //    - Continúa procesamiento normal
}
```

**Ejemplo: Wallet Service se reinicia**

```go
// Al iniciar Wallet Service:
func (s *Service) Start() {
    // 1. Subscribe a payment_events
    s.eventBus.Subscribe(ctx, "payment_events", func(ctx context.Context, event events.Event) error {
        if event.Type() == "WalletPaymentRequested" {
            return s.HandleWalletPaymentRequested(ctx, event)
        }
        return nil
    })

    // 2. Procesa eventos desde último offset
    // 3. RebuildWalletState() reconstruye estado desde eventos cada vez
    // 4. No requiere recuperación especial - estado siempre derivable
}
```

### 6.2 Sin Pérdida de Estado

**Ventajas de Event Sourcing Puro:**

- ✅ **Estado siempre reconstruible**: Cada vez que se necesita, se reconstruye desde eventos
- ✅ **No requiere checkpoint**: No hay estado persistido separado que pueda corromperse
- ✅ **Trazabilidad completa**: Historial completo de todos los cambios
- ✅ **Time-travel**: Puede recrear estado en cualquier punto histórico

**Comparación con Estado Persistido:**

| Aspecto              | Estado Persistido             | Event Sourcing Puro               |
| -------------------- | ----------------------------- | --------------------------------- |
| **Recuperación**     | Requiere checkpoint válido    | Siempre desde eventos             |
| **Corrupción**       | Puede corromperse             | Eventos inmutables, no corrupción |
| **Pérdida de datos** | Posible si checkpoint perdido | Imposible si eventos guardados    |
| **Trazabilidad**     | Limitada                      | Completa (historial inmutable)    |

---

## 7. Resumen de Estrategias Implementadas vs Futuras

### 7.1 Implementado en MVP

| Estrategia                                   | Detalles                                          |
| -------------------------------------------- | ------------------------------------------------- |
| **Event Sourcing Puro**                      | Estado reconstruido desde eventos                 |
| **Recuperación desde Event Store**           | `rebuildSagaFromEvents()`                         |
| **Idempotencia**                             | Event ID verification                             |
| **Rechazo inmediato (insufficient balance)** | `FundsInsufficient` event                         |
| **Sin compensación automática**              | No requerida para flujos normales                 |
| **Timeout Detection**                        | 30s por intento en External Payment Service       |
| **Retry Logic con Exponential Backoff**      | Max 5 intentos con backoff 5s→10s→20s→40s→60s     |
| **Eventos de Timeout y Retry**               | `PaymentGatewayTimeout` y `PaymentRetryRequested` |
| **Dead Letter Queue (DLQ)**                  | DLQ Simulator mock + routing automático           |
| **DB Errors Persistence**                    | Tabla `error_logs` con persistencia permanente    |

### 7.2 Documentado pero No Implementado

| Estrategia          | Estado    | Prioridad       | Documentación |
| ------------------- | --------- | --------------- | ------------- |
| **Circuit Breaker** | ⏳ Futuro | Baja (opcional) | Sección 5     |
