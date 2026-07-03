# Hints — distributed cache

Open only if you're stuck.

## Sharding

`shard = hash(key) % num_shards` or **consistent hash ring** with vnodes — your **hash ring** challenge.

Each shard = 1 primary node (+ optional read replicas).

On add node: ring moves only adjacent key ranges — not full reshuffle.

## Cache-aside pattern

```
GET:  app → cache → (miss) → DB → populate cache → return
SET:  app → DB → invalidate or update cache
```

Cache is **not** source of truth — origin DB always wins on conflict.

## Eviction

Per-node memory cap → evict when full. **LRU** is default; **LFU** for skewed access.

TTL expiry: lazy (check on read) + active sampling background sweep.

## Hot keys

| Technique | Idea |
|-----------|------|
| Local L1 | App-side micro-cache for top 100 keys |
| Read replicas | Same key served from N replicas (no write split) |
| Key fan-out | `key` → `key:0`…`key:9` — app merges |

## Negative caching

Cache "key not found" with short TTL — **Bloom filter** at edge can skip cache round-trip for keys that never existed.

## Stampede

On expiry: single-flight lock — one request rebuilds; others wait or serve stale.

**Rate limiter** on origin refresh path prevents DB meltdown.

## open-crafters tie-in

- **Hash ring** — shard placement and minimal rebalance on node changes
- **Bloom filter** — "definitely not in cache" fast path before cluster hop
- **Rate limiter** — per-key or per-origin refresh throttling
