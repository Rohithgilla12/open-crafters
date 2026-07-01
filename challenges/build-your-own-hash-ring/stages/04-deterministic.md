# Stage 4: Deterministic lookup

The same key must always map to the same node for a fixed ring membership.

## Your task

Implement clockwise lookup per [PROTOCOL.md](../PROTOCOL.md): build vnodes,
sort, walk from `hash_key(key)`, wrap if needed, return `node_id`.

## What the tester checks

- 50 lookups of the same key return the same node.
- That node matches the reference oracle (FNV-1a positions + tie-break).

## Notes

- Tie-break: same position → lexicographically smaller `node_id` wins when sorting.
