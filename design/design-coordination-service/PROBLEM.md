# Design a coordination service

Design an **etcd / ZooKeeper-class** coordination service: a small, strongly consistent metadata store for service discovery, configuration, leader election, and distributed locking.

## Functional requirements

1. **Put(key, value)** — write bytes at a path; create-or-replace or create-if-absent.
2. **Get(key)** — read latest value; optional prefix watch.
3. **Delete(key)** — remove key; recursive delete for subtrees.
4. **Watch(key)** — long-poll or stream changes: created, modified, deleted.
5. **Lease** — time-to-live key binding; keys auto-delete when lease expires (ephemeral nodes).
6. **Compare-and-swap** — atomic `txn`: if `key.version == N` then `put`.

## Scale

| Metric | Value |
|--------|-------|
| Cluster size | 3 or 5 nodes (odd for quorum) |
| Total keys | 10M (small values, metadata only) |
| Avg value size | 1 KB |
| Read QPS | 50k aggregate |
| Write QPS | 5k aggregate |
| Watches | 100k concurrent |
| Lease renewals / sec | 20k |

**Small data, strong consistency** — not a general-purpose database.

## Non-functional

- **Linearizable** reads and writes (or justify `serializable` + `read_index`).
- Survive minority node failure (f=1 for 3-node, f=2 for 5-node).
- Watch latency **&lt; 100ms** p99 from commit to delivery.
- Snapshot + compaction for unbounded revision history.

## Your task

Whiteboard **45–55 minutes**:

1. Why Raft (or Zab) — leader, log, quorum; what goes in each log entry?
2. Write path: client → leader → replicate → apply → watch notify.
3. Read path: leader-only vs quorum read vs `ReadIndex` — trade-offs.
4. Watch mechanism: how to fan out events without polling every key.
5. Lease and ephemeral keys — heartbeat renewal, fencing tokens.
6. Mapping **distributed locks** onto compare-and-swap + lease.

## Stretch

- Multi-key transactions across a key range.
- Auth (mTLS, RBAC per key prefix).
- Defragmentation and on-disk snapshot restore for new members.
- MirrorMaker-style cross-cluster replication (read-only follower cluster).
