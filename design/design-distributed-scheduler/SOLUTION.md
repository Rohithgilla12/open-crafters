# Reference architecture — distributed job scheduler

Spoiler: compare after your own attempt.

## High-level diagram

```
                    ┌─────────────────┐
  Teams ──schedule──▶│  Scheduler API  │
                    └────────┬────────┘
                             │ txn write
                    ┌────────▼────────┐
                    │  Job store      │
                    │  (SQL or queue) │
                    └────────┬────────┘
                             │ poll/lease
              ┌──────────────┼──────────────┐
              ▼              ▼              ▼
         Worker pool    Worker pool    Worker pool
              │              │              │
              └──────────────┴──────────────┘
                        complete/fail
```

## Storage options

| Approach | Pros | Cons |
|----------|------|------|
| **PostgreSQL** + `SKIP LOCKED` | Simple leases, strong durability | Poll QPS ceiling ~ tens of k |
| **Durable queue** (SQS-style) + index DB | Decouples poll from schedule | Two systems to reconcile |
| **Time-wheel in memory** + WAL | Very fast fire | Complex recovery |

For the stated scale, **Postgres (or Cockroach) with partial index on `run_at`** is a solid default. Shard by `tenant_id` when single-node poll becomes hot.

## Schedule path

1. Validate payload + `run_at` / cron.
2. Insert row: `status=scheduled`, `run_at`, `payload`, `idempotency_key`.
3. Return `job_id` immediately — ack only after commit (your **WAL** instinct).

## Poll / lease path

1. `BEGIN`
2. Pick oldest `scheduled` row where `run_at <= now()` with `FOR UPDATE SKIP LOCKED`.
3. Flip to `leased`, set `lease_expires = now() + TTL`, `attempt += 1`.
4. `COMMIT` → return job to worker.

Renew: extend `lease_expires` if worker still healthy.

## Complete / fail

- **Complete**: `status=completed`; if recurring, insert next `scheduled` row in same transaction.
- **Fail**: if `attempt < max_retries`, set `scheduled` with `run_at = now() + backoff`; else `dead`.

## Crash recovery

| Failure | Behavior |
|---------|----------|
| Worker dies | Lease expires → job returns to `scheduled` |
| Scheduler dies | Rows already committed — new leader continues |
| Duplicate complete | Idempotency on `(job_id, attempt)` |

Optional **leader election** (your **Raft** or a lock) for a singleton component that advances recurring jobs — only needed if you split "API" from "timer wheel."

## Why not a workflow engine?

Schedulers run **stateless handlers** with retry. Workflow engines add **durable program state** and deterministic replay — overkill for "send email at 3pm."

## Build challenges mapping

| Real system piece | open-crafters primitive |
|-------------------|-------------------------|
| Durable fire queue | `build-your-own-scheduler` |
| At-least-once buffer | `build-your-own-queue` |
| Singleton cron leader | `build-your-own-distributed-lock` |

After whiteboarding here, implementing the scheduler challenge is translating this diagram into NDJSON RPCs.
