# Stage 1: Boot the server

Over the next nine stages you'll build an **LSM-tree key-value store** — the
storage engine pattern behind RocksDB, LevelDB, and Cassandra. Writes land in
an in-memory memtable, get flushed to immutable SST files on disk, and get
merged by compaction.

First, the standard open-crafters opener: get a server on the air.

## Your task

Your program is started as:

```
./your_program.sh --port <port> --data-dir <path>
```

Listen on `127.0.0.1:<port>` for **newline-delimited JSON**: one request
object per line, one response line echoing the request `id`.

Implement `ping`:

```
→ {"id": "1", "method": "ping", "params": {}}
← {"id": "1", "result": {"message": "pong"}}
```

Ignore `--data-dir` for now — from stage 3 on, it's where your SST files
live.

## Tests

The tester opens **two concurrent connections** and interleaves pings, so
handle each connection independently (threads, goroutines, or an event loop).

## Notes

- Be accepting connections within 10 seconds of starting.
- Write each response with a trailing `\n` and flush — don't buffer.
