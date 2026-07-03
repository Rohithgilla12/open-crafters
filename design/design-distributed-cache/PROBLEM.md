# Design a distributed cache

Design a **Memcached / Redis Cluster-class** in-memory cache: clients `GET` and `SET` keys with TTL; cluster scales horizontally with automatic sharding and failover.

## Functional requirements

1. **Get(key)** — return value or cache miss; sub-millisecond on hit.
2. **Set(key, value, ttl?)** — store bytes; optional expiration.
3. **Delete(key)** — evict immediately.
4. **CAS / conditional** — `SET if not exists`, `SET if version matches` (compare-and-swap).
5. **Multi-get** — batch fetch N keys in one round trip.
6. **Cluster routing** — client or proxy directs key to correct shard; handles node add/remove.

## Scale

| Metric | Value |
|--------|-------|
| Cluster nodes | 500 |
| Total memory | 50 TB aggregate |
| Keys | 10B (most evicted; working set ~2B hot) |
| Peak GET QPS | 20M aggregate |
| Peak SET QPS | 2M aggregate |
| Avg value size | 512 B |
| TTL | 60s – 24h typical |

Reads dominate **10:1**. **Hot keys** are common (viral content, feature flags).

## Non-functional

- GET p99 **&lt; 1ms** on cache hit within same AZ.
- Survive single-node failure without cluster-wide outage.
- No stale reads after successful `SET` on same key (per-key linearizability or explain trade-off).
- Graceful degradation: cache miss falls through to origin DB — never return corrupt data.

## Your task

Whiteboard **40–50 minutes**:

1. Key → shard mapping; what happens when you add a node?
2. Per-node memory management: eviction policy (LRU vs LFU), memory limits.
3. Replication vs no replication — defend for cache use case.
4. Hot key mitigation: local cache, read replicas, key splitting.
5. Client library vs smart proxy vs sidecar — who owns routing?
6. Cache stampede / thundering herd on popular key expiry.

## Stretch

- Redis-style data structures (sorted sets, pub/sub) — scope creep vs pure KV.
- Persistence (RDB/AOF) — when does a cache become a database?
- Multi-region active cache with invalidation bus.
- Sliding-window rate limiting using cache primitives.
