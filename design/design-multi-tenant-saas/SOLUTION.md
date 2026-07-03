# Reference architecture — multi-tenant SaaS

## Tenancy models

| Model | Isolation | Cost | When |
|-------|-----------|------|------|
| Shared DB + `tenant_id` | App + RLS | Low | Default &lt; whale |
| Schema per tenant | Medium | Medium | Compliance niche |
| DB per tenant | Strong | High | Whales, enterprise |

Start shared; promote outliers.

## Request flow

```
HTTPS ──▶ Auth ──▶ tenant context ──▶ rate limit ──▶ app ──▶ DB
                         │                │
                         │                └─ tier quotas
                         └─ tenant_id in every query
```

## Data model

```sql
resources(id, tenant_id, type, payload, ...)
UNIQUE (tenant_id, id)
INDEX (tenant_id, created_at)
```

All queries include `tenant_id` from context — linter enforces in CI.

## Quotas

| Tier | API rpm | Storage |
|------|---------|---------|
| Free | 100 | 1 GB |
| Pro | 10k | 100 GB |

Enforced at gateway with your **rate limiter** semantics. Storage via nightly aggregation job.

## Metering pipeline

```
API middleware ──▶ usage events queue ──▶ aggregator ──▶ billing DB
```

Events: `{tenant_id, metric, quantity, ts}`. Idempotent with event UUID from **ID generator**.

## Whale migration

1. Flag tenant `tier=dedicated`.
2. Provision new DB; dual-write new rows.
3. Backfill historical data.
4. Flip read routing; drain shared shard.

## Offboarding

Soft-delete tenant → async purge job per `tenant_id` partition — auditable log of deletions.

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Tenant-scoped transactions | `build-your-own-mvcc` |
| Per-tenant limits | `build-your-own-rate-limiter` |
| Opaque resource IDs | `build-your-own-id-generator` |

Multi-tenancy is policy everywhere — the primitives you built are the enforcement layer.
