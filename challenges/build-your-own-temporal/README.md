# Build your own Temporal

Build a **durable workflow engine** from scratch — the server-side core of
[Temporal](https://temporal.io), Cadence, and AWS Step Functions.

Your engine will:

- dispatch **workflow tasks** to polling workers and apply the **commands**
  they return (the events-out/commands-in loop at the heart of Temporal),
- keep an **append-only event history** per workflow execution,
- run **activities** with server-side **retry policies** and exponential
  backoff,
- support **durable timers** (`sleep(30 days)` with no process waiting),
- deliver **signals** into running workflows,
- and **survive `SIGKILL`** at any point without losing a workflow, a timer,
  or a task.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + newline-delimited JSON, `ping` |
| 2 | [Start a workflow](stages/02-start-workflow.md) | `start_workflow`, `describe_workflow` |
| 3 | [Dispatch and complete a workflow task](stages/03-complete-workflow.md) | task queues, claims, `COMPLETE_WORKFLOW` |
| 4 | [Append-only event history](stages/04-history.md) | `get_history`, wake-up semantics |
| 5 | [Schedule and run activities](stages/05-activities.md) | `SCHEDULE_ACTIVITY`, activity tasks |
| 6 | [Activity retries with backoff](stages/06-retries.md) | retry policies, `fail_activity_task` |
| 7 | [Durable timers](stages/07-timers.md) | `START_TIMER`, `TIMER_FIRED` |
| 8 | [Survive a crash](stages/08-durability.md) | persistence, atomic writes, claim recovery |
| 9 | [Signals](stages/09-signals.md) | `signal_workflow` |
| 10 | [Concurrent workflows](stages/10-concurrency.md) | state isolation under interleaving |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) — it's the complete contract. Copy a starter
from [starters/](starters/) (Python and Go available), then:

```sh
./tester/tester --challenge build-your-own-temporal \
    --program path/to/your_program.sh --stage bind
```

Stuck? A reference solution lives in
[examples/solutions/build-your-own-temporal/python/](../../examples/solutions/build-your-own-temporal/python/)
— but it's worth struggling first; stage 4's wake-up semantics and stage 8's
claim recovery are where the real learning is.
