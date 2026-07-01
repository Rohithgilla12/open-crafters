# Stage 3: Release a lock

Holders must be able to give the lock back early — but only if they present the
current token.

## Your task

Implement `release`.

```json
→ {"id":"1","method":"release","params":{"name":"jobs","token":"…"}}
← {"id":"1","result":{"released": true}}
```

`released: true` only when `token` matches the current holder on an unexpired
lock. Wrong token, stale token, or already-free lock → `released: false` (not
an RPC error).

## What the tester checks

- Release with the correct token clears the lock (`status` → `held: false`).
- Releasing again with the same token returns `released: false`.

## Notes

- Do not delete the lock name from your map on release — just mark it free.
- Expired locks should also return `released: false`.
