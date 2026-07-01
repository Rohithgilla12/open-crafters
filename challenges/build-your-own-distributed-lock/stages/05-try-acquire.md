# Stage 5: Try without blocking

Not every caller wants an error on contention — sometimes you just want to know
whether you got the lock.

## Your task

Implement `try_acquire` with the same grant rules as `acquire`, but return
`acquired: false` instead of `LOCK_HELD` when the lock is held.

```json
→ {"id":"1","method":"try_acquire","params":{"name":"work","holder_id":"a","lease_ms":5000}}
← {"id":"1","result":{"acquired":true,"token":"…","expires_at_ms":…}}

→ {"id":"2","method":"try_acquire","params":{"name":"work","holder_id":"b","lease_ms":5000}}
← {"id":"2","result":{"acquired":false}}
```

Never return an RPC error solely because of contention.

## What the tester checks

- `try_acquire` on a held lock returns `acquired: false` without error.
- `acquire` still returns `LOCK_HELD` under the same conditions.

## Notes

- Share the grant logic between `acquire` and `try_acquire`; only the
  contention response differs.
