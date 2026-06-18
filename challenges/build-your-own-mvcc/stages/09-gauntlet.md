# Stage 9: The gauntlet

Nothing new to implement — this stage interleaves many overlapping
transactions and crashes you between rounds, checking that every commit, every
conflict, and every recovered read matches a reference model exactly. Bugs that
only surface under interleaving — a snapshot that isn't truly frozen, a
conflict check off by one sequence number, a commit that isn't durable — show
up here.

## What the tester does

Each round it opens several transactions **at the same snapshot** (all begin
before any commit), so they're genuinely concurrent. Then, one at a time, each:

- reads every key and must see the **pre-round** state (proving snapshots are
  frozen — earlier commits this round are invisible),
- writes one or two keys,
- commits — and the tester asserts `CONFLICT` exactly when the transaction wrote
  a key an earlier transaction already committed *this round*, success
  otherwise.

After each round it `SIGKILL`s you, restarts, and verifies the whole keyspace
against the model — committed data must survive, and the version history must
still read correctly.

## Notes

- Everything you need is from earlier stages working *together*: frozen
  snapshots (3), atomic commits (4), first-committer-wins (5), durability (7).
- The usual failures: reusing a single "latest" value instead of versioned
  reads (snapshots leak), comparing the wrong sequence number in the conflict
  check (false or missed conflicts), or rebuilding the sequence counter wrong on
  recovery (post-crash conflicts go haywire).
- Pass this and you've built the concurrency-control core of a real database —
  the part most engineers only ever read about.
