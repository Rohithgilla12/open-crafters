# Stage 5: Recover from any log

A file format is only real if *someone else* can write it and you can read
it. In this stage the tester stops your process, throws away your log,
**writes its own `wal.log`**, and restarts you. If your recovery only
handles logs your own code produced, this is where that surfaces.

## Your task

Nothing new to implement — this stage validates that your stage-4 reader and
your stage-3 recovery are honest implementations of the spec, not mirrors of
your writer's quirks.

## Tests

The crafted log exercises the corners readers tend to skip:

- an **overwrite** (the later record must win),
- a **delete** of a previously set key,
- a value containing **unicode** (multi-byte UTF-8 — if you confuse byte
  length with character count, the CRC or framing will break here),
- an **empty-string value** (`""` is a value; `found` must be `true`).

After restart, the tester reads all of it back through `get`.

## Notes

- If stage 4 passed but this fails, the bug is almost always in one of:
  byte-length vs string-length, reading exactly `length` bytes (not a line,
  not a buffer), or applying records in file order.
- This property — *state is fully reconstructible from the log alone* — is
  what you're actually building. Hold onto it; the next two stages attack it.
