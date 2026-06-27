# Walkthrough — Build your own WAL

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** (a design nudge for when you're stuck) followed by **How
it works** (read this *after* you pass the stage, to check your model against
the reference's). No code — the point is the design.

`crafters hint wal` prints just the hint for your next stage;
`crafters walkthrough wal --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** A line-delimited JSON server is a loop over lines: read one,
> decode, dispatch on `method`, write one line back, flush. Handle each
> connection independently so two can interleave.

**How it works:** The reference keeps one dispatch function on `method`, with
`ping` returning `{"message":"pong"}`. The transport — read a line, decode,
encode, write with a trailing newline, flush — is written once and never
touched again; every later stage just adds a method or changes what happens
*under* `set`/`del`. `--data-dir` is accepted and ignored until persistence.

## kv — An in-memory key-value store

> **Hint:** A map behind a lock. Pin the exact response shapes now —
> `found`/`deleted` booleans — because the durable stages reuse these
> operations verbatim and only change what happens underneath.

**How it works:** `set` writes the map, `get` returns `{value, found}`, `del`
returns `{deleted}` reflecting whether the key existed. The reference guards
the map with a single mutex from the start — the gauntlet's concurrent
workload is unforgiving of a race added late. Deletes are reported precisely
(`deleted:false` for a missing key) because the tester models every reply.

## persist — Survive a crash

> **Hint:** Write-before-ack. The operation must be on disk *before* you send
> the response — an ack for data that's only in memory is a lie waiting for a
> power cut. Any persistence works for this stage; the log comes next.

**How it works:** Before replying to `set`/`del`, the reference appends the
operation to disk and flushes (and fsyncs) it, then acknowledges. On startup
it replays whatever it persisted to rebuild the map. This stage accepts any
scheme — even rewriting a whole JSON file per write — but the reference
already uses the append-only log it will need in the next stage, so there's
nothing to throw away. The discipline, not the format, is the lesson:
*acknowledged ⇒ durable*.

## format — Write the log format

> **Hint:** One record per acknowledged op: `crc(4) | length(4) | payload`,
> all little-endian, no file header. Compute the CRC over the length bytes
> *and* the payload, so damage to the frame itself is detectable.

**How it works:** Each `set`/`del` serialises a small JSON payload
(`{"op":"set","key":...,"value":...}`), frames it with its byte length and a
CRC-32 (IEEE) computed over `length ‖ payload`, and appends the frame to
`wal.log` through a single long-lived append handle (never reopened per
write). The record is on disk before the ack, so the tester can parse the
file while the process is still running and find exactly the expected frames.
The CRC covers `length` deliberately: a torn or flipped length field would
otherwise send a reader chasing a bogus payload size — stages 6 and 7 exploit
exactly that.

## replay — Recover from any log

> **Hint:** Read *exactly* `length` bytes per record (not a line, not a
> buffer), apply records in file order, and measure lengths in bytes, not
> characters. If those three are right, a log someone else wrote replays
> identically.

**How it works:** Recovery reads frame by frame from byte 0 — CRC, length,
then precisely `length` bytes — decodes the payload, and applies each op to
the map in order. Because the reader trusts the spec rather than its own
writer, the tester's hand-crafted log (overwrite, delete, multi-byte UTF-8, an
empty-string value) replays correctly: byte-length framing handles the unicode
value, and `""` is a real value so `found` is `true`. The property being built
here — *state is fully reconstructible from the log alone* — is what the next
two stages attack.

## torn-writes — Torn writes

> **Hint:** Never trust `length` before checking it fits in the remaining
> file. On the first record you can't fully validate (short header, length
> overruns the file, or bad CRC), stop, keep everything before it, and
> truncate the file to that point *before* accepting new writes.

**How it works:** The reader treats recovery as "longest valid prefix." It
walks records until one fails to validate — fewer than 8 header bytes left,
a `length` larger than the bytes remaining, or a CRC mismatch — then stops,
discarding that record and the tail. Crucially it then **truncates `wal.log`
to the last good offset** before the append handle takes new writes, so the
file always parses cleanly from byte 0. The bug this catches: recovering
correctly in memory but appending *after* the torn bytes, leaving garbage in
the middle that silently swallows everything after it on the next restart.
Truncate first, then append. And by write-before-ack, a torn record is one
that was never acknowledged — prefix recovery loses nothing a client was
promised.

## checksums — Detect corruption

> **Hint:** A flipped byte mid-log is the same event as a torn tail to a
> reader that trusts nothing it can't checksum: stop at the first invalid
> record and discard it *and everything after it* — even records whose own
> CRCs are fine.

**How it works:** This is the previous stage's rule applied without
exception. When record 3's CRC fails, the reference stops there and drops 3,
4, and 5, even though 4 and 5 checksum cleanly — because the log's meaning is
sequential, and once one record is unreadable you can no longer prove the
later ones mean what they appear to (Postgres's redo stops at the first
invalid record for the same reason). If you built stage 6 as "validate, stop
at first failure, truncate, then append," this stage often passes for free —
that shared code path is the sign of the right design.

## checkpoint — Snapshots and log truncation

> **Hint:** Snapshot, *then* truncate — never the reverse. Write the full
> state to a temp file, fsync, rename over the snapshot; only once it's
> durable, reset the log to empty. Recovery becomes snapshot + log replayed on
> top, log wins.

**How it works:** `checkpoint` writes the entire map to `snapshot.json` via
the atomic temp-file-plus-`rename` trick, and only after that is durable
resets `wal.log` to zero bytes (re-pointing the append handle at the fresh
file — not a deleted inode). Recovery loads the snapshot, then replays the
log over it, so overlapping keys take the log's value. The ordering is the
whole lesson: a crash *between* snapshot and truncate just replays
already-applied ops onto the new snapshot, which is harmless because `set`/
`del` are absolute (idempotent); truncating first would instead lose
everything not yet snapshotted. One ordering costs redundant work, the other
costs data.

## gauntlet — The gauntlet

> **Hint:** If served state and durable bytes ever disagree, this finds it.
> The usual culprits: a stale append handle after checkpoint, a lock gap
> between "append to log" and "update memory," a non-atomic snapshot, or a
> wrong `deleted` flag.

**How it works:** Nothing new — the seeded, randomized workload with repeated
crashes and mid-run checkpoints just exercises every earlier guarantee at
once, then reconstructs state directly from your `snapshot.json` + `wal.log`
and demands it match the model exactly. The reference passes because each
earlier choice holds under stress: the map mutation and the log append happen
under one lock (no torn interleaving), the append handle is refreshed after
every checkpoint, the snapshot is published atomically, and `deleted`
reflects prior existence. Served state and durable bytes are derived from the
same operations applied in the same order, so they cannot drift.
