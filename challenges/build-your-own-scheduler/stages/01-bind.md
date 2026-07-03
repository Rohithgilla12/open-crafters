# Stage 1: Boot the server

You're building a **job scheduler** — delayed work, worker polling, leases.
First, get a server on the air.

## Your task

```
./your_program.sh --port <port> --data-dir <path>
```

Implement `ping`:

```
→ {"id": "1", "method": "ping", "params": {}}
← {"id": "1", "result": {"message": "pong"}}
```

## Tests

Two concurrent connections, interleaved pings.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Newline-delimited JSON server — read line, dispatch, respond, flush. `ping` returns `pong`. Scheduling logic comes in the next stage.

Or run: <code>crafters hint scheduler --stage bind</code>
</details>
<!-- /crafters-stage-hint -->
