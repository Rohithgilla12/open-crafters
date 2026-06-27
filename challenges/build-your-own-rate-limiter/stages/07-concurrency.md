# Stage 7: Atomic admission

A rate limiter's entire job is to enforce a ceiling — so the one bug it must
never have is admitting *more* than the limit. Under concurrency that bug is
easy to write: read tokens, decide "allowed", then subtract, with a race in
between. Two requests both read "1 token left", both decide yes, and you've
admitted 2. This stage makes the check-and-consume **atomic**.

## Your task

Nothing new on the wire — just make `take` correct under concurrent
connections. The tester configures a token bucket of known `capacity` and
fires far more concurrent takes than that across many connections.

The invariant: across all connections, the number of `allowed:true` responses
for a freshly filled bucket of `capacity C` (no time for meaningful refill) is
**exactly C** — never more.

## What the tester checks

- With `capacity` C and a slow refill, many concurrent `cost:1` takes over
  several connections yield **exactly C** admissions — not C+1, not C+5.

## Notes

- The decision and the deduction must happen under one lock (or one atomic
  compare-and-set) per key. Decode/encode JSON outside the lock if you like,
  but "is there room?" and "take the room" cannot be separated.
- Per-key locking is enough; you don't need one global lock across all keys.
