# Stage 8: Virtual nodes

More replicas per physical node flattens load imbalance.

## Your task

Same three nodes on two rings: `replicas=1` vs `replicas=50`. With 600 keys
each, the high-replica ring must have lower (max−min) key count spread.

## What the tester checks

- Spread with replicas=50 < spread with replicas=1.
- Lookups match the reference on both rings.

## Notes

- Virtual nodes = more points on the ring per physical machine.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Higher `replicas` places more vnodes per physical node, flattening load. With three nodes and 600 keys, `replicas=50` should have a smaller (max−min) key count spread than `replicas=1`.

Or run: <code>crafters hint hash-ring --stage virtual-nodes</code>
</details>
<!-- /crafters-stage-hint -->
