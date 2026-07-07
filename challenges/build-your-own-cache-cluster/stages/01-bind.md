# Stage 1: Boot the stack

Implement gateway `ping`. The harness pings hash-ring, bloom-filter, rate-limiter, and both cache nodes directly.

Read `HASHRING_ADDR`, `BLOOM_ADDR`, `LIMITER_ADDR`, `CACHE_NODE1_ADDR`, `CACHE_NODE2_ADDR`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Read all five env addresses. Gateway only needs `ping`; harness pings references directly.

Or run: <code>crafters hint cache-cluster --stage bind</code>
</details>
<!-- /crafters-stage-hint -->
