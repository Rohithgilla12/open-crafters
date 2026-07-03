# Design an event streaming platform

Design a **Kafka / Pulsar-class** event streaming system: producers append ordered records to topics; consumers read by partition offset with configurable retention and delivery semantics.

## Functional requirements

1. **Produce** — append `(key, value, headers)` to a named topic; return offset + timestamp.
2. **Consume** — read from a partition starting at offset, or subscribe to a consumer group for load-balanced delivery.
3. **Topics & partitions** — topics split into N partitions for parallelism; ordering guaranteed **within** a partition only.
4. **Retention** — time-based (e.g. 7 days) and/or size-based; old segments deleted automatically.
5. **Consumer groups** — track committed offsets per `(group, topic, partition)`; rebalance when members join/leave.
6. **Replay** — consumers can reset offset and re-read history within retention window.

## Scale

| Metric | Value |
|--------|-------|
| Topics | 50k |
| Partitions (cluster-wide) | 500k |
| Peak produce rate | 5M records/sec (~50 GB/sec ingress) |
| Peak consume rate | 8M records/sec (fan-out heavy) |
| Avg record size | 1 KB |
| Retention | 7 days default (~30 PB logical; tiered storage OK) |
| Consumer groups | 200k active |

Writes and reads are both hot — **fan-out** multiplies read bandwidth.

## Non-functional

- Produce p99 **&lt; 10ms** (ack after durable commit on leader).
- Survive broker failure without losing committed records (replication factor 3).
- At-least-once delivery by default; exactly-once as a stretch (idempotent producer + transactional consume).
- 99.99% availability on produce/consume paths per AZ.

## Your task

Whiteboard **45–55 minutes**:

1. Topic → partition → broker assignment (who owns which partition leader?).
2. Produce path: client → leader broker → replicate → ack.
3. On-disk layout per partition — segments, indexes, offset monotonicity.
4. Consumer group coordination: partition assignment, offset commits, rebalance protocol.
5. Retention and compaction policies — when to delete vs compact by key.
6. How a failed broker recovers without corrupting the log.

## Stretch

- Exactly-once semantics across produce + consume.
- Cross-datacenter mirroring (active-active vs active-passive).
- Tiered storage: hot SSD for recent data, cold object store for old segments.
- Schema registry and backward-compatible evolution.
