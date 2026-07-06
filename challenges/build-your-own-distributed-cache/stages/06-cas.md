# Stage 6: Compare and swap

Implement `cas`.

Params: `key`, `expected_version`, `value`, optional `ttl_ms`.

- Matching live version → `{"swapped": true, "version": <new>}`
- Stale version, missing, or expired key → `{"swapped": false}` (no change)

The tester sets version 1, CAS to version 2, then retries with `expected_version=1` (must fail).
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Swap only when `expected_version` matches the live entry's version. On success increment version and return `swapped: true`. Stale version or missing key → `swapped: false` without changing state.

Or run: <code>crafters hint distributed-cache --stage cas</code>
</details>
<!-- /crafters-stage-hint -->
