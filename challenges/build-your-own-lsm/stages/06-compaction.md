# Stage 6: Compact SSTables

Multiple SST files accumulate over time. Compaction merges them into one
sorted file and reclaims space from overwritten keys.

## Your task

Implement `compact`:

```
→ {"id": "1", "method": "compact", "params": {}}
← {"id": "1", "result": {}}
```

**Effects:**

1. Read all `.sst` files in `<data-dir>/sst/`.
2. Merge them into one new SST file with the next sequence number.
3. For duplicate keys, the value from the **newer** file (higher sequence
   number) wins.
4. Delete the old SST files.
5. Fsync the new file before acknowledging.

The memtable is not modified by `compact`.

## Tests

The tester puts a key, flushes, overwrites the key, flushes again (two SST
files with the same key), calls `compact`, and checks:

- exactly one SST file remains,
- `get` returns the latest value,
- the merged SST on disk contains the correct sorted entries.

## Notes

- After compaction, old files must be deleted — not left around.
- The merged file must still conform to SST1 format.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Read every SST, merge into one sorted run (newer values win on duplicate keys), write a single new SST, delete the old files. Same SST1 format — you're just reducing read amplification.

Or run: <code>crafters hint lsm --stage compaction</code>
</details>
<!-- /crafters-stage-hint -->
