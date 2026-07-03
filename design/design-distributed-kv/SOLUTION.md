# Reference architecture — distributed KV

## Cluster layout

```
Client SDK (smart routing)
        │
        ▼
┌───────────────────────────────────┐
│  Coordinator / proxy (optional)   │
└───────────────┬───────────────────┘
                │
    ┌───────────┼───────────┐
    ▼           ▼           ▼
 Partition 0  Partition 1  Partition 2
 (Raft group) (Raft group) (Raft group)
  3 nodes      3 nodes      3 nodes
  + LSM        + LSM        + LSM
```

## Write path (Raft model)

1. Client routes to partition leader via ring metadata.
2. Leader appends to Raft log: `Put(k,v)`.
3. Replicate to followers; commit on majority.
4. Apply to local LSM engine.
5. ACK client.

## Read path

| Mode | Path |
|------|------|
| Strong | Leader only (or `ReadIndex` quorum) |
| Eventual | Any replica; may be stale |

## Metadata service

`partition_map: partition_id → {leader, replicas, key_range}`

Stored in a small highly-available meta-cluster (or gossip between nodes).

## Node add/remove

1. Update ring → new partition assignments.
2. Stream SSTables / log catch-up for moved ranges.
3. Flip routing when caught up.
4. Decommission old copies.

**Hash ring** minimizes data moved: only adjacent partitions affected.

## Hot keys

Detect skew → split partition (sub-range) or dedicated leader.

## Comparison table

| System | Model |
|--------|-------|
| etcd | Raft + bbolt per node |
| Dynamo | Quorum, vector clocks |
| TiKV | Raft shard + RocksDB |

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Consensus per shard | `build-your-own-raft` |
| Key → node routing | `build-your-own-hash-ring` |
| Local storage engine | `build-your-own-lsm` |

You've built all three layers separately — this problem is wiring them into one product.
