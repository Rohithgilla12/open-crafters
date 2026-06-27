# Stage 2: Fixed-window counter

The simplest rate limiter in production: count requests per fixed time window,
reject once the count hits the limit, reset when the clock ticks into the next
window. It is what most "N requests per minute" APIs reach for first.

## Your task

Implement `configure` and `take` for `algorithm: "fixed_window"`.

```json
→ {"id":"1","method":"configure","params":{"key":"api","algorithm":"fixed_window","limit":3,"window_ms":1000}}
← {"id":"1","result":{}}

→ {"id":"2","method":"take","params":{"key":"api"}}
← {"id":"2","result":{"allowed":true,"remaining":2,"limit":3,"retry_after_ms":0}}
```

Window for time `now` is `floor(now / window_ms)`. Each window admits up to
`limit` units; the count resets the instant the window index changes. A `take`
of `cost` (default 1) is admitted iff `count + cost ≤ limit`. When denied,
nothing is consumed and `retry_after_ms` is the time until the current window
ends.

## What the tester checks

- The first `limit` takes are allowed with `remaining` counting down to 0.
- The next take is denied with `allowed:false` and a positive `retry_after_ms`.
- After the window elapses, takes are allowed again.

## Notes

- Align windows to the epoch (`floor(now / window_ms)`), not to the time of the
  first request — the tester relies on the boundary being predictable.
- Keep the limiter keyed; you'll add more keys and algorithms next.
