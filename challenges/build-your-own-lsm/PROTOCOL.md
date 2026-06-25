# Wire & File Protocol — Build your own LSM-tree

This document specifies what your program must implement: a key-value store
built on an **LSM-tree** — an in-memory memtable plus immutable on-disk
SSTables. The tester grades you two ways:

1. over TCP, as a client doing reads, writes, flushes, scans, and compactions
   (and killing your process),
2. on disk, by **parsing the SST files you write** — so the file format below
   is part of the contract, byte for byte.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — directory containing your durable state. The tester will
  `SIGKILL` your process and restart it with the same `--data-dir`.

Your server must accept connections within **10 seconds** and handle multiple
concurrent connections.

## Transport: newline-delimited JSON

Identical to the open-crafters convention: one JSON object per line.
Request `{"id": "...", "method": "...", "params": {...}}`; response echoes
`id` with exactly one of `result` or `error`
(`{"code": "...", "message": "..."}`). Unknown methods → error code
`UNKNOWN_METHOD`.

## Methods

### `ping`

- **params:** `{}`
- **result:** `{"message": "pong"}`

### `put`

- **params:** `{"key": "<string>", "value": "<string>"}`
- **result:** `{}`
- Writes go to the **memtable** (in memory). They are **not** durable until
  you `flush`.

### `get`

- **params:** `{"key": "<string>"}`
- **result:** `{"value": "<string>", "found": true}` when present,
  `{"value": null, "found": false}` when absent.
- Look up the memtable first, then SST files from newest to oldest.

### `del`

- **params:** `{"key": "<string>"}`
- **result:** `{"deleted": true}` if the key existed, `{"deleted": false}`
  otherwise.
- Records a tombstone in the memtable. The key is hidden immediately.

### `flush`

- **params:** `{}`
- **result:** `{}`
- **effects:** write the entire memtable to a new SST file in
  `<data-dir>/sst/`, sorted lexicographically by key, then clear the memtable.
- **durability:** only acknowledged after the SST file is fully written and
  fsynced. Flushed data must survive `SIGKILL`.

### `scan`

- **params:** `{"start": "<string>", "end": "<string>"}`
- **result:** `{"entries": [{"key": "<string>", "value": "<string>"}, ...]}`
- Return all live key/value pairs in the **half-open lexicographic range**
  `[start, end)` — `start` inclusive, `end` exclusive.
- Merge memtable and all SST files. For duplicate keys, the newest write
  wins. Tombstones hide keys. Entries in the result must be sorted by key
  ascending. Deleted keys must not appear.

### `compact`

- **params:** `{}`
- **result:** `{}`
- **effects:** merge **all** `.sst` files in `<data-dir>/sst/` into one new
  sorted SST file, then delete the old ones. For duplicate keys across files,
  the value from the **newer** file (higher sequence number) wins. Tombstones
  are preserved. The memtable is not flushed or modified by `compact`.

Keys and values are always strings in this challenge.

## The SSTable file format

SST files live at `<data-dir>/sst/NNNNNN.sst` where `NNNNNN` is a 6-digit
zero-padded sequence number starting at `000001`. Each flush (and compaction)
creates the next file in sequence.

```
magic       "SST1"              4 bytes
entry_count                     uint32 LE
repeat entry_count times:
  key_len                       uint32 LE
  key                           key_len bytes (UTF-8)
  value_len                     uint32 LE
  value                         value_len bytes (UTF-8)
```

Rules:

- `value_len = 0` means a **tombstone** (the key is deleted).
- Entries within a file must be sorted lexicographically by key.
- The tester parses your files byte-for-byte and fails on any deviation.

## Recovery

On startup:

1. Read all `.sst` files in `<data-dir>/sst/` in sequence order (sorted by
   filename).
2. Rebuild read visibility: for each key, the last occurrence across all
   files wins; tombstones remove keys.
3. Start with an empty memtable.

Only data in SST files survives a restart. Memtable contents are lost unless
flushed first.

## Compaction

`compact` merges every SST file into one sorted file with deduplicated keys
(newer wins), assigns it the next sequence number, and removes the inputs.
After compaction there must be exactly one SST file (plus whatever the
compaction output is — typically one merged file).

## What the tester does to your files

While your process is stopped, the tester may:

- **parse** your SST files and fail you on any format deviation,
- **kill and restart** your process to verify recovery from on-disk SSTables.

## A note on fsync

`SIGKILL` does not drop the OS page cache, so the tester physically *cannot*
catch a missing `fsync` on SST writes — only a real power cut can. Write your
implementation as if the power can fail after any syscall: fsync each SST
file before acknowledging `flush` or `compact`. That discipline is the actual
lesson; the tester checks everything it can observe.
