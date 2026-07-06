# Stage 1: Boot the harness

Parse `--port`, listen on `127.0.0.1:<port>`, and answer `ping` over
newline-delimited JSON.

```
→ {"id": "1", "method": "ping", "params": {}}
← {"id": "1", "result": {"message": "pong"}}
```

The tester opens **two concurrent connections** and pings on both.

## Notes

- Ignore `--data-dir` on your harness process if passed.
- One goroutine/thread per connection is fine.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Same newline-delimited JSON loop as every challenge: read line, decode, dispatch, respond, flush. `ping` returns `pong` — wire transport once, then add harness methods stage by stage.

Or run: <code>crafters hint harness --stage bind</code>
</details>
<!-- /crafters-stage-hint -->
