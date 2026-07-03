# Design a distributed job scheduler

You're the tech lead for a **platform job scheduler** — think internal Cron + Sidekiq + delayed tasks, but multi-tenant and crash-safe. Teams schedule:

- **One-shot delayed jobs** ("charge card in 7 days")
- **Recurring jobs** ("rollup analytics every hour")
- **High-volume fan-out** (millions of small tasks per day)

Workers poll for work, execute user code, and report success or failure.

## Functional requirements

1. **Schedule** a job to run at a future time (or immediately).
2. **Poll** — workers request the next runnable job (blocking or short poll).
3. **Lease** — a worker holds exclusive rights to a job for a TTL; if it doesn't complete or renew, the job becomes runnable again.
4. **Complete / fail** — terminal states; failures may **retry** with backoff.
5. **Cancel** a job that hasn't started (or is waiting).
6. **Recurring** — after success, reschedule per cron or fixed interval.
7. **Durability** — scheduled jobs survive scheduler process crashes and restarts.

## Non-functional requirements

| Dimension | Target |
|-----------|--------|
| Schedule API | 10k writes/sec (burst) |
| Poll API | 50k polls/sec across workers |
| Latency | p99 fire-time skew &lt; 5s from scheduled instant |
| Tenancy | thousands of logical queues / namespaces |
| Availability | 99.9% — brief blips OK, no lost jobs |

## Constraints

- Workers are **untrusted** — they can die, lie, or run twice.
- User job handlers may be **slow** (minutes) or **fast** (milliseconds).
- You **cannot** assume synchronized clocks across workers.
- Side effects (charging cards, sending email) must not double-apply without an explicit idempotency story.

## Your task

Spend **30–45 minutes** whiteboarding:

1. Core **data model** and storage choice(s).
2. **Hot paths** for schedule, poll/lease, complete, and crash recovery.
3. How **recurring** jobs avoid duplicate fires after restarts.
4. Where you'd use primitives you've built in open-crafters (queue, lock, WAL).

Draw boxes. Name APIs. Call out trade-offs. Then check hints only if stuck, and compare with the reference solution last.

## Stretch questions

- How do you support **priority** without starving low-priority tenants?
- What metrics and alerts would you ship on day one?
- How does this differ from a **workflow engine** (see the workflow platform design problem)?
