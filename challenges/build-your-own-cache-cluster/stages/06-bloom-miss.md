# Stage 6: Fast negative lookup

`get` on a key never stored must return `hit: false` when bloom says absent — without querying cache nodes.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** If bloom `contains` is false, return `hit: false` without calling cache.

Or run: <code>crafters hint cache-cluster --stage bloom-miss</code>
</details>
<!-- /crafters-stage-hint -->
