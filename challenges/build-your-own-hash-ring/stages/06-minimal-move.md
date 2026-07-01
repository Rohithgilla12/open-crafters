# Stage 6: Add node (minimal move)

Consistent hashing: adding a node should only steal keys that now belong to it.

## Your task

After 3 nodes and 200 recorded keys, add a 4th. Fewer than 45% of keys should
change owner; every lookup must still match the oracle with four nodes.

## What the tester checks

- `< 90` of 200 keys change owner when adding the 4th node.
- Post-add lookups match reference with 4 nodes.

## Notes

- Moving ~25% is typical for n=4; moving ≥45% suggests reshuffling everything.
