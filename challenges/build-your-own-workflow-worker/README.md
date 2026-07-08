# Build your own workflow worker

**Meta-compose capstone** — you write the **worker gateway** only. The harness spawns reference
Temporal and workflow-SDK processes and injects their addresses into your environment.

## What you build

A TCP gateway that exposes `run_workflow`, `start_workflow`, `await_workflow`, and
`signal_workflow` by polling Temporal, replaying histories through the SDK, and
completing workflow and activity tasks until executions finish.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | bind | Gateway `ping` + stack boots |
| 2 | greet | Run a simple workflow end-to-end |
| 3 | fetch | Activity stub completion |
| 4 | timer | Durable timer workflow |
| 5 | pipeline | Activity then timer |
| 6 | signal | Start, signal, await |
| 7 | duplicate | Reject duplicate workflow IDs |
| 8 | concurrent | Parallel workflows |
| 9 | gauntlet | Mixed workflow types |

```sh
crafters start workflow-worker
```

Prerequisites: pass temporal and workflow-sdk (or use reference binaries via the harness).
