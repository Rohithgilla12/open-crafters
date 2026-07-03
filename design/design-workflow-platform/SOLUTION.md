# Reference architecture — workflow platform

## Components

```
Client ──start──▶ History service ──append──▶ Workflow history store (per run)
                        │
                        ├──▶ Matching / task queues
                        │         │
                        │         ▼
                        │    Activity workers
                        │
                        └──▶ Timer subsystem
```

## History store

Append-only log keyed by `(namespace, workflow_id, run_id, event_id)`.

Events include: `WorkflowExecutionStarted`, `ActivityTaskScheduled`, `ActivityTaskStarted`, `ActivityTaskCompleted`, `TimerStarted`, `TimerFired`, `WorkflowExecutionCompleted`, …

Storage: your **WAL** instincts — batch append, replicate for durability.

## Worker replay loop

```
history = fetch_all_events()
reset_local_state()
for event in history:
    sdk.apply(event)   # update internal futures, timers
# workflow code runs until next blocking call
commands = sdk.drain_commands()
send_commands_to_server(commands)
```

On `ActivityTaskScheduled`, SDK records a future; during replay it **does not** re-run the activity — it waits for `ActivityTaskCompleted` in history.

## Crash mid-activity

1. Worker had scheduled activity, sent to server, died before complete.
2. Server times out task → `ActivityTaskTimedOut` or retry schedule.
3. New worker replays history → sees scheduled → waits → timeout event → workflow catch block runs.

No double email if activity is **idempotent** or guarded by `activity_id`.

## Timer at scale

Shard timers by `fire_at` bucket (hour/minute). Scanner writes `TimerFired` events — same as **distributed scheduler** problem but events instead of job rows.

## Determinism rules

| Allowed in workflow | Must be activity |
|---------------------|------------------|
| `if` on activity result | HTTP calls |
| `sleep` via SDK timer API | `time.Now()` |
| Child workflow calls | Random UUID (use server-generated) |

## vs plain scheduler

| Scheduler | Workflow engine |
|-----------|-----------------|
| Fire-and-forget handler | Durable program state |
| Payload in, ack out | Event-sourced replay |
| Retry job | Replay + compensations |

## Build challenges mapping

| Piece | Challenge |
|-------|-----------|
| History append + match | `build-your-own-temporal` |
| Deterministic replay | `build-your-own-workflow-sdk` |
| Durable log | `build-your-own-wal` |

Whiteboard here, then implement — the challenges are a miniature of this diagram with NDJSON instead of gRPC.
