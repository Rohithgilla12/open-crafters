# Reference architecture — event streaming

Spoiler: compare after your own attempt.

## Components

```
Producers ──▶ Broker cluster (partition leaders + followers)
                    │
                    ├── segment files + offset index (per partition)
                    │
Consumers ◀── Consumer group coordinator
                    │
                    └── offset commit store
```

## Cluster layout

```
                    ┌── Broker A (leader P0, P3)
                    │
Controller ─────────┼── Broker B (leader P1, follower P0)
(metadata)          │
                    └── Broker C (leader P2, follower P1)
```

**Controller** (or Raft metadata cluster) maintains:
`partition → {leader, replicas, ISR}`

## Produce path

1. Client hashes key → partition id; looks up leader broker.
2. Leader appends record to active segment; writes **WAL** entry; fsync per `acks` policy.
3. Replicate to in-sync replicas (ISR); wait for `min.insync.replicas - 1` acks.
4. Return `(offset, timestamp)` to client.

| `acks` | Durability | Latency |
|--------|------------|---------|
| 0 | fire-and-forget | lowest |
| 1 | leader fsync only | medium |
| all | full ISR commit | highest |

## Consume path

1. Consumer joins group → coordinator assigns partition subset.
2. Consumer fetches from leader starting at `committed_offset + 1`.
3. Process batch; commit offset (sync or async).
4. On rebalance: revoke partitions → commit → reassign.

```
Consumer loop (queue-shaped):
  poll(partition, max_bytes)
  → process(records)
  → commit_offset(partition, last_offset + 1)
```

## On-disk segment layout

```
partition-7/
  00000000000000000000.log    ← active segment
  00000000000001000000.log    ← sealed
  00000000000001000000.index  ← sparse offset → file position
  00000000000001000000.timeindex
```

Retention job deletes segments whose **last timestamp** &lt; cutoff.

**Compaction** (optional): rewrite segments keeping latest value per key — tombstones for deletes.

## Broker failure

1. Controller detects heartbeat timeout.
2. Elect new leader from ISR (never from out-of-sync replica if `unclean.leader.election=false`).
3. Producers/consumers refresh metadata → new leader.
4. Failed broker replays segments on recovery; truncates to leader's high watermark if lagging.

## Capacity

| Resource | Sizing |
|----------|--------|
| Disk | Sequential write ~500 MB/s/broker; plan segment size 1 GB |
| Network | Replication doubles ingress per RF=3 |
| Partitions | ~4k partitions/broker max (file handle + memory overhead) |
| Fetch | Zero-copy sendfile from segment files |

## Comparison table

| System | Log model | Coordination |
|--------|-----------|--------------|
| Kafka | Partitioned segments | ZooKeeper / KRaft (Raft) |
| Pulsar | BookKeeper ledgers | Metadata store |
| Redpanda | Single binary, Raft per partition | Built-in Raft |

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Append-only partition storage | `build-your-own-log` |
| Leader fsync before ack | `build-your-own-wal` |
| Consumer poll / lease / ack | `build-your-own-queue` |

Event streaming is a **replicated append log** with a consumer coordination layer on top — you've built each layer separately.
