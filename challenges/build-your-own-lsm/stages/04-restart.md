# Stage 4: Recover after restart

A flushed SST file is useless if you can't read it back after a crash.

## Your task

On startup, scan `<data-dir>/sst/` for `.sst` files and load them so that
`get` returns the correct values. Start with an empty memtable.

Recovery order: read SST files in sequence order (sorted by filename). For
duplicate keys across files, the **newer file wins**.

## Tests

The tester writes several keys (including an overwrite and a delete), calls
`flush`, kills your process with `SIGKILL`, restarts it, and checks every
key. Then it writes more keys, flushes again, kills, and restarts a second
time.

## Notes

- Tombstones (`value_len=0`) aren't tested here yet — stage 7 covers deletes
  on disk. But in-memory deletes before flush should not appear in the SST
  file at all (the key simply isn't written).
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** On startup, list `sst/*.sst`, sort by filename, and remember the paths. You don't need to load all data into RAM — lazy read per `get` is fine. Bump `next_seq` from the highest file number.

Or run: <code>crafters hint lsm --stage restart</code>
</details>
<!-- /crafters-stage-hint -->
