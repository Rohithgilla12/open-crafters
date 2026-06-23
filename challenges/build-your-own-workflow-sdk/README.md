# Build your own workflow SDK

Build the **deterministic replay engine** behind Temporal workers — the half
of a workflow system that runs *inside* the worker process.

If you completed [Build your own Temporal](../build-your-own-temporal/), you
built the **server**: histories, task queues, timers, crash recovery. This
challenge builds the **SDK**: given an event history, emit the commands
workflow code would produce — pure, repeatable, and correct.

Your engine will:

- replay **event histories** and return **commands** (`COMPLETE_WORKFLOW`,
  `SCHEDULE_ACTIVITY`, `START_TIMER`, …),
- handle **waiting states** with empty command lists,
- react correctly to **activities**, **timers**, and **signals** in history,
- and stay **deterministic** — same history in, same commands out, every time.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + newline-delimited JSON, `ping` |
| 2 | [Replay to completion](stages/02-simple-complete.md) | `replay` + `greet` workflow |
| 3 | [Schedule an activity](stages/03-schedule-activity.md) | `fetch` workflow schedules work |
| 4 | [React to activity completion](stages/04-activity-result.md) | activity result → complete |
| 5 | [Waiting means empty commands](stages/05-waiting.md) | no commands while waiting |
| 6 | [Durable timers in replay](stages/06-timers.md) | `timer_wait` workflow |
| 7 | [Signals in replay](stages/07-signals.md) | `signal_wait` workflow |
| 8 | [Same history, same commands](stages/08-determinism.md) | pure replay engine |
| 9 | [The gauntlet](stages/09-gauntlet.md) | `pipeline`: activity → timer → complete |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) — it's the complete contract. Copy a starter
from [starters/](starters/) (Python, Go, and TypeScript available), then:

```sh
crafters start workflow-sdk
crafters test
```

**Prerequisite:** do [Build your own Temporal](../build-your-own-temporal/)
first if you haven't — you'll understand event histories and commands much
faster.

Stuck? Reference solutions live in
[examples/solutions/build-your-own-workflow-sdk/](../../examples/solutions/build-your-own-workflow-sdk/).
