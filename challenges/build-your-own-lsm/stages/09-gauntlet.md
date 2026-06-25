# Stage 9: The gauntlet

Everything at once: randomized puts and deletes, flushes, compactions, and
repeated `SIGKILL` — plus an offline audit of your SST files.

## Your task

Nothing new to implement. Your store must handle:

- concurrent connections,
- 100 randomized put/del operations across 4 rounds,
- periodic `flush` and `compact` calls,
- 4 crash-recovery cycles,
- and an offline parse proving your on-disk SST files reconstruct to exactly
  the state you served over TCP.

## Tests

Each round: 25 random ops (with flush mid-round from round 2 onward, compact
from round 3 onward), verify all 10 keys, flush, `SIGKILL`, restart, verify
again. After 4 rounds, the tester kills your process and independently parses
every SST file to cross-check your served state.

## Notes

- Only flushed data survives a crash — the tester flushes before each kill.
- If you've made it here, you have a working LSM-tree. Nice work.
