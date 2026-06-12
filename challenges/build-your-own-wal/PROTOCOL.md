# Wire & File Protocol — Build your own WAL

This document specifies what your program must implement: a key-value store
server whose durability comes from a **write-ahead log** in an exactly
specified on-disk format. The tester grades you two ways:

1. over TCP, as a client doing reads and writes (and killing your process),
2. on disk, by **parsing the log you write** and by **writing logs and
   snapshots for you to recover** — so the file format below is part of the
   contract, byte for byte.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — directory containing your durable state. The tester will
  `SIGKILL` your process and restart it with the same `--data-dir`; it will
  also truncate and corrupt the files in it while your process is stopped.

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

### `set`

- **params:** `{"key": "<string>", "value": "<string>"}`
- **result:** `{}`
- **durability:** from the Persistence stage onward, you must only
  acknowledge after the operation's log record is written to the WAL (see
  below). An acknowledged `set` must survive `SIGKILL`.

### `get`

- **params:** `{"key": "<string>"}`
- **result:** `{"value": "<string>", "found": true}` when present,
  `{"value": null, "found": false}` when absent.

### `del`

- **params:** `{"key": "<string>"}`
- **result:** `{"deleted": true}` if the key existed, `{"deleted": false}`
  otherwise.
- Same durability rule as `set`.

### `checkpoint` (from the Checkpoint stage)

- **params:** `{}`
- **result:** `{}`
- **effects:** atomically write the entire current state to the snapshot
  file, then reset the WAL to empty. See [Snapshots](#snapshots).

Keys and values are always strings in this challenge.

## The WAL file format

The log lives at `<data-dir>/wal.log`. It is a sequence of records with **no
file header** — byte 0 is the start of the first record.

Each record is:

| field | size | encoding |
|---|---|---|
| `crc` | 4 bytes | CRC-32 (IEEE 802.3, the standard `crc32` in zlib / Go `hash/crc32` / Python `zlib.crc32`), little-endian, computed over `length` (as its 4 little-endian bytes) followed by `payload` |
| `length` | 4 bytes | length of `payload` in bytes, little-endian unsigned |
| `payload` | `length` bytes | UTF-8 JSON, one operation |

Payloads:

```json
{"op": "set", "key": "fruit", "value": "apple"}
{"op": "del", "key": "fruit"}
```

Rules:

- Every acknowledged `set` and `del` appends **exactly one record**, in
  acknowledgement order. (Yes, even a `del` of a missing key.)
- The log is **append-only** during normal operation. Only recovery (below)
  and `checkpoint` may shrink it.
- Extra JSON fields in payloads are tolerated by the tester, but `op`, `key`,
  and `value` must be exactly as specified.

## Recovery

On startup, rebuild state by replaying `wal.log` from the beginning (on top
of the snapshot, once you have snapshots). A record is **invalid** if any of:

- fewer than 8 bytes remain in the file (torn header),
- `length` exceeds the bytes remaining after the header (torn payload),
- the stored `crc` does not match the computed one.

Recovery **must stop at the first invalid record** and discard it *and
everything after it* — even if later bytes look like valid records. Then
**truncate the log** back to the last valid record before accepting new
writes, so the file always parses cleanly from byte 0.

Why prefix semantics are correct: if you fsync-before-ack, a torn or corrupt
tail can only be a record that was never acknowledged, so dropping it loses
nothing a client was promised.

## Snapshots

From the Checkpoint stage onward, state may also live in
`<data-dir>/snapshot.json`:

```json
{"data": {"fruit": "apple", "color": "green"}}
```

- **Recovery order:** load `snapshot.json` if it exists, then replay
  `wal.log` on top of it (log wins for overlapping keys).
- **`checkpoint`** must be crash-ordered: make the new snapshot durable
  *first* (write to a temp file, fsync, rename), *then* reset `wal.log` to
  empty (0 bytes; an absent file is also accepted). If you crash between the
  two steps, replaying the old log onto the new snapshot must be harmless —
  it is, because `set` and `del` are absolute (replaying them is idempotent).

## What the tester does to your files

While your process is stopped, the tester may:

- **truncate** the tail of `wal.log` mid-record (simulating a torn write /
  power loss),
- **flip a byte** inside a record in the middle of the log (simulating media
  corruption) — recovery must keep only the records before it,
- **replace** `wal.log` and/or `snapshot.json` entirely with files it crafted
  (your recovery must work from any spec-conformant state, not just state
  your own code wrote),
- **parse** your files and fail you on any deviation from this format.

## A note on fsync

The honest fine print: `SIGKILL` does not drop the OS page cache, so the
tester physically *cannot* catch a missing `fsync` — only a real power cut
can. Write your implementation as if the power can fail after any syscall:
fsync the log before acknowledging, fsync the snapshot before renaming it.
That discipline is the actual lesson of this challenge; the tester checks
everything it can observe, and this part is on your honor.
