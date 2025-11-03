# Event-Driven Payment System

Sistema de pagos orientado a eventos que implementa Event Sourcing y el patrón SAGA Orchestration, diseñado como demostración de arquitectura para un desafío técnico.

## Propósito

Este sistema permite procesar pagos de dos formas:

- **Pagos con Billetera**: Usa el saldo almacenado del usuario
- **Pagos con Tarjeta**: Procesa pagos a través de un gateway externo

**Características principales:**

- ✅ Event Sourcing puro para reconstrucción completa del estado
- ✅ SAGA Orchestration para coordinar transacciones distribuidas
- ✅ Arquitectura Hexagonal (Ports & Adapters)
- ✅ Event Bus con particionamiento inteligente (Redpanda/Kafka-compatible)
- ✅ Retry logic con exponential backoff para pagos externos
- ✅ Dead Letter Queue (DLQ) para manejo de errores

## Arquitectura

### Diagrama de Capas

```
┌────────────────────────────────────────────────────────┐
│                  Infrastructure Layer                  │
│         (Adaptadores: HTTP, Event Bus, Event Store)    │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐   |
│  │   HTTP      │  │  Event Bus   │  │ Event Store  │   │
│  │   (Gin)     │  │              │  │ (PostgreSQL) │   │
│  └──────┬──────┘  └──────-┬──────┘  └─────-─-┬─────┘   │
└─────────┼─────────────────┼──────────────────┼─────────┘
          │                 │                  │
          │    ┌────────────▼──────────────┐   │
          └───►│   Application Layer       │◄──┘
               │   (Casos de Uso)          │
               │  ┌─────────────────────┐  │
               │  │ SAGA Orchestrator   │  │
               │  │ Wallet Service      │  │
               │  │ External Payment    │  │
               │  │ Metrics Service     │  │
               │  └──────────┬──────────┘  │
               └─────────────┼─────────────┘
                             │
          ┌──────────────────┼──────────────────┐
          │                  │                  │
┌─────────▼──────────────────▼──────────────────▼─────────┐
│                  Domain Layer                           │
│        (Entidades, Eventos, Reglas de Negocio)          │
│  ┌─────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │    Saga     │  │    Wallet    │  │    Events    │    │
│  │  (Entity)   │  │  (Aggregate) │  │  (Domain)    │    │
│  └─────────────┘  └──────────────┘  └──────────────┘    │
│                                                         │
│                                                         │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Componentes Principales

#### 1. **Domain Layer** (`internal/domain/`)

Lógica pura del negocio sin dependencias externas:

- `Saga`: Entidad que representa el estado de una transacción distribuida
- `Wallet`: Agregado que representa la billetera del usuario
- `Events`: Eventos de dominio (WalletPaymentRequested, FundsDebited, etc.)

#### 2. **Application Layer** (`internal/application/`)

Coordina el dominio con el resto del sistema:

- **SAGA Orchestrator**: Orquesta el flujo completo de pagos mediante eventos
- **Wallet Service**: Maneja operaciones de wallet (validación de saldo, deducciones)
- **External Payment Service**: Procesa pagos externos con retry logic
- **Metrics Service**: Recolecta métricas y procesa eventos de DLQ

#### 3. **Infrastructure Layer** (`internal/infrastructure/`)

Adaptadores que conectan con el mundo exterior:

- **HTTP Adapters**: Handlers HTTP (Gin Framework)
- **Event Bus**: Implementación con Redpanda (Kafka-compatible)
- **Event Store**: Persistencia en PostgreSQL
- **DLQ**: Dead Letter Queue (mock)

### Flujo de Eventos - Pago con Billetera

```
1. Cliente → POST /api/payments/wallet
   ↓
2. SAGA Orchestrator publica WalletPaymentRequested
   ↓
3. Wallet Service consume evento:
   - Reconstruye estado de Wallet desde eventos
   - Valida saldo disponible
   - Publica FundsDebited (éxito) o FundsInsufficient (fallo)
   ↓
4. SAGA Orchestrator consume FundsDebited:
   - Actualiza estado del SAGA
   - Publica WalletPaymentCompleted
   ↓
5. Metrics Service registra métricas
```

### Tecnologías

- **Lenguaje**: Go 1.21+
- **Event Store**: PostgreSQL (con JSONB para eventos)
- **Event Bus**: Redpanda (Kafka-compatible)
- **HTTP Framework**: Gin
- **Containerización**: Colima + Docker Compose

## Cómo Ejecutar

### Setup Inicial

```bash
# 1. Instalar dependencias
brew install colima postgresql  # macOS
# O usar PostgreSQL local si prefieres

# 2. Configurar variables de entorno (opcional)
# Copia .env.example a .env y ajusta los valores
cp .env.example .env

# 3. Iniciar infraestructura y servicios
make start
```

Este comando:

- Inicia Colima (si no está corriendo)
- Levanta PostgreSQL y Redpanda con Docker
- Ejecuta migraciones de base de datos
- Inicia los 4 servicios de la aplicación

**Nota**: Si Docker falla, el sistema intentará usar PostgreSQL local automáticamente.

### Verificar que Todo Funciona

```bash
# Health checks de todos los servicios
make health

# Ver logs
tail -f logs/*.log
```

## Cómo Testear

### Tests Unitarios

Los tests están ubicados junto a cada archivo (`*_test.go`):

```bash
# Ejecutar todos los tests
make test

# Tests con coverage
go test ./... -cover

# Tests verbose
go test ./... -v

# Tests de un paquete específico
go test ./internal/domain/saga/...
```

### Probar la API

#### 1. Crear un Pago con Billetera

```bash
curl -X POST http://localhost:8080/api/payments/wallet \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "123e4567-e89b-12d3-a456-426614174000",
    "service_id": "service-123",
    "amount": 100.0,
    "currency": "USD"
  }'
```

#### 2. Consultar Estado del Pago

```bash
# Usar el payment_id de la respuesta anterior
curl http://localhost:8080/api/v1/payments/{payment_id}
```

#### 3. Consultar Balance de Billetera

```bash
curl http://localhost:8081/internal/wallet/{user_id}
```

#### 4. Procesar un Reembolso

```bash
curl -X POST http://localhost:8081/internal/wallet/refund \
  -H "Content-Type: application/json" \
  -d '{
    "payment_id": "{payment_id}",
    "user_id": "{user_id}",
    "amount": 50.0,
    "reason": "Service cancellation"
  }'
```

## Estructura del Proyecto

```
app/
├── cmd/                    # Puntos de entrada de cada servicio
│   ├── orchestrator/
│   ├── wallet/
│   ├── external-payment/
│   └── metrics/
├── internal/
│   ├── domain/            # Lógica de negocio pura
│   │   ├── saga/
│   │   ├── wallet/
│   │   └── events/
│   ├── application/       # Casos de uso
│   │   ├── saga/
│   │   ├── wallet/
│   │   ├── externalpayment/
│   │   └── metrics/
│   ├── infrastructure/    # Adaptadores externos
│   │   ├── http/
│   │   ├── eventbus/
│   │   ├── eventstore/
│   │   └── dlq/
│   └── common/            # Utilidades compartidas
│       ├── configs/
│       ├── logger/
│       └── metrics/
├── migrations/            # Migraciones SQL
├── docker-compose.yml     # Configuración de infraestructura
└── Makefile              # Comandos de automatización
```

## API Endpoints

### SAGA Orchestrator (Puerto 8080)

| Método | Endpoint                   | Descripción              |
| ------ | -------------------------- | ------------------------ |
| POST   | `/api/payments/wallet`     | Crear pago con billetera |
| POST   | `/api/payments/creditcard` | Crear pago con tarjeta   |
| GET    | `/api/v1/payments/:id`     | Consultar estado de pago |
| GET    | `/health`                  | Health check             |

### Wallet Service (Puerto 8081)

| Método | Endpoint                    | Descripción        |
| ------ | --------------------------- | ------------------ |
| GET    | `/internal/wallet/:user_id` | Consultar balance  |
| POST   | `/internal/wallet/refund`   | Procesar reembolso |
| GET    | `/health`                   | Health check       |

### External Payment Service (Puerto 8082)

- `GET /health` - Health check

### Metrics Service (Puerto 8083)

- `GET /health` - Health check

## Comandos Útiles

```bash
make help          # Ver todos los comandos disponibles
make start         # Iniciar todo (infraestructura + servicios)
make stop          # Detener todos los servicios
make health        # Verificar estado de servicios
make test          # Ejecutar tests
make clean         # Limpiar artefactos
```

## Decisiones de Arquitectura

### ¿Por qué Arquitectura Hexagonal?

- **Separación de responsabilidades**: El dominio no conoce detalles de implementación
- **Testabilidad**: Cada capa se testea independientemente
- **Flexibilidad**: Cambiar adaptadores sin afectar el núcleo de negocio

### ¿Por qué Event Sourcing?

- **Fuente única de verdad**: Todos los eventos se almacenan
- **Auditoría completa**: Historial completo de cambios
- **Time travel**: Reconstruir estado en cualquier punto del tiempo
- **Debugging**: Rastrear qué eventos llevaron a un estado específico

### ¿Por qué SAGA Orchestration?

- **Transacciones distribuidas**: Coordina operaciones entre múltiples servicios
- **Compensación**: Maneja rollback mediante eventos compensatorios
- **Resiliencia**: Recupera de fallos parciales

### ¿Por qué Redpanda?

- **Kafka-compatible**: 100% compatible con Kafka API
- **Ligero y eficiente**: Más ligero que Kafka
- **Production-ready**: Implementación real, no simulada
- **Particionamiento**: Orden garantizado dentro de cada partición
