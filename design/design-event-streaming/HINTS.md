# Hints — event streaming

Open only if you're stuck.

## Partitioning

`partition = hash(key) % num_partitions` — same key always lands in same partition for ordering.

No key? round-robin or sticky batching. **Hash ring** can assign partitions to brokers with minimal movement on broker add/remove.

## Produce path

```
Client ──▶ partition leader ──▶ append to local log (WAL)
                │
                ├── replicate to followers (ISR)
                └── ack after min.in.sync.replicas commit
```

Leader handles all writes for a partition; followers tail the log.

## On-disk log

Each partition is an **append-only log** — your **log** challenge: segment files + offset index.

Segments roll at size/time boundary. Retention deletes whole segments from the tail.

## Consumer groups

A **coordinator** (or embedded group protocol) tracks:
- which consumer owns which partition
- last committed offset per partition

Rebalance on member join/leave — stop-the-world vs cooperative sticky assignment.

## Offset commits

Sync commit after process (at-least-once) vs async (may lose on crash).

**Queue** semantics appear in the consumer worker loop: poll → process → ack offset.

## Durability

Replication = N copies of the same ordered log. Leader failure → promote ISR follower.

Unclean leader election trades availability for consistency — defend your choice.

## open-crafters tie-in

- **Log** — partitioned append-only segments with offsets
- **WAL** — fsync-before-ack on the leader's hot path
- **Queue** — consumer poll/lease/ack pattern mirrors your queue challenge
