# Build your own notification platform

**Meta-compose capstone** — you write the **gateway** only. The harness spawns reference
queue, scheduler, and rate-limiter processes and injects their addresses
into your environment.

## What you build

A TCP gateway that exposes `configure_limit`, `notify`, `schedule_notify`,
`receive`, and `ack` by coordinating the three child protocols documented in
those challenges.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | bind | Gateway `ping` + stack boots |
| 2 | configure | Per-user rate limit |
| 3 | notify | Immediate enqueue when allowed |
| 4 | receive | Delivery + ack |
| 5 | rate-limit | Denied notifies stay off the queue |
| 6 | schedule | Delayed digest delivery |
| 7 | multi | Several notifications |
| 8 | concurrent | Parallel notifies |
| 9 | gauntlet | Mixed immediate, delayed, and limits |

```sh
crafters start notification-platform
```

Prerequisites: pass queue, scheduler, and rate-limiter (or use reference binaries via the harness).
