# Stage 5: Set if not exists

Implement `setnx` — store only when the key is absent (or expired).

- Success → `{"stored": true, "version": <int>}`
- Key already live → `{"stored": false}` (no overwrite)
- After `delete`, `setnx` may succeed again
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Only store when the key is missing or expired. Return `stored: false` if a live entry exists — do not overwrite.

Or run: <code>crafters hint distributed-cache --stage setnx</code>
</details>
<!-- /crafters-stage-hint -->
