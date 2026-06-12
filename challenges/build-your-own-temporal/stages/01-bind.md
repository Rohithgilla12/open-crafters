# Stage 1: Boot the server

Welcome! Over the next ten stages you'll build a **durable workflow engine** —
the server-side core of systems like [Temporal](https://temporal.io),
Cadence, and AWS Step Functions. By the end, your engine will orchestrate
workflows that run activities with retries, sleep on durable timers, react to
signals, and survive your process being `SIGKILL`ed mid-flight.

First: get a server on the air.

## Your task

Your program is started as:

```
./your_program.sh --port <port> --data-dir <path>
```

Listen for TCP connections on `127.0.0.1:<port>`. The protocol is
**newline-delimited JSON**: each request is a single-line JSON object, and you
reply with a single-line JSON object echoing the request `id`.

Implement the `ping` method:

```
→ {"id": "1", "method": "ping", "params": {}}
← {"id": "1", "result": {"message": "pong"}}
```

You can ignore `--data-dir` for now — it becomes important in stage 8.

## Tests

The tester opens **two concurrent connections** and interleaves `ping`
requests across them, so a single-connection-at-a-time server won't pass.
Handle each connection independently (threads, goroutines, or an event loop).

## Notes

- Your server must be accepting connections within 10 seconds of starting.
- Don't buffer responses: write the response line (with a trailing `\n`) and
  flush after every request.
