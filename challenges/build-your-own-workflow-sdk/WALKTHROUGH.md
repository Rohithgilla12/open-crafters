# Walkthrough — Build your own workflow SDK

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint workflow-sdk` prints just the hint for your next stage;
`crafters walkthrough workflow-sdk --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Newline-delimited JSON — read line, dispatch, respond, flush.
> `ping` returns `pong`. The replay engine is pure logic you'll add to
> `handle_request`; no `--data-dir` needed.

**How it works:** Transport is a thin wrapper. Each stage adds methods to the
dispatch table. This challenge grades deterministic replay, not persistence.

## simple-complete — Replay to completion

> **Hint:** `replay` takes a workflow type and event history. Walk events in
> order; for `greet`, emit one `COMPLETE_WORKFLOW` command with the greeting.
> Same history in → same commands out.

**How it works:** The reference validates history (`event_id` starts at 1 and
increments), dispatches on `workflow_type`, and returns a command list. `greet`
is a single-shot completion with no activities.

## schedule-activity — Schedule an activity

> **Hint:** For `fetch` at the start, history has only `WORKFLOW_EXECUTION_STARTED`.
> Return `SCHEDULE_ACTIVITY` — you're at the "need to run side effect" point.
> Don't complete yet; the activity hasn't run.

**How it works:** Replay tracks how far the workflow has driven. With no
activity completion in history, the next command is schedule. Activity type and
input come from the workflow definition / first event.

## activity-result — React to activity completion

> **Hint:** When history ends with `ACTIVITY_TASK_COMPLETED`, the workflow
> should consume that result and emit `COMPLETE_WORKFLOW`. Replay simulates
> "what would the code do after the activity returns?"

**How it works:** The reference walks events, updates internal replay state, and
after seeing completion produces the final command. One activity → one
completion is the `fetch` pattern.

## waiting — Waiting means empty commands

> **Hint:** If the workflow is blocked on an activity or timer that hasn't
> fired yet, return **no commands** — an empty list. Waiting is not an error;
> it's "nothing to do until more history arrives."

**How it works:** Replay inspects the tail of history. If the last relevant
event is "scheduled but not completed," the engine returns `[]`. Workers poll
again after new events append.

## timers — Durable timers in replay

> **Hint:** `timer_wait` workflow: before `TIMER_FIRED` in history, return
> empty commands; after it fires, return `COMPLETE_WORKFLOW`. Timers are just
> events you haven't seen yet.

**How it works:** The reference treats `TIMER_STARTED` / `TIMER_FIRED` like
activity schedule/complete. Until the fire event exists, replay is waiting.
After fire, emit completion.

## signals — Signals in replay

> **Hint:** `signal_wait` blocks until `WORKFLOW_EXECUTION_SIGNALED` appears.
> Before the signal event → empty commands. After → complete with the signal
> payload.

**How it works:** Replay scans for the signal event in history. No signal yet
means blocked. Signal in history means the workflow unblocks and completes with
the input from the event attributes.

## determinism — Same history, same commands

> **Hint:** No `Math.random()`, no `Date.now()`, no reading files — replay is
> a pure function of history. Call `replay` twice with identical input; byte-
> identical command output.

**How it works:** The reference uses only history contents and workflow type to
decide commands. No wall clock, no randomness, no external I/O. State is
reconstructed by re-walking events each time.

## gauntlet — The gauntlet

> **Hint:** `pipeline` chains activities and timers. Walk the full history
> event by event, tracking what's done vs pending. Each replay step asks: "given
> everything so far, what's the next command?" — maybe schedule, maybe wait,
> maybe complete.

**How it works:** The gauntlet uses longer histories mixing activities, timers,
and completions. The reference's replay loop is one state machine driven only
by events seen so far — the same pattern as earlier stages, composed.
