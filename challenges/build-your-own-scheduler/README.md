# Build your own scheduler

Build a **durable job scheduler** — the component behind cron, Sidekiq, Celery
Beat, and every "run this later" system.

Your scheduler will:

- **schedule** jobs after a delay or at an absolute time,
- let workers **poll** for due work (non-blocking),
- **lease** jobs so two workers never process the same run,
- **retry** failed jobs with a configurable policy,
- **cancel** pending work,
- **persist** fire times across crashes (absolute `run_at_ms`, not relative),
- and support **recurring** jobs that reschedule after success.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + `ping` |
| 2 | [Schedule a delayed job](stages/02-schedule.md) | `schedule`, `poll` |
| 3 | [Complete a job](stages/03-complete.md) | `complete`, `get_job` |
| 4 | [Job leases](stages/04-lease.md) | lease expiry + redelivery |
| 5 | [Retries](stages/05-retry.md) | `fail` + retry policy |
| 6 | [Cancel a job](stages/06-cancel.md) | `cancel` |
| 7 | [Survive a crash](stages/07-durability.md) | persist `run_at_ms` |
| 8 | [Recurring jobs](stages/08-recurring.md) | `interval_ms` |
| 9 | [The gauntlet](stages/09-gauntlet.md) | integration stress |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) — it's the complete contract. Copy a starter
from [starters/](starters/), then:

```sh
crafters start scheduler
crafters test
```

Stuck? Reference solutions live in
[examples/solutions/build-your-own-scheduler/](../../examples/solutions/build-your-own-scheduler/).
