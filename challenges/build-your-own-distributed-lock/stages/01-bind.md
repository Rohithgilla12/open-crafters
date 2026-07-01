# Stage 1: Boot the server

Every challenge starts the same way: bind a port, speak the transport, answer
a ping. Get this passing and you have a working skeleton to build on.

## Your task

Parse `--port` and `--data-dir`, listen on `127.0.0.1:<port>`, and handle
newline-delimited JSON requests. Implement `ping`:

```json
→ {"id": "1", "method": "ping", "params": {}}
← {"id": "1", "result": {"message": "pong"}}
```

Respond to any unknown method with an error:

```json
← {"id": "2", "error": {"code": "UNKNOWN_METHOD", "message": "..."}}
```

## What the tester checks

- The server accepts connections within 10 seconds.
- Two concurrent connections each answer `ping` three times.

## Notes

- The starter already passes this stage — run `crafters test` to confirm, then
  start on stage 2.
- You won't need `--data-dir` until the Durability stage; accept it now.
