# Design a distributed KV store

Design a **Dynamo / etcd-class** key-value store: clients `PUT` and `GET` keys, you handle replication, partitioning, and failures.

## Functional requirements

1. **Put(key, value)** — store bytes; optional TTL.
2. **Get(key)** — return latest value or not found.
3. **Delete(key)**.
4. **Scan(prefix)** — range read (optional v1 — mention trade-off).
5. **List nodes** — cluster membership for ops.

## Scale

| Metric | Value |
|--------|-------|
| Cluster size | 50–500 nodes |
| Keys | 10B |
| Avg value size | 4 KB |
| Read QPS | 1M aggregate |
| Write QPS | 200k aggregate |

## Consistency (pick and defend)

Offer at least two modes in your design:

- **Strong** — linearizable reads/writes for a key.
- **Eventual** — cheaper, faster, stale reads OK.

## Non-functional

- Survive **f node** failures (typical f=1 per replica set).
- Add/remove nodes without full downtime.
- Rebalance when load skews.

## Your task

Whiteboard **45–55 minutes**:

1. Partitioning: how `key` → `partition` → `replica set`.
2. Write path with replication (quorum? leader?).
3. Read path — strong vs eventual.
4. Membership changes: new node joins, old node leaves.
5. On-disk structure per node — why LSM?

## Stretch

- Compare to single-node RocksDB + proxy layer.
- Cross-datacenter replication lag.
