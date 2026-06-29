# Walkthrough — Build your own Temporal

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint temporal` prints just the hint for your next stage;
`crafters walkthrough temporal --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Newline-delimited JSON over TCP — read line, dispatch, respond,
> flush. `ping` returns `pong`. You'll add workflow APIs stage by stage.

**How it works:** Transport is isolated from engine state. Each connection gets
its own handler. `--data-dir` is for crash durability later.

## start-workflow — Start a workflow

> **Hint:** `start_workflow` creates a workflow execution with a unique
> `workflow_id`, appends a `WORKFLOW_EXECUTION_STARTED` event to its history,
> and returns a `run_id`. Histories are append-only from day one.

**How it works:** The reference stores workflows keyed by `workflow_id`, each
with an event list and status. The first event records type and input.
`run_id` distinguishes retries/replays of the same logical start.

## complete-workflow — Dispatch and complete a workflow task

> **Hint:** After start, a workflow task appears on the task queue. Workers
> `poll_workflow_task`, get a token + history, then `complete_workflow_task`
> with commands (e.g. `COMPLETE_WORKFLOW`). Tasks are consumed once.

**How it works:** The engine enqueues a workflow task when new work exists.
Poll returns the task token and current history slice. Complete validates the
token, appends resulting events, and may enqueue follow-up tasks.

## history — Append-only event history

> **Hint:** Every state change is an event with monotonic `event_id`. Never
> mutate or delete past events — `get_history` returns the full ordered list.
> Commands from workers become new events.

**How it works:** The reference assigns `event_id` sequentially per workflow.
`get_history` is a straight dump. Replay correctness later depends on this
append-only discipline.

## activities — Schedule and run activities

> **Hint:** Workers send `SCHEDULE_ACTIVITY` in `complete_workflow_task`.
> That appends an event and enqueues an activity task. Activity workers
> `poll_activity_task`, run work, then `complete_activity_task` with the result.

**How it works:** Activity scheduling is event-sourced: schedule → activity
task → completion event. Workflow tasks reappear after activity completion so
the workflow can continue. Two task queues: workflow tasks and activity tasks.

## retries — Activity retries with backoff

> **Hint:** On `fail_activity_task`, reschedule with incremented `attempt` and
> a later `scheduled_time` (backoff). Cap at `maximum_attempts`; then append
> failure and let the workflow decide. Don't lose the activity — re-enqueue it.

**How it works:** The reference tracks per-activity attempt count. Failure
appends `ACTIVITY_TASK_FAILED` (or schedules retry event) and creates a new
activity task with delay. Success appends `ACTIVITY_TASK_COMPLETED`.

## timers — Durable timers

> **Hint:** `START_TIMER` command appends a timer event with `fire_at`.
> A background tick or heap wakes timers; when due, append `TIMER_FIRED` and
> enqueue a workflow task. Timers must survive if you kill the process after
> scheduling.

**How it works:** The reference stores pending timers with wall-clock deadlines
(in ms). A ticker goroutine scans for due timers, appends events, and notifies
workflows. Persistence makes timers durable across restarts.

## durability — Survive a crash

> **Hint:** Snapshot the entire engine state to `--data-dir` after every
> mutation (temp file + rename). On boot reload workflows, histories, task
> queues, and pending timers. `SIGKILL` has no shutdown hook.

**How it works:** The reference serialises all workflows and queue state to
JSON, fsyncs via atomic rename. Recovery reconstructs in-memory structures and
resumes timer processing. No event may be lost after ack.

## signals — Signals

> **Hint:** `signal_workflow` appends `WORKFLOW_EXECUTION_SIGNALED` with the
> signal name and payload, then enqueues a workflow task so the worker can
> react. Signals can arrive while a workflow is running or waiting.

**How it works:** Signals are external events in the history. The reference
validates `workflow_id`, appends the signal event, and schedules workflow work.
Workers see signals when they poll and replay history.

## concurrency — Concurrent workflows

> **Hint:** Many `workflow_id`s in flight at once — isolate state per workflow.
> Task tokens must be single-use and scoped to one workflow/run. Poll fairly
> or FIFO; never mix histories between workflows.

**How it works:** The reference keys everything by `workflow_id`. Tokens encode
which task they belong to. Concurrent starts, activities, and timers on
different workflows don't share state. One mutex guards the engine; correctness
comes from per-workflow isolation.
