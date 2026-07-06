# Stage 1: Boot the server

Parse `--port`, listen on `127.0.0.1:<port>`, and answer `ping` over
newline-delimited JSON.

```
→ {"id": "1", "method": "ping", "params": {}}
← {"id": "1", "result": {"message": "pong"}}
```

The tester opens **two concurrent connections** and pings on both.

## Notes

- In-memory only — ignore `--data-dir` if passed.
- One goroutine/thread per connection is fine.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Same newline-delimited JSON loop as every challenge: read line, decode, dispatch, respond, flush. `ping` returns `pong` — wire transport once, then add cache methods stage by stage.

Or run: <code>crafters hint distributed-cache --stage bind</code>
</details>
<!-- /crafters-stage-hint -->
