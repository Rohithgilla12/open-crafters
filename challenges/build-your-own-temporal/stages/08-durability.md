# Stage 8: Survive a crash

The "durable" in durable execution. A workflow engine that loses workflows
when it restarts is a to-do list. In this stage the tester gets violent: it
**`SIGKILL`s your server** — no shutdown hook, no warning — restarts it with
the same `--data-dir`, and expects nothing of value to be lost.

## Your task

Persist your engine's state to `--data-dir` so that after a kill + restart:

1. **Workflows survive**: ids, run_ids, statuses, results, and complete
   histories are intact.
2. **Pending timers still fire** — at the right wall-clock time. A timer that
   was due to fire during the downtime fires promptly after restart. (This is
   why you stored fire-at timestamps in stage 7.)
3. **Unfinished tasks are re-delivered**: a workflow or activity task that was
   *claimed but not completed* when the process died must become available
   again. Task tokens are in-memory things; claims die with the process,
   tasks don't.

## Tests

The tester kills your server three times: while a timer is pending, while a
workflow task is claimed but incomplete, and after the workflow completed.
Each time, the state visible after restart must be exactly right.

## Notes

- A full-snapshot JSON file, rewritten **atomically** on every state change
  (write to a temp file, then rename over the old one) is entirely sufficient
  here — and is itself a lesson: atomic rename is the simplest durable-write
  primitive. A write-ahead log is the "real" answer, but save it for the WAL
  challenge.
- Be deliberate about what you persist. Histories, statuses, pending
  activities (with `available_at`), timers (fire-at), and "needs a workflow
  task" — yes. Claims and task tokens — no: dropping them on restart is what
  makes re-delivery work, provided a claimed-but-incomplete task is persisted
  as "still needs doing".
- `SIGKILL` means your process gets no chance to flush. Persist *before*
  acknowledging a request, not after.
