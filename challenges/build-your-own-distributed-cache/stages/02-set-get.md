# Stage 2: Set and get

Implement `set` and `get`.

- `set` params: `key`, `value` (UTF-8 strings) → `{"version": <int>}`
- `get` on miss → `{"hit": false}`
- `get` on hit → `{"hit": true, "value": "...", "version": <int>}`

First store on a key starts at **version 1**; each successful overwrite
increments the version.

Missing `key` or `value` on `set` → `INVALID_PARAMS`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `map[key]entry` with `value`, `version`, and optional `expires_at`. `get` on a missing key returns `hit: false`. First `set` starts at version 1; each overwrite increments version.

Or run: <code>crafters hint distributed-cache --stage set-get</code>
</details>
<!-- /crafters-stage-hint -->
