# Stage 8: LRU eviction

Implement `configure` with `max_keys >= 1`.

When at capacity, inserting a **new** key evicts the **least-recently-used**
key. `get` and successful writes on an existing key mark it most recent.

The tester uses `max_keys=3`, stores `k1,k2,k3`, reads `k1`, inserts `k4`,
and expects **`k2` evicted** (`k1`, `k3`, `k4` still hit).
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** After `configure max_keys`, track recency (linked list or `OrderedDict`). `get` and successful writes mark a key most-recent. Inserting a **new** key at capacity evicts the least-recently-used key.

Or run: <code>crafters hint distributed-cache --stage lru</code>
</details>
<!-- /crafters-stage-hint -->
