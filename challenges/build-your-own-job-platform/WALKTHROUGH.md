# Walkthrough — Build your own job platform

Meta-compose gateway — orchestrate scheduler, queue, and lock via env addresses.

## bind — Boot the stack

> **Hint:** Read `SCHEDULER_ADDR`, `QUEUE_ADDR`, `LOCK_ADDR` from the environment. Your gateway only needs `ping` for this stage; the harness pings the references directly.

## submit — Schedule a job

> **Hint:** `submit_job` forwards `payload` and `delay_ms` to scheduler `schedule`; return `job_id`.

## delayed — Delayed delivery

> **Hint:** Under lock `dispatcher`, poll scheduler until `job: null`, `send` each to queue `jobs` as JSON body, then `receive` from queue in `receive_work`.

## complete — Complete work

> **Hint:** `complete_work` calls scheduler `complete` with `lease_token` then queue `ack` with `receipt`.

## empty — Nothing to do

> **Hint:** When scheduler has nothing due and queue is empty, return `{"work": null}` without blocking.

## cancel — Cancel pending

> **Hint:** Proxy `cancel` to scheduler; cancelled jobs must never appear in `receive_work`.

## multi — Several jobs

> **Hint:** Dispatch loop must drain all due jobs into queue `jobs`; workers receive in send order.

## concurrent — Parallel submits

> **Hint:** Scheduler generates unique `job_id`s; your gateway can be stateless aside from the dispatch lock.

## gauntlet — The gauntlet

> **Hint:** Immediate jobs beat delayed ones in the queue; reuse submit/receive/complete/cancel paths.
