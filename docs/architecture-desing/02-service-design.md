# Documento de Diseño de Servicios - Arquitectura Event-Driven

## 1. SAGA Orchestrator Service

### 1.1 Límites del Servicio

**Responsabilidades:**

- **Iniciar SAGAs**: Crea instancias de SAGA cuando se recibe solicitud de pago
- **Orquestar Workflows**: Coordina pasos secuenciales de la SAGA mediante eventos
- **Gestionar Estado**: Mantiene estado de cada SAGA (state machine)
- **Manejar Compensaciones**: Ejecuta compensaciones automáticas cuando detecta fallos
- **Timeout Handling**: Gestiona timeouts y dispara compensaciones automáticas
- **Publicar Comandos**: Publica comandos al Event Store para servicios

**No es responsable de:**

- Acceso directo a bases de datos (todo vía eventos)
- Lógica de negocio específica (delegado a servicios especializados)
- Procesamiento de pagos (External Payment Service)
- Gestión de saldos (Wallet Service)

### 1.2 Interfaces

#### REST API

```http
POST   /api/v1/payments                # Iniciar pago (crea SAGA)
GET    /api/v1/payments/{id}            # Consultar estado de pago/SAGA
GET    /api/v1/sagas/{id}              # Consultar estado de SAGA
GET    /api/v1/sagas?status=PENDING   # Listar SAGAs por estado
```

### 1.3 State Machine de SAGA

```go
type PaymentSaga struct {
    SagaID       string
    PaymentID    string
    UserID       string
    CurrentState SagaState
    Steps        []SagaStep
    Completed    []SagaStep
    Failed       bool
    FailureReason string
    CreatedAt    time.Time
    LastActivity time.Time
    Version      int
}

type SagaState string

const (
    // Common states
    SagaInitialized      SagaState = "INITIALIZED"
    SagaCompleted        SagaState = "COMPLETED"
    SagaFailed           SagaState = "FAILED"
    SagaFinalized        SagaState = "FINALIZED"

    // Wallet payment states
    SagaValidatingBalance SagaState = "VALIDATING_BALANCE"

    // External payment states
    SagaSendingToGateway SagaState = "SENDING_TO_GATEWAY"
    SagaSentToGateway    SagaState = "SENT_TO_GATEWAY"
    SagaAwaitingResponse SagaState = "AWAITING_RESPONSE"
)

type SagaStep struct {
    Name          string
    Command       CommandEvent
    Compensation  CommandEvent
    Completed     bool
    CompletedAt   time.Time
    Failed        bool
    FailedAt      time.Time
}
```

### 1.4 Eventos Publicados

```yaml
PaymentRequested:
  saga_id: string
  payment_id: string
  user_id: string
  service_id: string
  amount: decimal
  currency: string
  timestamp: datetime

PaymentProcessing:
  saga_id: string
  payment_id: string
  status: string

PaymentCompleted:
  saga_id: string
  payment_id: string
  transaction_id: string
  completed_at: datetime

PaymentFailed:
  saga_id: string
  payment_id: string
  reason: string
  compensation_required: boolean

CompensationInitiated:
  saga_id: string
  payment_id: string
  reason: string
  compensation_steps: array
```

### 1.5 Eventos Suscritos

```yaml
FundsLocked: # Desde Wallet Service
FundsInsufficient: # Desde Wallet Service
PaymentSentToGateway: # Desde External Payment Service
PaymentGatewayResponse: # Desde External Payment Service (webhook)
PaymentGatewayTimeout: # Desde External Payment Service
FundsDebited: # Desde Wallet Service
FundsUnlocked: # Desde Wallet Service (compensación)
```

### 1.6 Dependencias

- **Event Store (Redpanda + PostgreSQL)**: Almacenamiento y streaming de eventos (fuente de verdad única). Redpanda es Kafka-compatible.
- **Estado de SAGAs**: Se reconstruye desde eventos almacenados en Event Store (Event Sourcing puro)
- **Wallet Service**: Vía eventos (no acceso directo)
- **External Payment Service**: Vía eventos (no acceso directo)

---

## 2. Wallet Service

### 2.1 Límites del Servicio

**Responsabilidades:**

- **Consumir Eventos**: Escucha eventos del Event Store (PaymentRequested, Commands)
- **Rebuild State**: Reconstruye estado desde eventos (Event Sourcing)
- **Procesar Lógica de Negocio**: Valida saldos, bloquea fondos, debita, acredita
- **Publicar Eventos**: Publica eventos resultado al Event Store

**No es responsable de:**

- Decisión de tipo de pago (SAGA Orchestrator maneja esto)
- Comunicación con pasarelas externas (External Payment Service)
- Orquestación de workflows (SAGA Orchestrator)

### 2.2 Interfaces

#### REST API (Interno - Para Debugging)

```http
GET    /internal/wallet/{user_id}              # Consultar billetera
GET    /internal/wallet/{user_id}/state        # Estado reconstruido desde eventos
POST   /internal/wallet/{user_id}/rebuild      # Reconstruir estado manualmente
```

### 2.3 Eventos Suscritos

```yaml
PaymentRequested: # Desde SAGA Orchestrator (solo si payment_type: "wallet")
CreditFunds: # Comando (top-up, refunds)
```

### 2.4 Eventos Publicados

```yaml
FundsDebited:
  payment_id: string
  user_id: string
  amount: decimal
  new_balance: decimal
  payment_type: "wallet"

FundsInsufficient:
  payment_id: string
  user_id: string
  requested_amount: decimal
  available_balance: decimal
  payment_type: "wallet"

WalletBalanceUpdated:
  user_id: string
  balance: decimal
  available_balance: decimal
```

### 2.5 Dependencias

- **Event Store (Redpanda + PostgreSQL)**: Consumo y publicación de eventos. Redpanda es Kafka-compatible.

---

## 3. External Payment Service

### 3.1 Límites del Servicio

**Responsabilidades:**

- **Consumir Eventos**: Escucha eventos de procesamiento de pago externo
- **Integración con Gateways**: Llama a pasarelas de pago externas
- **Publicar Respuestas**: Publica eventos de respuesta del gateway al Event Store
- **Manejo de Reintentos**: Implementa circuit breaker y retry logic
- **Webhook Processing**: Recibe webhooks de gateways y los publica como eventos

**No es responsable de:**

- Decisión de éxito/fallo (SAGA Orchestrator evalúa)
- Validación de saldos (gateway externo maneja sus propias validaciones)

### 3.2 Interfaces

#### REST API (Interno - Webhook Handler)

```http
POST   /internal/webhooks/{provider}     # Webhook handler genérico
```

### 3.3 Eventos Suscritos

```yaml
PaymentRequested: # Desde SAGA Orchestrator (solo si payment_type: "external")
PaymentRetryRequested: # Desde SAGA Orchestrator (reintentos)
```

### 3.4 Eventos Publicados

```yaml
PaymentSentToGateway:
  payment_id: string
  gateway_provider: string
  gateway_payment_id: string
  sent_at: datetime

PaymentGatewayResponse:
  payment_id: string
  gateway_provider: string
  status: string
  transaction_id: string
  response_data: object

PaymentGatewayTimeout:
  payment_id: string
  gateway_provider: string
  attempt: int
  timeout_duration_seconds: int

PaymentGatewayUnavailable:
  payment_id: string
  gateway_provider: string
  reason: string
```

### 3.5 Dependencias

- **Event Store (Redpanda + PostgreSQL)**: Consumo y publicación de eventos. Redpanda es Kafka-compatible.
- **External Gateways**: APIs de pasarelas de pago externas
- **Circuit Breaker**: Protección contra gateways caídos
- **Webhook Handler**: Recibe callbacks de gateways

---

## 4. Metrics Service

### 4.1 Límites del Servicio

**Responsabilidades:**

- **Consumir Eventos**: Escucha todos los eventos relevantes del Event Store
- **Agregación en Tiempo Real**: Calcula métricas (tasas de éxito, latencias, volúmenes)
- **Observabilidad**: Publica métricas y logs estructurados

**No es responsable de:**

- Lógica de negocio (solo observabilidad)
- Almacenamiento histórico largo plazo (Analytics Service opcional)

### 4.2 Interfaces

#### REST API

```http
GET    /api/v1/metrics/payments/success-rate     # Tasa de éxito
GET    /api/v1/metrics/payments/latency           # Latencia promedio
GET    /api/v1/metrics/payments/volume            # Volumen de pagos
GET    /api/v1/metrics/sagas/completion-rate      # Tasa de completación de SAGAs
GET    /api/v1/metrics/sagas/compensation-rate    # Tasa de compensaciones
```

### 4.3 Eventos Suscritos

```yaml
PaymentRequested: # Tasa de requests
PaymentCompleted: # Tasa de éxito
PaymentFailed: # Tasa de fallos
PaymentGatewayResponse: # Latencia de gateways
FundsLocked: # Volumen de fondos bloqueados
FundsDebited: # Volumen de fondos debitados
CompensationInitiated: # Tasa de compensaciones
SagaFinalized: # Tiempo de completación de SAGAs
```

### 4.4 Dependencias

- **Event Bus (Redpanda)**: Consumo de eventos en tiempo real. Redpanda es Kafka-compatible.
- **Observability System**: Publicación de métricas y logs estructurados

---

## 5. Patrones de Comunicación

### 5.1 Event-Driven Puro

**Todos los servicios:**

- **Consumen eventos** del Event Store
- **Publican eventos** al Event Store
- **No se comunican directamente** entre sí
- **Desacoplamiento total**

### 5.2 Commands vs Events

**Commands**: Instrucciones para ejecutar una acción

- Publicados por SAGA Orchestrator
- Ejemplo: `UnlockFunds`, `LockFunds`, `SendToGateway`

**Events**: Hechos que ya ocurrieron (inmutables)

- Publicados por servicios después de procesar
- Ejemplo: `FundsLocked`, `PaymentCompleted`, `FundsUnlocked`

### 5.3 Event Sourcing Pattern

**Flujo:**

```
1. Service recibe evento/command
2. Rebuild estado desde Event Store
3. Aplica lógica de negocio
5. Publica nuevo evento
```
