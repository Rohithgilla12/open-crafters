# Stage 9: The gauntlet

Nothing new to implement — this stage interleaves everything and adds the one
dimension correctness tests miss: **throughput**. A rate limiter sits on the
hot path of every request your service handles, so an implementation that is
correct but slow (a global lock, an fsync per `take`, an unbounded sliding-log
scan) is a production outage waiting to happen.

## Your task

Survive a mixed workload:

1. Several keys with different algorithms, exercised together.
2. A `SIGKILL` + restart mid-run; limiters and their consumption survive.
3. A **throughput floor**: after the crash, the tester fires a few thousand
   `take` calls on one connection and requires them to complete within a
   generous time budget. This is a performance tier — the bar is lenient
   enough that any reasonable design clears it, but a pathological one
   (per-`take` fsync, O(n) rescans, lock convoy) will not.

## What the tester checks

- Per-key state stays correct across the crash (drained stays drained, other
  keys unaffected).
- The post-crash throughput run completes within the time budget.

## Notes

- The state you persist is small (a handful of keys) — rewriting it atomically
  on each `take` is fine and keeps you well above the throughput floor. The
  trap is fsync-per-take or serialising every key behind one lock.
- Trim sliding-window logs as you go; an ever-growing log turns `take` into an
  O(n) scan and fails the floor.
