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
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Lookup is a pure function of ring membership and the key — no randomness, no mutation. Call it fifty times; same answer every time, and it must match the PROTOCOL hash walk (FNV-1a positions, sort, clockwise search with wrap. The tester compares against a reference oracle — wrong hash = fail.

Or run: <code>crafters hint hash-ring --stage deterministic</code>
</details>
<!-- /crafters-stage-hint -->
