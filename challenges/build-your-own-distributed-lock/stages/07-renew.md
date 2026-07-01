# Stage 7: Renew a lease

Long-running work needs to extend its lease without releasing and re-acquiring
(which would invite a race).

## Your task

Implement `renew`.

```json
→ {"id":"1","method":"renew","params":{"name":"jobs","token":"…","lease_ms":3000}}
← {"id":"1","result":{"expires_at_ms":…}}
```

New expiry is `max(now, current_expires_at_ms) + lease_ms`. Wrong token or
free/expired lock → `NOT_HOLDER`. `lease_ms < 1` → `INVALID_PARAMS`.

## What the tester checks

- Valid `renew` returns a later `expires_at_ms` than before.
- Wrong token returns `NOT_HOLDER`.
- `lease_ms: 0` returns `INVALID_PARAMS`.

## Notes

- Extending from `max(now, current_expires)` means a renew on an unexpired lock
  stacks time onto the existing lease, not onto `now` alone.
- `release` still uses `released: false` for wrong tokens; only `renew` errors.
