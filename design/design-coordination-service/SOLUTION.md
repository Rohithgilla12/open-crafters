# Reference architecture — coordination service

Spoiler: compare after your own attempt.

## Cluster layout

```
         Clients (SDK)
              │
    ┌─────────┼─────────┐
    ▼         ▼         ▼
  Node 1    Node 2    Node 3
 (follower) (leader)  (follower)
    │         │         │
    └─────────┴─────────┘
         Raft peers
    WAL + snapshot per node
```

Typical deployment: **3 nodes** across 3 AZs (tolerate 1 AZ loss).

## Write path

1. Client sends `Put(/config/rate-limit, "1000")` to any node.
2. If not leader → redirect or forward to leader.
3. Leader appends to **Raft log**: `{op: put, key, value, lease_id?}`.
4. Replicate to followers; commit index advances on majority.
5. Apply to in-memory (+ optional bbolt) state machine.
6. Bump `mod_revision`; notify watchers.
7. ACK client with `{header_revision, mod_revision}`.

Linearizable: any read after successful write sees new value (from leader or `ReadIndex`).

## Read path

| Mode | Mechanism | Cost |
|------|-----------|------|
| Linearizable | Leader `ReadIndex` or read from leader after confirm leadership | 1 RTT |
| Serializable | Read from any node (may be stale on partitioned follower) | 0 extra |
| Quorum read | Contact majority — rarely needed if Raft used correctly | 2 RTT |

Default SDK: reads go to leader or use `ReadIndex` barrier.

## Watch path

```
Client                          Leader
  │── watch(/jobs/, rev=100) ──▶│ register watcher
  │                               │
  │◀── stream: PUT /jobs/7 ───────│ on apply(rev=101)
  │◀── stream: DEL /jobs/3 ───────│ on apply(rev=102)
```

Watchers keyed by prefix in a trie. Event buffer for slow consumers; cancel if lag &gt; threshold.

## Lease lifecycle

```
1. Grant(lease_id, TTL=10s)     → Raft log entry
2. Put(key, val, lease_id)    → bound to lease
3. KeepAlive(lease_id)        → reset TTL (heartbeat stream)
4. Lease expired (no heartbeat) → apply delete of all bound keys
```

Used for:
- **Service registry** — `/services/api/instance-42` disappears on crash
- **Leader election** — first creator of `/election/payment` wins
- **Lock liveness** — stale lock auto-releases

## Distributed lock recipe

```
acquire:
  txn(if not exists /locks/batch-7):
    put /locks/batch-7 = {node_id, fencing_token=mod_revision}
    attach lease

work:
  pass fencing_token to downstream DB writes

release:
  txn(if mod_revision == my_token):
    delete /locks/batch-7
```

Your **distributed lock** challenge implements this pattern end-to-end.

## Snapshot & compaction

```
Periodic snapshot: {full keyspace tree, revision} → disk
Raft log truncated before snapshot index
New member: install snapshot + replay tail log
```

Prevents unbounded **WAL** growth.

## Failure modes

| Event | Behavior |
|-------|----------|
| Leader crash | Election ~300ms–1s; uncommitted writes fail |
| Follower crash | Cluster continues; replace node |
| Network partition | Minority partition cannot elect leader → read-only or unavailable |
| Slow watcher | Server drops watch; client re-watch from last revision |

## Capacity

| Resource | Limit |
|----------|-------|
| Keyspace | 10M keys × 1 KB ≈ 10 GB — fits in RAM |
| Write throughput | ~10k/s on SSD-backed Raft (5k is comfortable) |
| Watch fan-out | O(watchers per prefix); shard not needed at this scale |

Coordination stores stay **small and strongly consistent** — don't use them as a blob or event log.

## Comparison table

| System | Protocol | Data API |
|--------|----------|----------|
| etcd | Raft | KV + lease + watch + txn |
| ZooKeeper | Zab | ZNode tree + ephemeral + watch |
| Consul | Raft | KV + DNS + health |

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Consensus & replication | `build-your-own-raft` |
| Lock acquire / renew / fence | `build-your-own-distributed-lock` |
| Durable log on each node | `build-your-own-wal` |

A coordination service is **Raft + a watchable KV API + leases** — the primitives you've built map directly onto etcd's core.
