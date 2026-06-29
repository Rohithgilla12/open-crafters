# Stage 1: Boot the server

Every challenge starts the same way: prove your program boots, binds a port,
and speaks the wire protocol. No storage yet — just a heartbeat.

## Your task

Parse `--port` and `--data-dir` from the command line, listen on
`127.0.0.1:<port>`, and answer the `ping` method over newline-delimited JSON.

```
→ {"id": "1", "method": "ping", "params": {}}
← {"id": "1", "result": {"message": "pong"}}
```

Each request is one JSON object on its own line; each response echoes the
request's `id` and carries exactly one of `result` or `error`.

## Tests

The tester opens **two concurrent connections** and pings on both a few times.
Both must answer `{"message": "pong"}`. You must accept connections within 10
seconds of starting.

## Notes

- `--data-dir` is an existing directory you own for durable state. You can
  ignore it until the Durability stage.
- Handle each connection independently — the tester keeps several open at
  once. A thread (or goroutine, or async task) per connection is the simplest
  thing that works.
- The starter templates already pass this stage. Start from one.
