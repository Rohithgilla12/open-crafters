# Hints — multi-tenant SaaS

## Default pattern

**Shared database, shared schema**, every row has `tenant_id`. Cheapest ops; requires discipline.

## Tenant context

Gateway validates JWT → extracts `tenant_id` → passes in signed internal header. **Never** trust client-supplied tenant ID alone.

## Enforcement layers

1. App middleware: `WHERE tenant_id = ctx.tenant`
2. DB row-level security (Postgres RLS) as safety net
3. Integration tests that attempt cross-tenant access

Your **MVCC** challenge: transactions still scoped to tenant.

## Metering

Async counter increment per request: `usage(tenant, metric, hour) += 1` via queue — don't block latency.

## Whales

Threshold triggers **dedicated shard** or `tenant_id` routing to isolated DB — gradual migration with dual-write.

## open-crafters tie-in

- **MVCC** — transactional tenant data
- **Rate limiter** — per-tenant quotas
- **ID generator** — tenant-safe resource IDs (no enumerable cross-tenant)
