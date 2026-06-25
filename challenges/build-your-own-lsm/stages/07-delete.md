# Stage 7: Tombstones

Deletes in an LSM-tree can't simply remove keys from immutable SST files.
Instead, you write a **tombstone** — an entry with `value_len = 0` — that
hides the key on read.

## Your task

When `del` is called and the key is later flushed, write a tombstone entry
(`value_len = 0`) to the SST file instead of omitting the key.

On `get` and `scan`, tombstones hide the key — even if an older SST file
still contains a live value for that key (the newer tombstone wins).

## Tests

The tester puts two keys, flushes, deletes one, flushes the tombstone, and
checks that the deleted key is invisible. It kills and restarts to verify
tombstones survive recovery. It also parses your SST files to confirm a
tombstone entry exists for the deleted key.

## Notes

- A tombstone in the memtable (from `del` before flush) should hide the key
  immediately, without waiting for flush.
- Tombstones in compacted files should still hide keys after compaction.
