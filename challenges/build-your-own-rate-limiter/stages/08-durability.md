# Stage 8: Survive a crash

A rate limiter that forgets its state on restart is a liability: crash-loop
your limiter and every client gets a free, full burst — exactly when your
system can least afford it. Configured limiters and their consumption must
survive `SIGKILL` + restart with the same `--data-dir`.

## Your task

Persist enough state that, after a crash and restart:

1. Limiters configured before the crash still exist with the same algorithm
   and parameters.
2. A bucket **drained to empty** just before the crash is **still empty** just
   after restart — the restart must not refill it for free.
3. Refill still works on **wall-clock** time: tokens that legitimately accrued
   during the downtime are available, but the bucket has not reset to full.

The tester configures a token bucket with a slow refill, drains it, `SIGKILL`s
your process, restarts it, and immediately takes — expecting a **denial**,
because almost no time has passed and the bucket was empty.

## What the tester checks

- After restart, `take` on the drained bucket is denied (state was not reset
  to full).
- The limiter's configuration is intact (a `peek` reports the original
  `limit`/`capacity`).

## Notes

- Because refill is computed from absolute time, you only need to persist the
  *token balance and the timestamp it was current as of* — not a running
  timer. Store `tokens` plus `as_of_ms`; recompute accrual from `now` on load.
- `SIGKILL` cannot be caught, so persist on each mutating call (an atomic
  temp-file rename is enough — the state is tiny). Don't rely on a shutdown
  hook.
