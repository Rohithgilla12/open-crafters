# Build your own distributed KV

**Meta-compose capstone** — you write the **gateway** only. The harness spawns reference
hash-ring, a 3-node Raft cluster, and an LSM storage shard.

## What you build

A sharded key-value gateway: `put`, `get`, and `delete` with consistent-hash routing
to a replicated Raft shard or a durable LSM shard.

## Environment

| Variable | Service |
|----------|---------|
| `HASHRING_ADDR` | Reference hash ring |
| `RAFT1_ADDR` | Raft cluster node 1 |
| `RAFT2_ADDR` | Raft cluster node 2 |
| `RAFT3_ADDR` | Raft cluster node 3 |
| `LSM_ADDR` | Reference LSM engine |

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | bind | Gateway `ping` + stack boots |
| 2 | put-get | Basic round-trip on LSM shard |
| 3 | routing | Keys land on raft-shard or lsm-shard |
| 4 | raft-write | Replicated writes on Raft shard |
| 5 | raft-read | Reads from a Raft follower |
| 6 | delete | Remove keys on LSM shard |
| 7 | lsm-durable | Values survive LSM flush |
| 8 | concurrent | Parallel writes |
| 9 | gauntlet | Routing + flush + delete |

```sh
crafters start distributed-kv
```
