# Stage 8: Multi-file recovery

Production LSM-trees always have multiple SST files on disk between
compactions. Recovery must handle the full layered state correctly.

## Your task

Ensure startup recovery works with **multiple SST files** at once:

- Read all files in sequence order.
- Apply newer-over-older precedence for overlapping keys.
- Respect tombstones across files.
- Serve correct `get` results after restart.

Also ensure `compact` produces a durable merged file that survives another
crash.

## Tests

The tester writes 8 keys with periodic flushes, deletes one, overwrites
another, kills the process, and verifies all keys. It then compacts, kills
again, and verifies again. Finally it parses your SST files offline and
checks they reconstruct to the served state.

## Notes

- This stage doesn't introduce new RPC methods — it's a stress test of
  recovery and compaction correctness with realistic file counts.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Flushed SSTs are the durable source of truth. After kill + restart, reload the SST index and verify memtable is empty (unflushed writes may be lost unless you also WAL the memtable — this challenge doesn't require that).

Or run: <code>crafters hint lsm --stage durability</code>
</details>
<!-- /crafters-stage-hint -->
