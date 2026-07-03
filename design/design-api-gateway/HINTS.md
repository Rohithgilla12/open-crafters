# Hints — API gateway

## Tiering

- **Edge** (Cloudflare-style): TLS, DDoS, coarse IP rate limits.
- **Regional gateway**: API key auth, fine quotas, routing.
- **Service mesh** (optional): mTLS to backends.

## Rate limiting

Token bucket or sliding window per `(api_key, endpoint)` — your **rate limiter** challenge.

Central Redis cluster with local token cache + periodic sync for accuracy vs speed trade-off.

## Auth

API key → `tenant_id, tier, scopes` lookup in Redis (cache miss → auth DB).

Attach `X-Tenant-Id` to upstream request — backends trust gateway.

## Routing

Config: `path prefix → service name → upstream pool`.

Health checks mark instances out of rotation. **Consistent hash** on `tenant_id` for sticky debugging (optional).

## open-crafters tie-in

- **Rate limiter** — admission control per key
- **Distributed lock** — config rollout leader (single writer)
- **Hash ring** — shard rate-limit counters or route canary traffic
