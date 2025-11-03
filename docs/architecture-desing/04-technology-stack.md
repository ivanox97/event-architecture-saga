# Recomendación de Stack Tecnológico - Arquitectura Event-Driven

## 1. Event Store - Dual Storage Strategy

### 1.1 Message Broker / Streaming Layer: Redpanda (Kafka-compatible)

**Implementación MVP:**

- **Redpanda**: Kafka-compatible, más ligero y eficiente para desarrollo local
- **100% Compatible**: Usa la misma API de Kafka, puede intercambiarse por Kafka en producción
- **Configuración**: Ejecuta en Docker Compose, puerto `19092` para Kafka API

**Justificación para Producción (Apache Kafka):**

1. **Event Streaming Nativo**: Diseñado específicamente para streams de eventos
2. **Alto Throughput**: Millones de eventos por segundo
3. **Orden Garantizado**: Particionamiento garantiza orden por partition key
4. **Consumer Groups**: Procesamiento paralelo con balanceo automático
5. **Retención Configurable**: Retención corta para procesamiento en tiempo real (7 días)
6. **Replication**: Alta disponibilidad con replicación nativa
7. **Ecosistema**: Kafka Connect, Schema Registry, KSQL

**Configuración MVP (Redpanda):**

- Broker: `localhost:19092`
- Topic: `events.payments.v1` con 12 particiones
- Configurable mediante variable de entorno `KAFKA_BROKERS`

**Configuración Producción (Kafka):**

```yaml
kafka:
  cluster:
    brokers: 6
    replication_factor: 3
    min_insync_replicas: 2
    partitions_per_topic: 12

  retention:
    time: 7 days
    size: 10GB per partition

  performance:
    acks: all
    enable_idempotence: true
    compression: snappy
    max_in_flight_requests: 5

  consumer:
    enable_auto_commit: false
    isolation_level: read_committed
    group_id: service-specific-groups
```

**Recursos:**

- Kafka Cluster: 6 nodes (3 brokers + replicas)
- Storage: SSD, 1TB por broker
- CPU: 16 cores por broker
- Memory: 64GB por broker (32GB heap for JVM)

**Alternativas consideradas:**

- **RabbitMQ**:

  - ✅ Más simple de configurar
  - ❌ Menor throughput
  - ❌ No diseñado para event sourcing
  - ❌ Menos adecuado para ordenamiento

---

### 1.2 Event Store Persistence: PostgreSQL

**Justificación para Event Sourcing:**

1. **ACID Compliance**: Garantiza que eventos se almacenen permanentemente
2. **JSONB Support**: Almacenamiento eficiente de datos de eventos
3. **Particionamiento Temporal**: Particiones mensuales para performance
4. **Query Capabilities**: Consultas complejas sobre eventos históricos
5. **Point-in-Time Recovery**: Capacidad de restaurar a cualquier momento

**Schema Event Store:**

```sql
CREATE DATABASE event_store_db;

CREATE TABLE events (
    event_id UUID PRIMARY KEY,
    aggregate_id VARCHAR(255) NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_version INT NOT NULL DEFAULT 1,
    event_data JSONB NOT NULL,
    event_metadata JSONB,
    timestamp TIMESTAMP NOT NULL,
    sequence_number BIGSERIAL NOT NULL,
    kafka_offset BIGINT,
    kafka_partition INT,
    kafka_topic VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    INDEX idx_aggregate (aggregate_id, sequence_number),
    INDEX idx_timestamp (timestamp),
    INDEX idx_kafka (kafka_topic, kafka_partition, kafka_offset)
) PARTITION BY RANGE (timestamp);
```

**Configuración:**

```yaml
postgresql_event_store:
  version: 15.x
  instances:
    primary: 1
    replicas: 3 (read replicas para queries)

  resources:
    cpu: 16 cores
    memory: 64GB
    storage: 2TB SSD (provisioned IOPS)

  backup:
    strategy: Continuous WAL archiving
    retention: 30 days
    point_in_time_recovery: enabled

  partitioning:
    strategy: monthly
    auto_partition_creation: enabled
```

**Alternativas consideradas:**

- **EventStore DB**:
  - ✅ Diseñado específicamente para event sourcing
  - ❌ Ecosistema más pequeño
  - ❌ Menos conocido que PostgreSQL
  - ✅ **Buena alternativa considerada**

---

### 2.2 Observabilidad

**Justificación:**

1. **Métricas en Tiempo Real**: Seguimiento de tasas de éxito, latencias, volúmenes
2. **Logs Estructurados**: Logs JSON con correlation IDs para trazabilidad
3. **Visualización**: Dashboards para monitoreo y análisis
4. **Alertas**: Alertas basadas en umbrales configurados

**Integración Metrics Service:**

```go
type MetricsService struct {
    observabilityClient ObservabilityClient
    logger             Logger
}

func (ms *MetricsService) PublishMetric(metricName string, value float64, tags map[string]string) {
    ms.observabilityClient.RecordMetric(metricName, value, tags)
}
```
