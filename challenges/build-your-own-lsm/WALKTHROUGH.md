# Walkthrough — Build your own LSM-tree

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint lsm` prints just the hint for your next stage;
`crafters walkthrough lsm --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Newline-delimited JSON over TCP — read a line, dispatch on
> `method`, write a response line, flush. `ping` returns `pong`. Build the
> server loop once; each stage adds storage methods.

**How it works:** Transport and dispatch are separated. Concurrent connections
each get their own read loop. `--data-dir` is parsed early; the SST directory
is created when persistence begins.

## put-get — Put and get

> **Hint:** An in-memory `map` is your memtable. `put` upserts; `get` returns
> `found: false` for missing keys; `del` removes. No disk yet — learn the RPC
> contract first.

**How it works:** All reads and writes hit the memtable under a lock. Values are
strings; existence is explicit in the wire format. This is the logical model
every later layer (flush, scan, tombstones) preserves.

## flush — Flush memtable to disk

> **Hint:** Serialize the memtable to `<data-dir>/sst/NNNNNN.sst` using the
> SST1 format in PROTOCOL.md (magic, entries, footer). Clear the memtable after
> a successful write. File names sort lexicographically by sequence number.

**How it works:** The reference sorts keys, writes SST1 records (key, value
length, value), appends a footer with entry count, and names the file with a
zero-padded sequence. After flush the memtable is empty but data lives on disk.
`get` must search memtable first, then SSTs newest-to-oldest.

## restart — Recovery after restart

> **Hint:** On startup, list `sst/*.sst`, sort by filename, and remember the
> paths. You don't need to load all data into RAM — lazy read per `get` is
> fine. Bump `next_seq` from the highest file number.

**How it works:** Recovery rebuilds the SST file list from the directory.
Memtable starts empty. `get` checks memtable, then walks SST files from newest
to oldest until a key hits. Sequence counter continues from `max(seq)+1` so new
flushes never collide.

## scan — Range scan

> **Hint:** Merge memtable keys with all SST keys in `[start, end)`, dedupe
> keeping the newest version of each key, skip tombstones. Sort the result
> lexicographically.

**How it works:** The reference collects candidates from memtable + every SST
in range, resolves conflicts by memtable-over-SST and newer-SST-over-older-SST,
and omits deleted entries. Range is half-open `[start, end)` per the protocol.

## compaction — Compact SST files

> **Hint:** Read every SST, merge into one sorted run (newer values win on
> duplicate keys), write a single new SST, delete the old files. Same SST1
> format — you're just reducing read amplification.

**How it works:** Compaction is full merge: load all entries, sort by key,
keep the latest value per key (including tombstones), write one output SST with
the next sequence number, atomically update the file list, remove inputs.
Memtable is unchanged.

## delete — Tombstones

> **Hint:** `del` removes from memtable immediately. On flush, write tombstone
> entries (`value_len = 0`) so deletes survive restart and hide older values in
> SSTs during get/scan.

**How it works:** Memtable entries track `deleted: true`. Flush encodes
tombstones as zero-length values. `get` treats tombstone as absent; `scan` skips
them. Compaction retains tombstones so deleted keys don't resurrect from old
SST layers.

## durability — Survive a crash

> **Hint:** Flushed SSTs are the durable source of truth. After kill + restart,
> reload the SST index and verify memtable is empty (unflushed writes may be
> lost unless you also WAL the memtable — this challenge doesn't require that).

**How it works:** The tester flushes, kills, restarts, and checks data that
reached disk. The reference relies on SST files surviving process death.
Memtable-only state is ephemeral by design until flush.

## gauntlet — The gauntlet

> **Hint:** Interleave put/get/del/flush/scan/compact across restarts. The
> invariants: newest wins on conflicts, tombstones hide old values, filenames
> define age order, compaction preserves the merged view.

**How it works:** The gauntlet exercises the full pipeline under random ops and
crashes. The reference never breaks the LSM ordering rules: memtable first,
newer SSTs override older, tombstones propagate through flush and compaction.
