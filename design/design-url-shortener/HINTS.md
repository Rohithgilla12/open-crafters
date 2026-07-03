# Hints — URL shortener

## Code generation

- **Counter + base62**: monotonic ID from your **ID generator**, encode to 7 chars — no collisions, sortable.
- **Hash + truncate**: hash long URL; collision check with **Bloom filter** first, then DB.
- **Random**: retry on unique constraint violation.

Counter/base62 is the production default at scale.

## Storage

`codes` table or KV: `code (PK) → {long_url, owner_id, created_at, expires_at}`.

Custom aliases are the same row — uniqueness on `code` covers both.

## Redirect path

```
CDN edge → cache lookup (Redis) → on miss, DB → populate cache → 302
```

Never hit DB on 95%+ of reads.

## Analytics

Append click events to a **log** / queue asynchronously. Redirect returns immediately.

## open-crafters tie-in

- **ID generator** — short code from snowflake counter slice
- **Bloom filter** — "code probably exists" before DB on create
- **Object store** — optional archive of click logs
