# Design a workflow orchestration platform

Design **Temporal-style** workflow orchestration: developers write ordinary code that **survives process crashes** and can run for days or months.

Example workflow:

```python
def onboard_user(user_id):
    send_welcome_email(user_id)          # activity
    wait_for_verification(user_id, days=7)  # timer + signal
    provision_account(user_id)           # activity
```

If the worker crashes after the email sends, resume without sending twice.

## Functional requirements

1. **Start workflow** with input → returns `workflow_id`, `run_id`.
2. **Activities** — side-effecting tasks executed by workers (HTTP, DB, email).
3. **Timers** — sleep until duration or calendar time.
4. **Signals** — external events injected into a running workflow.
5. **Queries** — read-only snapshot of workflow variables (best effort).
6. **History** — full audit of what happened; replay must reproduce decisions.

## Non-functional

- **Durability**: no lost workflows across server restarts.
- **Determinism**: same history → same commands (critical).
- **Scale**: 100k concurrent workflows, 10k new starts/sec (burst).
- Multi-tenant namespaces with quotas.

## Your task

Whiteboard **45–55 minutes**:

1. Split **server** vs **worker SDK** responsibilities.
2. Event **history** schema — what events exist?
3. **Task delivery** — how does an activity get assigned to a worker?
4. **Timer firing** — who wakes the workflow at T+7d?
5. **Replay** — step through crash mid-activity recovery.

Explicitly call out what must **not** go in workflow code (random, `now()`, raw I/O).

## Stretch

- Versioning: deploy new workflow code while old runs continue.
- Child workflows and saga compensation.
