# Design multi-tenant SaaS isolation

Design the **platform layer** for B2B SaaS (Notion / Datadog style): many customer organizations (tenants) share your infrastructure, but each must see only their data — with fair resource sharing and usage-based billing.

## Functional requirements

1. **Tenant onboarding** — create org, invite users, assign roles.
2. **Data isolation** — tenant A cannot read/write tenant B's resources.
3. **Quotas** — API calls, storage, seats per plan tier.
4. **Metering** — track usage for invoices (API calls, GB stored).
5. **Admin** — support impersonation with audit trail (optional).
6. **Export / delete** — GDPR tenant offboarding.

## Scale

| Metric | Value |
|--------|-------|
| Tenants | 200k |
| Users | 5M |
| Requests / sec | 80k peak |
| Largest tenant | 10× median traffic ("whale") |

## Non-functional

- **No cross-tenant leaks** — security invariant, not best-effort.
- Noisy neighbor: one tenant's spike shouldn't starve others (within tier).
- Whale tenants isolatable without full rewrite.

## Your task

Whiteboard **40–50 minutes**:

1. Tenancy model: shared table + `tenant_id` vs silos — trade-offs.
2. How `tenant_id` flows request → DB → cache keys.
3. Row-level security or app-layer enforcement?
4. Metering hooks in the request path.
5. Migration path when a tenant outgrows the pool.

## Stretch

- Data residency (EU tenants on EU cells).
- Per-tenant feature flags and schema customization.
