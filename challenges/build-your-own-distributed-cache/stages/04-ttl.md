# Stage 4: TTL expiration

Support optional `ttl_ms` on `set` (and later on `setnx` / `cas`).

- `ttl_ms > 0` → entry expires after that many milliseconds (wall clock)
- Expired keys behave as misses on `get`
- Immediate `get` after `set` with `ttl_ms=200` must hit; after ~280ms sleep, must miss
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Store `expires_at = now + ttl_ms` on set. On every `get`, if `now >= expires_at`, remove the entry and return a miss.

Or run: <code>crafters hint distributed-cache --stage ttl</code>
</details>
<!-- /crafters-stage-hint -->
