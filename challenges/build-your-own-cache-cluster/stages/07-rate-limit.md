# Stage 7: Stampede guard

Call rate-limiter `take` on `rl:<key>` before cache access. Return gateway error `RATE_LIMITED` when `allowed: false`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `take` on `rl:<key>` before cache I/O; return `RATE_LIMITED` when `allowed: false`.

Or run: <code>crafters hint cache-cluster --stage rate-limit</code>
</details>
<!-- /crafters-stage-hint -->
