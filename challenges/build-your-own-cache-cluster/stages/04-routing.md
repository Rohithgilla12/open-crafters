# Stage 4: Shard routing

Keys must be stored on the cache node returned by hash-ring `lookup` for ring `cache` — `node1` and `node2` each hold distinct keys.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Map `node1` → `CACHE_NODE1_ADDR`, `node2` → `CACHE_NODE2_ADDR`.

Or run: <code>crafters hint cache-cluster --stage routing</code>
</details>
<!-- /crafters-stage-hint -->
