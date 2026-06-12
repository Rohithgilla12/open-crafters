# Stage 7: Detect corruption

A torn tail is honest damage — the file just ends early. Media corruption is
nastier: a byte flips **in the middle** of the log, and everything around it
still looks plausible. This is what the CRC in every record has been waiting
for. The tester flips one byte inside record 3 of 5 and restarts you.

## Your task

The same rule as stage 6, applied without mercy: recovery **stops at the
first invalid record and discards it and everything after it** — even
though records 4 and 5 have perfectly valid CRCs.

Why discard intact-looking records after a corruption? Because the log's
meaning is *sequential*: once one record is unreadable, you can no longer
prove the later ones mean what they appear to mean (was the corrupt record a
`del`? an overwrite? part of a multi-record operation in a richer system?).
Postgres does exactly this — redo stops at the first invalid record.

## Tests

After the byte-flip and restart: `k1`, `k2` present; `k3`, `k4`, `k5` gone —
and never served with a corrupted value. Then a new write, a re-parse of
your file (must be exactly `k1`, `k2`, `fresh` — corrupt tail truncated),
and another kill to prove the new write was durable.

## Notes

- If you implemented stage 6 as "validate each record, stop at the first
  failure, truncate, then append", this stage may already pass. That's the
  sign of the right design: torn tails and flipped bytes are the *same
  failure* to a reader that trusts nothing it can't checksum.
- The failure mode this stage catches: skipping the bad record and continuing
  to replay (`k4`, `k5` present = fail), or crashing on boot instead of
  recovering.
