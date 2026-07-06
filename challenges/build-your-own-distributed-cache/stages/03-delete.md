# Stage 3: Delete a key

Implement `delete`.

- Existing key → `{"deleted": true}` and the key is gone
- Missing or already-deleted key → `{"deleted": false}`
- After delete, `get` must miss
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** `delete` removes the key if present and returns `deleted: true`. Double-delete returns `deleted: false`. After delete, `get` misses.

Or run: <code>crafters hint distributed-cache --stage delete</code>
</details>
<!-- /crafters-stage-hint -->
