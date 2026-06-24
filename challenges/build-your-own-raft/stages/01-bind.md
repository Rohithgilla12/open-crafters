# Stage 1: Boot the cluster

You're building a **3-node Raft cluster** that replicates a key-value store. Before
elections and replication, the standard open-crafters opener: get every node on the
air and talking over TCP.

## Your task

Each node is started as:

```
./your_program.sh --node-id <id> --peers <peer-list> --port <port> --data-dir <path>
```

Listen on `127.0.0.1:<port>` for **newline-delimited JSON**: one request object per
line, one response line echoing the request `id`.

Implement `ping`:

```
→ {"id": "1", "method": "ping", "params": {}}
← {"id": "1", "result": {"message": "pong", "node_id": "<your --node-id>"}}
```

The tester spawns **three nodes** (ids `1`, `2`, `3`) and pings each one. You can
ignore `--peers` and `--data-dir` for this stage.

## Tests

The tester dials every running node concurrently. Handle each connection
independently (threads, goroutines, or an async event loop).

## Notes

- Be accepting connections within 10 seconds of starting.
- Write each response with a trailing `\n` and flush — don't buffer.
