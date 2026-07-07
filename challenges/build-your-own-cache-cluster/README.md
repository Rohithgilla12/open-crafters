# Build your own cache cluster

**Meta-compose capstone** — you write the **gateway** only. The harness spawns reference
hash-ring, bloom-filter, rate-limiter, and two cache nodes.

## What you build

A Memcached-style client gateway: `set`, `get`, `delete`, and `mget` with shard
routing, bloom negative cache, and per-key rate limiting.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | bind | Gateway `ping` + stack boots |
| 2 | set-get | Basic round-trip |
| 3 | delete | Remove keys |
| 4 | routing | Keys land on the correct cache node |
| 5 | bloom | Stored keys added to filter |
| 6 | bloom-miss | Fast miss for unknown keys |
| 7 | rate-limit | `RATE_LIMITED` under tight limiter |
| 8 | mget | Batch reads |
| 9 | gauntlet | TTL + delete + delayed get |

```sh
crafters start cache-cluster
```
