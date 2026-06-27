# Stage 4: Sliding window

Fixed windows have a notorious flaw: the **boundary burst**. With a limit of 3
per second, a client can fire 3 at `t = 0.999s` and 3 more at `t = 1.001s` —
6 requests in 2ms, because they straddle two windows. A sliding window closes
that gap by measuring the trailing `window_ms` from *now*, not from a fixed
grid.

## Your task

Implement `configure`/`take` for `algorithm: "sliding_window"`.

A `take` of `cost` is admitted iff the total cost admitted in the half-open
interval `(now − window_ms, now]` plus `cost` is `≤ limit`. Keep the
timestamps of admitted units (or equivalent) and count only those newer than
`now − window_ms`. When denied, `retry_after_ms` is the time until enough of
the oldest in-window cost ages past `window_ms`.

## What the tester checks

- The boundary burst that fixed window allows is **denied** here: after using
  the limit late in one notional window, the same volume early in the next is
  rejected.
- Once the earliest admissions age out of the trailing window, takes are
  allowed again — at most `limit` in any `window_ms`-wide span.

## Notes

- Trim entries older than `window_ms` as you go so state stays bounded.
- This is the same trailing-window idea behind sliding-window log and
  sliding-window counter limiters; you're building the exact (log) form.
