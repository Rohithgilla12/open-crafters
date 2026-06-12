# Stage 6: Torn writes

Disks don't write your record atomically. Power fails *during* the append,
and what's left on disk is the first half of a record — a **torn write**.
Every real WAL implementation lives or dies by how it handles this; now
yours will too. The tester kills your process, **truncates `wal.log`
mid-record**, and restarts you.

## Your task

On recovery, when you hit a record that can't be completed —

- fewer than 8 bytes left for a header, or
- `length` claims more bytes than remain in the file, or
- the CRC doesn't match

— **stop replaying, keep everything before it, discard the tail.** Then
**truncate the file** back to the last valid record before accepting new
writes, so the log always parses cleanly from byte 0.

## Tests

The tester writes `k1`–`k5`, kills you, cuts 3 bytes off the end of the log,
restarts, and expects `k1`–`k4` present and `k5` gone. Then it writes `k6`
and re-parses your file: it must contain exactly 5 clean records — proving
you truncated the torn tail rather than appending after garbage. One more
kill verifies `k6` was durable.

## Notes

- The subtle bug this stage exists to catch: recovering correctly in memory
  but appending the next record *after* the torn bytes. The file then has
  garbage in the middle, and the **next** restart silently loses everything
  after it. Truncate first, then append.
- Is discarding `k5` "losing an acknowledged write"? In this test, the
  tester truncated the file behind your back, so yes — but in reality,
  write-before-ack means a torn record is precisely one that was never
  acknowledged. Prefix recovery loses nothing a client was ever promised.
  That's the deep reason WALs work.
- Don't trust `length` before checking it against the remaining file size —
  a torn header can claim a 3GB payload.
