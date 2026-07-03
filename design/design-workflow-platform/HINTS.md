# Hints — workflow platform

## Two binaries

- **Frontend / history service** — append-only event log per workflow, matchmaking tasks to workers.
- **Worker + SDK** — replays history into workflow code; emits **commands** (schedule activity, start timer).

## History is source of truth

Never ask the workflow "what state are you in?" — derive state by replaying events:

`WorkflowExecutionStarted → ActivityTaskScheduled → ActivityTaskCompleted → TimerStarted → …`

## Activities vs workflow code

Workflow code is **orchestration** (control flow). Activities are **side effects**. The SDK stubs activities during replay — only executes when history says the task completed.

## Timers

Server records `TimerStarted` with `fire_at`. A **timer queue** (or shard of the scheduler) pushes `TimerFired` into history when due — same machinery as delayed jobs.

## Task queues

`task_queue` name routes activity tasks to worker pools. Poll is identical in spirit to your **scheduler** challenge.

## open-crafters tie-in

You literally build both halves:

- `build-your-own-temporal` — server
- `build-your-own-workflow-sdk` — replay worker
