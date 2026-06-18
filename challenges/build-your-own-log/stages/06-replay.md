# Stage 6: Replay and batching

The payoff of "reads don't consume": a brand-new consumer can read the topic
from offset 0 and see the entire history — replay it — to rebuild its state.
And because logs are huge, reads come in **batches**: you ask for at most `max`
records and get a `next_offset` to continue from.

## Your task

- Honor `max` in `read`: return at most `max` records and set `next_offset`
  accordingly, so a consumer can page through the log.
- Guarantee replayability: reading from offset 0 returns the whole log, and
  returns it again identically on a second read.

## Tests

Append six records. `read(offset=0, max=2)` → first two, `next_offset` 2;
continue to page through all of them. A full read from 0 returns all six, twice
in a row, unchanged.

## Notes

- `next_offset = offset + len(returned)`. Paging is just calling `read` again
  with the previous `next_offset`.
- No new storage — this is the read path you already have, plus a limit.
