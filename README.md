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
- **Containerización**: Podman + podman-compose

## Cómo Ejecutar

### Requisitos Previos

#### 1. Instalar Podman

**macOS:**

```bash
brew install podman podman-compose
```

#### 2. Iniciar Podman Machine (solo necesario la primera vez)

```bash
# Iniciar Podman machine
podman machine init
podman machine start
```

**⚠️ Problema de Permisos?** Si tienes errores relacionados con permisos en `~/.config`, ejecuta:

```bash
./fix-podman-permissions.sh
```

Esto solo es necesario una vez. La máquina de Podman se iniciará automáticamente cuando sea necesario.

#### 3. Instalar Go (si no lo tienes)

**macOS:**

```bash
brew install go
```

### Setup Inicial

```bash
# 1. Clonar o navegar al proyecto
cd app

# 2. Instalar dependencias Go
go mod download

# 3. Iniciar todo el sistema
make start
```

Este comando automáticamente:

- ✅ Verifica que Podman esté instalado
- ✅ Inicia la máquina de Podman (si no está corriendo)
- ✅ Levanta PostgreSQL y Redpanda con Podman
- ✅ Ejecuta las migraciones de base de datos
- ✅ Inicia los 4 servicios de la aplicación (Orchestrator, Wallet, External Payment, Metrics)

### Verificar que Todo Funciona

```bash
# Health checks de todos los servicios
make health

# Ver logs
tail -f logs/*.log
```

### Comandos Útiles

```bash
# Ver todos los comandos disponibles
make help

# Iniciar infraestructura (PostgreSQL + Redpanda)
make up

# Detener infraestructura
make down

# Iniciar/Detener máquina de Podman
make podman-start
make podman-stop

# Verificar estado de Podman
make podman-status
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

Todos los comandos usan el mismo `user_id` por defecto (`123e4567-e89b-12d3-a456-426614174000`) para facilitar las pruebas. Puedes cambiarlo usando `USER_ID=<tu_user_id>` en cualquier comando.

#### Flujo Completo de Prueba

```bash
# 1. Agregar fondos a la billetera
make add-funds USER_ID=123e4567-e89b-12d3-a456-426614174000 AMOUNT=5000.0

# 2. Verificar balance
make test-balance

# 3. Crear un pago con billetera
make test-payment AMOUNT=1500.0

# 4. Verificar el estado del pago (usa el payment_id de la respuesta anterior)
make test-payment-status PAYMENT_ID=<payment_id_de_la_respuesta>

# 5. Verificar balance actualizado
make test-balance

# 6. (Opcional) Procesar un reembolso
make test-refund PAYMENT_ID=<payment_id> AMOUNT=500.0
```

#### Comandos Individuales

##### 1. Agregar Fondos a una Billetera

```bash
# Agregar fondos (requerido antes de hacer pagos)
make add-funds USER_ID=123e4567-e89b-12d3-a456-426614174000 AMOUNT=5000.0

# O cambiar el user_id y amount
make add-funds USER_ID=<tu_user_id> AMOUNT=<cantidad>
```

##### 2. Consultar Balance de Billetera

```bash
# Usa el user_id por defecto
make test-balance

# O especifica un user_id diferente
make test-balance USER_ID=123e4567-e89b-12d3-a456-426614174000
```

##### 3. Crear un Pago con Billetera

```bash
# Crear pago (usa user_id y amount por defecto)
make test-payment

# O especifica amount y user_id
make test-payment USER_ID=123e4567-e89b-12d3-a456-426614174000 AMOUNT=1500.0
```

##### 4. Consultar Estado del Pago

```bash
# Reemplaza <payment_id> con el ID de la respuesta anterior
make test-payment-status PAYMENT_ID=be203f47-5826-45a0-8cbd-f9d8ae59a654
```

##### 5. Crear un Pago con Tarjeta de Crédito

```bash
# Crear pago con tarjeta (usa user_id, amount y card_token por defecto)
make test-payment-card

# O especifica amount, user_id y card_token
make test-payment-card USER_ID=123e4567-e89b-12d3-a456-426614174000 AMOUNT=200.0 CARD_TOKEN=my-card-token-123
```

##### 6. Procesar un Reembolso

```bash
# Procesar reembolso (necesita payment_id y amount)
make test-refund PAYMENT_ID=<payment_id> AMOUNT=50.0

# También puedes especificar user_id si es diferente al predeterminado
make test-refund PAYMENT_ID=<payment_id> AMOUNT=50.0 USER_ID=<user_id>
```

#### Notas

- Todos los comandos usan `USER_ID=123e4567-e89b-12d3-a456-426614174000` por defecto para facilitar las pruebas
- El `AMOUNT` por defecto es `100.0` en los comandos de pago
- Guarda el `payment_id` de las respuestas para consultar el estado o hacer reembolsos
- Los comandos muestran la respuesta formateada en JSON cuando es posible

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

| Método | Endpoint                     | Descripción                |
| ------ | ---------------------------- | -------------------------- |
| GET    | `/internal/wallet/:user_id`  | Consultar balance          |
| POST   | `/internal/wallet/add-funds` | Agregar fondos a billetera |
| POST   | `/internal/wallet/refund`    | Procesar reembolso         |
| GET    | `/health`                    | Health check               |

### External Payment Service (Puerto 8082)

- `GET /health` - Health check

### Metrics Service (Puerto 8083)

- `GET /health` - Health check

## Comandos Útiles

### Comandos de Infraestructura

```bash
make help          # Ver todos los comandos disponibles
make start         # Iniciar todo (infraestructura + servicios)
make stop          # Detener todos los servicios
make up            # Iniciar solo PostgreSQL y Redpanda
make down          # Detener PostgreSQL y Redpanda
make health        # Verificar estado de servicios
make podman-start  # Iniciar máquina de Podman
make podman-stop   # Detener máquina de Podman
```

### Comandos de Prueba de API

```bash
# Agregar fondos a una billetera
make add-funds USER_ID=<user_id> AMOUNT=<cantidad>

# Consultar balance
make test-balance [USER_ID=<user_id>]

# Crear pago con billetera
make test-payment [USER_ID=<user_id>] [AMOUNT=<cantidad>]

# Consultar estado de pago
make test-payment-status PAYMENT_ID=<payment_id>

# Crear pago con tarjeta
make test-payment-card [USER_ID=<user_id>] [AMOUNT=<cantidad>] [CARD_TOKEN=<card_token>]

# Procesar reembolso
make test-refund PAYMENT_ID=<payment_id> AMOUNT=<cantidad> [USER_ID=<user_id>]
```

**Nota**: Todos los comandos de prueba usan `USER_ID=123e4567-e89b-12d3-a456-426614174000` por defecto. Ver [Probar la API](#probar-la-api) para más detalles.

### Otros Comandos

```bash
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
