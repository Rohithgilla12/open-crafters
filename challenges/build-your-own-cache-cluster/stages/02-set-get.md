# Stage 2: Set and get

Implement `set` and `get` with ring routing to the correct cache node. Add keys to bloom filter `keys` after a successful `set`.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `lookup` ring `cache` → cache `set` on the right node → bloom `add` on `keys`.

Or run: <code>crafters hint cache-cluster --stage set-get</code>
</details>
<!-- /crafters-stage-hint -->
