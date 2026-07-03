# Hints — distributed job scheduler

Open only if you're stuck.

## Data model nudge

Think in terms of **Job** (immutable intent), **Attempt** (one execution try), and **Schedule** (when the next attempt is due). A job can have many attempts over time.

## Storage

The scheduler's brain is a **durable priority queue** keyed by `(run_at, job_id)`. Poll is `SELECT … WHERE run_at <= now ORDER BY run_at LIMIT 1 FOR UPDATE SKIP LOCKED` (or equivalent).

## Leases

A lease is a row: `(job_id, worker_id, expires_at)`. Poll atomically: mark leased + return payload. No separate distributed lock service required if the DB transaction is your lock.

## Exactly-once side effects

The scheduler gives **at-least-once delivery**. Exactly-once *effects* need **idempotency keys** in user handlers (store `job_id` + `attempt` in a dedup table before side effect).

## Recurring

Don't compute infinite future rows. Store `cron_expr` + `last_run_at`; on success, compute `next_run_at` and insert **one** new runnable row.

## open-crafters tie-in

You've implemented the hard parts in miniature:

- **Queue** — visibility timeout ≈ lease
- **Scheduler challenge** — fire times + durability file
- **Distributed lock** — optional for singleton cron leader
