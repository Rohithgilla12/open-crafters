# Walkthrough — Build your own scheduler

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint scheduler` prints just the hint for your next stage;
`crafters walkthrough scheduler --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Newline-delimited JSON server — read line, dispatch, respond,
> flush. `ping` returns `pong`. Scheduling logic comes in the next stage.

**How it works:** Transport is separate from job storage. `--data-dir` is used
when jobs must survive crashes.

## schedule — Schedule a job

> **Hint:** `schedule` takes `run_at_ms` (epoch ms) and payload. Store the
> job as `pending` with a unique `job_id`. `poll` returns nothing until
> `now >= run_at_ms`.

**How it works:** The reference keeps jobs in a map keyed by id. Pending jobs
sit in a structure scannable by `run_at_ms`. Schedule returns immediately; work
becomes visible to workers only when due.

## complete — Complete a job

> **Hint:** `poll` hands out a due job with a `lease_token`. `complete` takes
> that token and marks the job `completed` with a result. Invalid or stale
> tokens are rejected.

**How it works:** Poll marks a job `running` and issues a lease token. Complete
validates token + job id, stores result, sets status `completed`. The job never
polls again.

## lease — Worker lease expiry

> **Hint:** Each poll grants a lease for `lease_ms`. If `complete` doesn't
> arrive before expiry, the job becomes pollable again — another worker can
> pick it up. Same payload, new token.

**How it works:** The reference stores `lease_expires_at_ms` on poll. A
background tick or lazy check on poll promotes expired `running` jobs back to
due/pending. At-least-once execution semantics.

## retry — Failed job retries

> **Hint:** `fail` with a lease token schedules a retry: increment `attempt`,
> set `run_at_ms = now + retry_delay_ms`, clear the lease. Stop after
> `max_attempts`.

**How it works:** Fail validates the token, records the error, and if attempts
remain, reschedules with backoff. Otherwise mark `failed` permanently. Poll
only returns jobs that are due and not exhausted.

## cancel — Cancel a job

> **Hint:** `cancel(job_id)` removes a pending job or stops a not-yet-completed
> scheduled job. Return whether it existed. Completed jobs can't be cancelled.

**How it works:** The reference deletes or marks cancelled before completion.
In-flight jobs may need lease invalidation. Poll skips cancelled/failed/completed.

## durability — Survive a crash

> **Hint:** Persist the full job table to `--data-dir` after every schedule,
> complete, fail, or cancel. On boot reload and resume — due jobs pollable,
> leases expired jobs back to pending.

**How it works:** Snapshot or append to disk with temp+rename. Recovery restores
all jobs and recomputes which are due. Leases from before the crash are void.

## recurring — Recurring jobs

> **Hint:** `schedule` with `interval_ms` reschedules itself after `complete`:
> next `run_at_ms = now + interval_ms`, same payload, new lease cycle. The job
> id may stay the same or spawn successors per your protocol.

**How it works:** The reference on successful complete checks `interval_ms`.
If set, reset status to pending with a future `run_at_ms` instead of marking
completed. Cron-like behavior without a separate cron daemon.

## gauntlet — The gauntlet

> **Hint:** Mix delayed jobs, leases, failures, retries, cancel, recurring,
> and crashes. Invariants: durable schedule, single-use lease tokens, at-least-
> once delivery until max attempts, time-based visibility.

**How it works:** The gauntlet stresses timing and recovery. The reference
uses wall-clock ms for scheduling, persists after mutations, and expires leases
on restart. Same poll/complete/fail paths throughout.
