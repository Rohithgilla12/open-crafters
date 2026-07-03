# Stage 1: Boot the server

You're building the **worker-side half** of a workflow system — the replay
engine that turns event histories into commands. Before replay logic, the
standard open-crafters opener: get a server on the air.

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

You can ignore `--data-dir` for this challenge — replay is evaluated
statelessly per request.

## Tests

The tester opens **two concurrent connections** and interleaves pings, so
handle each connection independently (threads, goroutines, or an event loop).

## Notes

- Be accepting connections within 10 seconds of starting.
- Write each response with a trailing `\n` and flush — don't buffer.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Newline-delimited JSON — read line, dispatch, respond, flush. `ping` returns `pong`. The replay engine is pure logic you'll add to `handle_request`; no `--data-dir` needed.

Or run: <code>crafters hint workflow-sdk --stage bind</code>
</details>
<!-- /crafters-stage-hint -->
