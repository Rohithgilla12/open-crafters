# Reference architecture — distributed cache

Spoiler: compare after your own attempt.

## Components

```
App servers ──▶ Client library (ring-aware)
                      │
          ┌───────────┼───────────┐
          ▼           ▼           ▼
       Shard 0      Shard 1      Shard N
       (primary)    (primary)    (primary)
          │           │           │
       [optional read replicas per shard]
```

## Key routing

1. Client hashes key → shard id via **consistent hash ring**.
2. Ring metadata: `shard → {primary_host, replica_hosts, token_range}`.
3. Direct TCP/UDP to primary for writes; reads can target replica (stale OK) or primary (strong).

```
hash("user:42:session") → vnode 847 → Shard 12 @ node-cache-12
```

## GET path

1. Client computes shard; sends `GET key`.
2. Primary looks up in-memory hash table.
3. Hit → return value + TTL remaining.
4. Miss → return `NULL`; app loads from DB and `SET`s.

Optional **Bloom filter** layer at client: if `bloom.mightContain(key) == false` → skip network call entirely.

## SET path

1. Route to primary for key's shard.
2. Store `{value, expires_at, version}`.
3. If over memory limit → evict LRU/LFU entries.
4. Replicate to read replicas async (eventual) or sync (stronger).

| Consistency | Replication |
|-------------|-------------|
| Eventual | Async replicate; faster SET |
| Per-key strong | Sync replicate before ACK |

## Node add/remove

1. Update ring → new shard ownership map.
2. **Rebalance**: new owner streams keys from old owner (or cold start — cache fills naturally).
3. Dual-write window during migration to avoid lost keys.
4. Clients refresh ring config via gossip or config service.

**Hash ring** property: only ~1/N keys move when adding Nth node.

## Hot key handling

```
                    ┌── local L1 (100ms TTL)
App ──▶ proxy ──────┤
                    └── shard primary + 3 read replicas
                              │
                         rate-limited origin fetch
```

Proxy detects QPS &gt; threshold on single key → enable single-flight + local replica fan-out.

## Stampede prevention

```
on miss for hot_key:
  if lock.acquire(hot_key, 50ms):
    value = origin.fetch(hot_key)   # rate-limited
    cache.set(hot_key, value)
    lock.release(hot_key)
  else:
    wait or return stale-if-present
```

## Memory sizing

| Parameter | Guideline |
|-----------|-----------|
| Per-node RAM | 64–128 GB; ~80% for data, rest overhead |
| Working set | Monitor hit ratio; target &gt; 95% |
| Value overhead | ~50 B per key (pointers, TTL metadata) |

50 TB / 500 nodes ≈ 100 GB data per node — fits with overhead.

## Comparison table

| System | Sharding | Replication |
|--------|----------|-------------|
| Memcached | Client-side consistent hash | None (by design) |
| Redis Cluster | Hash slots (16k) | Primary + replicas per slot |
| Twemproxy | Proxy-side ketama ring | Depends on backend |

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Key → shard routing | `build-your-own-hash-ring` |
| Fast negative lookups | `build-your-own-bloom-filter` |
| Origin refresh throttling | `build-your-own-rate-limiter` |

A distributed cache is a **sharded in-memory hash table** with routing and eviction policy — correctness comes from the origin, speed from the ring.
