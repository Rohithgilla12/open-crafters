# Stage 9: The gauntlet

Concurrent `set`/`get` across **8 connections** (many keys), then verify every
written key is still readable.

Finish with:

- `set` + `ttl_ms` then sleep until expired
- `setnx` on a live key → `stored: false`

Use a mutex (or equivalent) around shared cache state.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Mutex around the map + LRU structure. Many connections set/get distinct keys concurrently; final verify reads them all. Finish with TTL expiry and `setnx` on a live key.

Or run: <code>crafters hint distributed-cache --stage gauntlet</code>
</details>
<!-- /crafters-stage-hint -->
