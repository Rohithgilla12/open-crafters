# Reference architecture — API gateway

## Request path

```
Client ──TLS──▶ Edge ──▶ Regional GW ──▶ Backend pool
                  │           │
                  │           ├─ auth cache
                  │           ├─ rate limiter
                  │           └─ router
```

## Rate limit flow

1. Extract `api_key` from `Authorization`.
2. Resolve tenant + tier → limits `{rpm: 1000, burst: 50}`.
3. `INCR` sliding window in Redis: key `rl:{api_key}:{minute}`.
4. If over limit → `429` + `Retry-After` (never hit backend).
5. Else forward with `X-Request-Id`, `X-Tenant-Id`.

Under race: use **atomic Lua script** or centralized limiter service — same semantics as your graded challenge.

## Auth caching

JWT: verify signature locally if public keys cached.

Opaque API keys: Redis `apikey:{hash} → metadata`, TTL 5m, invalidate on revoke.

## Routing table

Versioned config (etcd / S3):

```yaml
routes:
  - prefix: /v1/payments
    service: payments-svc
    timeout_ms: 3000
```

Sidecar or control plane pushes updates; gateways reload without restart.

## Caching responses

Only `GET` with explicit `Cache-Control`. Key: `(tenant, path, query)`. Never cache authenticated mutations.

## Failure modes

| Issue | Behavior |
|-------|----------|
| Backend 503 | Retry idempotent GETs once; circuit breaker |
| Redis down | Fail closed on rate limits OR degrade to local approximate (product choice) |
| Auth DB slow | Serve from stale cache briefly |

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Per-key quotas | `build-your-own-rate-limiter` |
| Config leader election | `build-your-own-distributed-lock` |
| Sharded counter rings | `build-your-own-hash-ring` |

The gateway is mostly policy + your rate limiter at scale.
