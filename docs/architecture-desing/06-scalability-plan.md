# Plan de Escalabilidad - Arquitectura Event-Driven

## Estado Actual (MVP)

### ¿Qué tenemos funcionando ahora?

Actualmente el sistema funciona con Event Sourcing puro, lo que significa que reconstruimos el estado completo desde los eventos cada vez que lo necesitamos. Esto es perfecto para empezar porque nos da total trazabilidad y consistencia, pero puede volverse lento cuando crece el volumen de datos.

Tenemos un único topic de Redpanda (Kafka-compatible) llamado `events.payments.v1` con 12 particiones que separan inteligentemente los eventos de wallet (pares) y external payments (impares). Esto nos permite mantener el orden por usuario y pago respectivamente.

Para manejar errores, implementamos un DLQ (Dead Letter Queue) simulado y una base de datos para registrar errores que requieren atención manual. También tenemos lógica de reintentos con backoff exponencial para cuando el gateway de pagos externo falla o se demora.

### ¿Qué nos falta para escalar?

Hay varias optimizaciones que podemos implementar cuando el tráfico lo justifique. La idea es no implementarlas prematuramente, sino tenerlas bien pensadas para cuando realmente las necesitemos.

Las principales optimizaciones pendientes son:

- **Wallet DB**: Una base de datos optimizada solo para consultas de balances (Read Model)
- **SAGA State Store**: Para consultas ultra-rápidas del estado de pagos
- **Snapshots**: Para acelerar la reconstrucción de estado cuando hay muchos eventos
- **Read Replicas**: Para distribuir la carga de lecturas en PostgreSQL
- **Separación de Topics**: Para escalar wallet y external payments independientemente
- **Sharding**: Para cuando tengamos millones de usuarios

---

## 1. Estrategias de Escalamiento Horizontal

### 1.1 Separación de Topics para Escalado Independiente

**¿Qué problema resuelve?**

Actualmente tenemos un solo topic `events.payments.v1` que maneja tanto pagos de wallet como pagos externos. Esto está bien al inicio, pero llegará un momento en que queramos escalar estos dos flujos de forma independiente. Por ejemplo, puede que los pagos externos requieran más retención de eventos o más particiones, mientras que los pagos de wallet pueden necesitar menos recursos.

**¿Cómo lo hacemos?**

La idea es migrar gradualmente a dos topics separados:

- `events.payments.wallet.v1` - Para todo lo relacionado con pagos desde wallet
- `events.payments.external.v1` - Para pagos con tarjeta de crédito externa

La migración debe ser gradual para no causar downtime:

1. **Primero hacemos dual-write**: Durante un par de semanas, publicamos en ambos topics (el antiguo y los nuevos). Esto nos da tiempo de validar que todo funciona correctamente.

2. **Luego migramos consumers gradualmente**: Movemos los servicios uno por uno a consumir de los nuevos topics. Empezamos con los menos críticos para ganar confianza.

3. **Finalmente deprecamos el topic antiguo**: Una vez que estemos 100% migrados y hayamos monitoreado por una semana sin problemas, podemos dejar de escribir en el topic original.

**¿Por qué es beneficioso?**

Una vez separados, cada tipo de pago puede tener su propia configuración. Los pagos externos pueden tener más particiones si generan más eventos, o diferentes políticas de retención. Podemos escalar cada flujo según sus necesidades específicas sin afectar al otro.

Además, cuando algo falle, será más fácil debuggear porque los eventos están separados por naturaleza del flujo. El monitoreo también se vuelve más claro: puedes ver exactamente cuántos eventos de wallet vs external se están procesando.

**Consideraciones importantes:**

Debemos mantener la misma estrategia de particionamiento en los nuevos topics. Los eventos de wallet siguen particionándose por `user_id` y los de external por `payment_id`. Esto garantiza que el orden se mantenga y que no tengamos condiciones de carrera.

**Convención de nombres:**

```
<domain>.<entity>.<action>.<version>
```

**Ejemplos de topics específicos:**

- **SAGA Domain**: `saga.events.payment.requested.v1`, `saga.events.payment.completed.v1`
- **Wallet Domain**: `wallet.events.funds.debited.v1`, `wallet.events.funds.insufficient.v1`
- **Payment Domain**: `payment.events.sent_to_gateway.v1`, `payment.events.gateway.response.v1`
- **Metrics Domain**: `metrics.events.payment.completed.v1`, `metrics.events.payment.failed.v1`

**¿Cuándo considerar esta estrategia?**

Esta separación granular tiene sentido cuando:

- Necesitas configuraciones muy específicas por tipo de evento (retención, particiones, replication factor)
- Diferentes equipos manejan diferentes dominios y quieren independencia total
- El volumen de eventos es tan alto que necesitas escalar cada tipo de evento independientemente
- Requieres políticas de seguridad o compliance diferentes por tipo de evento

**Trade-offs:**

La ventaja es máximo control y escalabilidad independiente. Sin embargo, aumenta la complejidad operativa: más topics para monitorear, más consumer groups, más configuración. Para la mayoría de sistemas, la separación en dos topics (`events.payments.wallet.v1` y `events.payments.external.v1`) es suficiente.

---

### 1.2 Horizontal Scaling de Bases de Datos (Read Replicas)

**¿Qué problema resuelve?**

En Event Sourcing, la base de datos principal (Event Store) se satura rápidamente si todo el tráfico de lecturas y escrituras va al mismo servidor. Las reconstrucciones de estado, las queries de auditoría y los analytics compiten con las escrituras de nuevos eventos.

**¿Cómo funciona?**

La solución es simple: tener un servidor principal que solo maneja escrituras y varios servidores réplica (read replicas) que manejan todas las lecturas. PostgreSQL replica automáticamente todos los cambios del primary a las replicas, así que siempre tienen los datos actualizados (con un pequeño delay típicamente menor a 100ms).

**Para el Event Store:**

Configuramos 1 primary que solo recibe escrituras y 3-6 read replicas que manejan todas las lecturas. El routing se hace automáticamente: cuando queremos escribir, usamos el primary. Cuando queremos leer (reconstrucciones, queries, analytics), distribuimos la carga entre las replicas usando round-robin.

**¿Por qué es beneficioso?**

Primero, alivia la presión del servidor principal. Las escrituras son críticas y no deben verse afectadas por consultas pesadas. Segundo, escalamos las lecturas horizontalmente: si necesitamos más capacidad de lectura, simplemente agregamos más replicas.

Esto es especialmente útil para reconstrucciones de estado, que pueden ser muy costosas. Podemos ejecutar múltiples reconstrucciones en paralelo usando diferentes replicas sin afectar las escrituras.

**Para Read Models (cuando los implementemos):**

Una vez que tengamos Wallet DB y SAGA Projections, también aplicamos read replicas allí. Estas bases de datos van a recibir muchísimas consultas (cada vez que alguien consulte su balance o el estado de un pago), así que necesitamos escalarlas horizontalmente.

La estrategia es la misma: 1 primary para escrituras (actualización del read model cuando llegan eventos) y múltiples replicas para las consultas. Esto nos permite soportar miles de queries por segundo sin problema.

**Ventajas adicionales:**

Si el primary se cae, podemos promover una replica a primary sin pérdida de datos. Las replicas también nos dan alta disponibilidad geográfica si las colocamos en diferentes zonas.

---

## 2. Enfoques de Balanceo de Carga

### 2.1 Load Balancing a Nivel de API Gateway

**¿Cuándo lo necesitamos?** Tan pronto como tengamos múltiples instancias de nuestros servicios corriendo.

**¿Qué problema resuelve?**

Cuando tenemos varias instancias del mismo servicio (por ejemplo, 5 instancias del SAGA Orchestrator), necesitamos distribuir las peticiones HTTP entrantes de forma inteligente. No queremos que una instancia esté sobrecargada mientras otra está ociosa.

**¿Cómo funciona?**

Kong, nuestro API Gateway, se encarga de esto usando el algoritmo "least connections". Esto significa que cuando llega una petición, Kong la dirige a la instancia que tiene menos conexiones activas. Es más inteligente que round-robin porque considera la carga real actual.

**Health checks automáticos:**

Lo más importante aquí son los health checks. Kong monitorea constantemente la salud de cada instancia. Si una instancia empieza a fallar (3 fallos HTTP consecutivos), Kong automáticamente deja de enviarle tráfico hasta que se recupere.

Esto previene que los usuarios reciban errores por una instancia que está caída. El sistema se auto-repara sin intervención manual.

**¿Por qué es beneficioso?**

Primero, garantiza que ninguna instancia se sobrecargue. Segundo, nos da alta disponibilidad: si una instancia falla, el tráfico se redistribuye automáticamente a las instancias sanas. Tercero, simplifica el despliegue: podemos agregar o quitar instancias sin afectar el servicio.

**Configuración práctica:**

Para cada servicio configuramos health checks que revisan cada 10 segundos. Si una instancia responde correctamente 3 veces seguidas, se considera saludable. Si falla 3 veces seguidas, se marca como no saludable y se excluye del pool.

---

## 3. Particionamiento de Bases de Datos

### 3.1 Particionamiento Temporal del Event Store

**¿Qué problema resuelve?**

Con el tiempo, la tabla de eventos crece enormemente. Aunque los índices ayudan, cuando tienes cientos de millones de filas, las queries que buscan eventos por rango de fechas se vuelven muy lentas porque tienen que escanear demasiados datos.

**¿Cómo funciona?**

En lugar de tener una tabla gigante, dividimos los eventos en particiones mensuales. PostgreSQL maneja esto automáticamente: cuando haces una query por fecha, solo busca en la partición relevante, no en toda la tabla.

Por ejemplo, si quieres eventos de enero 2024, PostgreSQL solo busca en la partición `events_2024_01`, que tiene mucho menos datos que la tabla completa.

**Mantenimiento simplificado:**

Esto también hace el mantenimiento mucho más fácil. Cuando una partición tiene más de un año, podemos moverla a almacenamiento frío (como S3) para reducir costos. O simplemente eliminarla si ya no la necesitamos, sin afectar a las otras particiones.

**Auto-creación de particiones:**

Configuramos un job que automáticamente crea la partición del mes siguiente antes de que la necesitemos. Así nunca nos quedamos sin partición cuando empezamos un nuevo mes.

**¿Por qué es beneficioso?**

Las queries son muchísimo más rápidas porque PostgreSQL solo necesita buscar en una pequeña partición en lugar de toda la tabla. Los índices también funcionan mejor porque son más pequeños y eficientes por partición.

Además, el costo de almacenamiento se puede optimizar: mantenemos activas solo las últimas 12-24 meses y archivamos el resto.

---

### 3.2 Sharding de Wallet DB (CQRS Read Model)

**¿Por qué necesitamos Wallet DB?**

Actualmente, cuando alguien quiere consultar su balance, reconstruimos todo el estado desde los eventos. Esto funciona bien cuando un usuario tiene pocos eventos, pero imagina un usuario que ha hecho miles de transacciones: reconstruir todo eso toma tiempo.

Wallet DB es una optimización que mantiene el balance actualizado en una tabla especializada. Cuando procesamos un evento que afecta el balance (como `FundsDebited` o `FundsCredited`), actualizamos esta tabla. Cuando alguien consulta su balance, simplemente hacemos un SELECT directo - instantáneo.

**Ventajas principales:**

La diferencia de performance es abismal. De reconstruir miles de eventos (que puede tomar segundos) a un simple SELECT que toma milisegundos. Además, permite hacer queries complejas como "dame todos los usuarios con balance mayor a X" que serían imposibles o muy lentas con Event Sourcing puro.

**Sharding para millones de usuarios:**

Cuando Wallet DB crece demasiado (más de 1 millón de usuarios o más de 100GB), dividimos la tabla en múltiples "shards" o particiones horizontales. Cada shard contiene una porción de los usuarios.

El routing es simple: hacemos un hash del `user_id` y ese hash nos dice en qué shard está ese usuario. Por ejemplo, si tenemos 10 shards, el hash del user_id módulo 10 nos da el número de shard (0-9).

**Escalabilidad horizontal:**

Lo mejor del sharding es que podemos agregar más shards cuando necesitemos. Cada shard puede tener sus propias read replicas, así que escalamos tanto en capacidad total como en capacidad de lectura.

**Cache adicional:**

Como optimización extra, podemos cachear balances frecuentemente consultados en Redis. Esto nos da respuestas en menos de un milisegundo para los usuarios más activos, mientras reducimos la carga en Wallet DB.

---

### 4.2 Cache de Read Models (Wallet DB y Projections)

**¿Por qué cachear?**

Incluso con Wallet DB, que es rápido, hay usuarios que consultan su balance constantemente. O pagos que se consultan repetidamente. Cachear estos datos en Redis nos da respuestas casi instantáneas y reduce la carga en nuestras bases de datos.

**Estrategia de cache:**

Cacheamos balances de usuarios con un TTL de 5 minutos. Si un usuario consulta su balance y está en cache, respondemos instantáneamente. Si no está, consultamos Wallet DB y cacheamos el resultado.

Para estados de pago, cacheamos con un TTL más corto (1 minuto) porque cambian más frecuentemente.

**¿Por qué es beneficioso?**

Reduce dramáticamente la latencia para usuarios frecuentes. También protege nuestras bases de datos durante picos de tráfico: aunque lleguen miles de consultas simultáneas, muchas serán servidas desde cache sin tocar la base de datos.

---

## 5. Análisis de Cuellos de Botella de Rendimiento

### 5.1 Event Processing Bottleneck

**¿Cómo identificamos este problema?**

El síntoma más claro es el consumer lag.

**¿Por qué ocurre?**

Puede ser porque estamos procesando eventos uno por uno de forma secuencial, o porque cada evento hace múltiples operaciones costosas (consultas a base de datos, llamadas HTTP, etc.).

**Soluciones:**

**Batch Processing:** En lugar de procesar eventos uno por uno, los agrupamos en batches de 100 eventos y los procesamos juntos. Esto reduce el overhead de red y de base de datos. En lugar de hacer 100 inserts individuales, hacemos uno con 100 eventos.

**Procesamiento Paralelo:** Si tenemos múltiples workers procesando eventos en paralelo, podemos manejar más carga. Eso sí, debemos mantener el orden dentro de cada agregado (todos los eventos de un mismo usuario deben procesarse en orden).

---

### 5.2 Query Performance Bottleneck

**¿Por qué ocurre?**

Con Event Sourcing puro, cada consulta requiere reconstruir el estado desde todos los eventos. Si un usuario tiene 10,000 eventos, tenemos que cargarlos todos y aplicarlos secuencialmente. Esto es correcto pero lento.

**Soluciones:**

**CQRS (Wallet DB):**

En lugar de reconstruir desde eventos cada vez, mantenemos una tabla `wallets` que se actualiza cada vez que procesamos un evento relacionado con balances. Cuando alguien consulta su balance, simplemente hacemos `SELECT balance FROM wallets WHERE user_id = ?`. Esto es instantáneo comparado con reconstruir miles de eventos.

La magia está en que este read model se actualiza asíncronamente: cuando llega un evento que cambia el balance, un proceso de proyección lo lee y actualiza la tabla. Hay un pequeño delay (típicamente menos de 100ms), pero para consultas de balance esto es perfectamente aceptable.

**CQRS Projections para SAGAs:**

Lo mismo aplica para consultas de estado de pago. Mantenemos una tabla `saga_projections` con el estado actual de cada pago. `GetPaymentStatus` se convierte en un simple SELECT en lugar de reconstruir toda la saga.

---

### 5.3 Circuit Breaker

Un circuit breaker es como un fusible eléctrico. Cuando detecta que el gateway está fallando (por ejemplo, 5 fallos consecutivos), "abre el circuito" y deja de intentar llamadas durante un período (digamos 60 segundos).

Durante ese período, cualquier intento de llamar al gateway falla inmediatamente sin hacer la llamada HTTP. Esto previene que nuestros servicios se bloqueen esperando timeouts.

**Beneficios:**

- **Falla rapido:** No esperamos timeouts de 30 segundos cuando el gateway está caído
- **Previene fallas en cascada** El problema no se propaga a otros servicios
- **Auto-recovery:** Cuando el gateway vuelve, el sistema se recupera automáticamente
