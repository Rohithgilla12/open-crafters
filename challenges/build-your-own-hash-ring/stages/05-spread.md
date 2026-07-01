# Stage 5: Key spread

With several nodes, keys should spread across the ring — not pile onto one node.

## Your task

Ensure `lookup` uses the full hash walk. With three nodes and `replicas=1`,
two thousand keys (`item-0000` … `item-1999`) should land on each node at
least ~15% of the time.

## What the tester checks

- 3 nodes, `replicas=1`, 2000 keys: each node owns ≥ 15%.
- Every lookup matches the reference oracle.

## Notes

- Broken vnode positions or wrong clockwise walk fail here.
