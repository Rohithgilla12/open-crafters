# Reference architecture — URL shortener

## Components

```
Shorten API ──▶ ID service ──▶ codes DB
                    │
Redirect CDN ──▶ Redis cache ──▶ codes DB (cold)
                    │
Click beacon ──▶ Kafka ──▶ analytics warehouse
```

## Shorten path

1. Validate URL, auth user.
2. If custom alias: check uniqueness; else allocate `id` from counter, base62 encode.
3. Insert row; warm cache.
4. Return short URL.

Idempotency: same user + same long URL → return existing code (optional product choice).

## Redirect path

1. CDN receives `GET /Ab3xK9`.
2. Edge cache (geo-distributed) — if hit, `302` immediately.
3. Miss → regional Redis → miss → DB → set Redis + CDN.
4. Fire-and-forget click event (pixel or server-side log).

Target: **&lt;1ms** edge, **&lt;5ms** regional cache miss.

## Capacity

| Layer | Sizing |
|-------|--------|
| Redis | ~20% of hot codes, LRU; 500k RPS/shard |
| DB | Read replicas for cache miss; writes to primary |
| CDN | Absorbs majority of redirect QPS |

500B URLs won't fit in RAM — **most codes are cold**. Cache working set of recent viral links.

## Custom aliases

Same table; `is_custom=true`. Reserve blocked words list. Higher collision rate → user-facing errors.

## Expiration

Lazy: check `expires_at` on read; background job deletes expired rows + cache purge.

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Unique short codes | `build-your-own-id-generator` |
| Fast existence checks | `build-your-own-bloom-filter` |
| Click log archive | `build-your-own-object-store` |

Classic read-heavy system — generation is write-once, redirect is infinite read.
