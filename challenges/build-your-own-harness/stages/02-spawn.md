# Stage 2: Spawn a program

Implement `spawn` — start a subprocess and return its listen address.

```
→ {"id": "1", "method": "spawn", "params": {"program": "/path/to/toy/your_program.sh"}}
← {"id": "1", "result": {"addr": "127.0.0.1:54321"}}
```

Launch the child as `<program> --port <free-port> --data-dir <temp-dir>`.
Wait up to **10 seconds** for TCP readiness.

The tester spawns the toy KV fixture and pings the returned `addr` directly.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** Reserve a free loopback port, create a temp `--data-dir`, start the child with both flags, poll TCP for up to 10s, return `{"addr": "127.0.0.1:port"}`. Track child PIDs so you can reap them later.

Or run: <code>crafters hint harness --stage spawn</code>
</details>
<!-- /crafters-stage-hint -->
