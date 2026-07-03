# Stage 1: Boot the server

Prove your program boots, binds a port, and speaks the wire protocol. No
transactions yet — just a heartbeat.

## Your task

Parse `--port` and `--data-dir`, listen on `127.0.0.1:<port>`, and answer the
`ping` method over newline-delimited JSON.

```
→ {"id": "1", "method": "ping", "params": {}}
← {"id": "1", "result": {"message": "pong"}}
```

## Tests

The tester opens two concurrent connections and pings on both. Accept
connections within 10 seconds.

## Notes

- `--data-dir` is yours for durable state; ignore it until the Durability
  stage.
- The starter templates already pass this stage — start from one.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Newline-delimited JSON server loop — read line, dispatch on `method`, respond, flush. `ping` returns `pong`. Build transport once; transactions come next.

Or run: <code>crafters hint mvcc --stage bind</code>
</details>
<!-- /crafters-stage-hint -->
