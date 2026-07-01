# Stage 2: Acquire a lock

The core operation: grant exclusive access to a named lock for a bounded time.

## Your task

Implement `acquire` and `status`.

```json
→ {"id":"1","method":"acquire","params":{"name":"jobs","holder_id":"worker-1","lease_ms":5000}}
← {"id":"1","result":{"token":"…","expires_at_ms":1710000005000}}

→ {"id":"2","method":"status","params":{"name":"jobs"}}
← {"id":"2","result":{"held":true,"holder_id":"worker-1","expires_at_ms":…,"token":"…"}}
```

Grant the lock when it is free or the previous lease has expired. Issue a
unique `token` per acquisition. Set `expires_at_ms = now + lease_ms`.

Return `INVALID_PARAMS` when `name`, `holder_id`, or `lease_ms` is missing,
or when `lease_ms < 1`.

## What the tester checks

- `acquire` returns a non-empty `token` and a future `expires_at_ms`.
- `status` reports `held: true` with matching `holder_id`, `token`, and expiry.
- Missing `name` on `acquire` returns `INVALID_PARAMS`.

## Notes

- Keep locks in a map keyed by `name`; you'll add contention handling next.
- `status` should treat expired leases as not held (`held: false`).
