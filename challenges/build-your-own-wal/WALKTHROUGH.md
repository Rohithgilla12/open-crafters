# Walkthrough — Build your own WAL

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass, to
compare your model). No code — the point is the design.

`crafters hint wal` prints just the hint for your next stage;
`crafters walkthrough wal --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** One read loop per connection: read a line, `json` decode, dispatch
> on `method`, write one response line and flush. `ping` is the only handler
> for now — get the envelope right and every later stage is a new `case`.

**How it works:** The reference keeps transport (accept, read line, write line)
separate from dispatch. Each connection runs independently so concurrent pings
interleave safely. `--data-dir` is parsed and ignored until persistence.

## kv — An in-memory key-value store

> **Hint:** A `map[string]string` behind a mutex is enough. `set` overwrites;
> `get` returns `found: false` for missing keys; `del` removes and reports
> whether the key existed.

**How it works:** All RPC handlers share one in-memory map guarded by a lock.
Nothing is durable yet — the lesson is purely the wire contract and concurrent
access. Deletes are explicit: absent keys return `found: false` on `get` and
`deleted: false` on `del`.

## persist — Survive a crash

> **Hint:** Append every mutating operation to a file under `--data-dir`
> *before* you acknowledge the RPC. On startup, replay that file into the map.
> `SIGKILL` means no shutdown hook — persist on each write.

**How it works:** The reference appends one JSON line per `set`/`del` to
`wal.log`, fsyncs, then returns success. Recovery reads the file line by line
and replays ops into a fresh map. The invariant is **write-before-ack**: if the
client got `ok`, the bytes are on disk.

## format — Write the log format

> **Hint:** Prefix each payload with a 4-byte little-endian length, and put a
> 4-byte CRC32 *before* that covering `length || payload`. The CRC lets you spot
> garbage without guessing where a record ends.

**How it works:** Records are `crc32 | len | json`. On append the reference
computes CRC over the length bytes plus body, writes the triple, fsyncs. On
replay it reads length, reads payload, verifies CRC — mismatch means stop
(truncated tail). This is the framing every later durability stage builds on.

## replay — Recover from any log

> **Hint:** Replay is "read records until EOF or first invalid frame, apply ops
> in order." A torn last record is simply incomplete — stop there, don't apply
> it, and truncate the file tail before accepting new appends.

**How it works:** Recovery loads `snapshot.json` if present, replays `wal.log`
record by record, and halts at the first CRC/length failure. The torn tail is
truncated so the next append continues from a clean boundary. Committed prefix
is never lost.

## torn-writes — Torn writes

> **Hint:** A crash can leave the last record half-written. Your length + CRC
> framing already tells you where validity ends — truncate back to the last good
> record and keep serving from there.

**How it works:** The tester crafts a log with a partial final record. The
reference replays until CRC fails, truncates `wal.log` to the last valid
offset, and resumes. No special "torn write detector" — correct framing +
truncate-on-recovery is the whole trick.

## checksums — Detect corruption

> **Hint:** When CRC fails mid-log, stop replay and truncate — same as torn
> writes, but the corruption can be anywhere. Never apply a record you can't
> verify.

**How it works:** The reference treats any CRC mismatch as an uncommitted
suffix: replay stops, tail is truncated, in-memory state reflects only verified
records. Corruption in the middle looks like an aborted transaction — everything
before it stands.

## checkpoint — Snapshots and log truncation

> **Hint:** A checkpoint is a full `snapshot.json` of the map, written
> atomically (temp file + rename), *then* truncate or reset `wal.log`. Order
> matters: snapshot first, log second.

**How it works:** `checkpoint` serialises the entire KV state to a temp file,
renames it over `snapshot.json`, then truncates the WAL. Recovery loads the
snapshot and only replays records appended after the checkpoint. Log size stays
bounded.

## gauntlet — The gauntlet

> **Hint:** You already have the pieces: framed append + fsync, snapshot +
> truncate, truncate-on-bad-CRC recovery. The gauntlet is random writes, kills,
> and restarts — nothing new, just don't skip fsync or reorder checkpoint steps.

**How it works:** The reference runs the same code paths under stress: append
with CRC framing, checkpoint when asked, recover after every restart by
snapshot + replay + tail truncate. Correctness comes from the same invariants
repeated under chaos.
