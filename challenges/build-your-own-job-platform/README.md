# Build your own job platform

**Meta-compose capstone** — you write the **orchestrator gateway** only. The harness spawns reference
scheduler, queue, and distributed-lock processes and injects their addresses
into your environment.

## What you build

A TCP gateway that exposes `submit_job`, `receive_work`, `complete_work`, and
`cancel_job` by coordinating the three child protocols documented in those
challenges.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | bind | Gateway `ping` + stack boots |
| 2 | submit | Schedule via scheduler |
| 3 | delayed | Fire-time delivery through queue |
| 4 | complete | Complete + ack path |
| 5 | empty | Idle `receive_work` |
| 6 | cancel | Cancel pending jobs |
| 7 | multi | Several jobs end-to-end |
| 8 | concurrent | Parallel submits |
| 9 | gauntlet | Mixed delay, complete, cancel |

```sh
crafters start job-platform
```

Prerequisites: pass scheduler, queue, and distributed-lock (or use reference binaries via the harness).
