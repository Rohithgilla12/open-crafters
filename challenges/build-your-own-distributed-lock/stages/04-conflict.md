# Stage 4: Lock contention

A lock is only useful if two workers cannot both think they hold it.

## Your task

When `acquire` is called while another holder has an **unexpired** lease,
return error `LOCK_HELD` instead of granting.

```json
→ {"id":"1","method":"acquire","params":{"name":"shared","holder_id":"alpha","lease_ms":5000}}
← {"id":"1","result":{"token":"…","expires_at_ms":…}}

→ {"id":"2","method":"acquire","params":{"name":"shared","holder_id":"beta","lease_ms":5000}}
← {"id":"2","error":{"code":"LOCK_HELD","message":"…"}}
```

The original holder must remain unchanged.

## What the tester checks

- Second `acquire` while the first lease is active returns `LOCK_HELD`.
- `status` still shows the first holder.

## Notes

- Check expiry before rejecting: an expired lease is free for a new acquirer.
- Same holder trying again without releasing should also get `LOCK_HELD` (only
  one active lease per lock).
