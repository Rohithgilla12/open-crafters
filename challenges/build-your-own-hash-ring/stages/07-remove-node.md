# Stage 7: Remove a node

Remove a physical node and its vnodes; keys remap to survivors.

## Your task

Implement `remove_node`:

```json
→ {"id":"1","method":"remove_node","params":{"ring_id":"shrink","node_id":"west"}}
← {"id":"1","result":{"removed": true}}
```

After removal, `lookup` must never return the removed node. Each lookup must
match the reference oracle on remaining nodes.

## What the tester checks

- Removed node never returned.
- Lookups match reference after remove.

## Notes

- `removed: false` if the node wasn't on the ring (no error).
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Drop the physical node and its vnodes. Lookups must never return the removed ID; surviving keys remap via the same clockwise walk on the smaller ring.

Or run: <code>crafters hint hash-ring --stage remove-node</code>
</details>
<!-- /crafters-stage-hint -->
