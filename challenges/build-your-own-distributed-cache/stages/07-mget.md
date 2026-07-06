# Stage 7: Multi-get

Implement `mget`.

Params: `{"keys": ["k1", "k2", ...]}` — **1 to 50** keys.

Result: `{"entries": [...]}` in the **same order** as `keys`. Each entry is
either a hit (`hit`, `value`, `version`) or `{"key": "...", "hit": false}`.

Empty array → `INVALID_PARAMS`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Return `entries` in the same order as the input `keys` array. Each element is either a hit (value + version) or `hit: false`. Reject empty or >50 keys with `INVALID_PARAMS`.

Or run: <code>crafters hint distributed-cache --stage mget</code>
</details>
<!-- /crafters-stage-hint -->
