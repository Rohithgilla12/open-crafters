# Stage 3: Token bucket

The workhorse of real rate limiting (AWS, Stripe, NGINX). A bucket holds up to
`capacity` tokens and refills continuously. Each request spends tokens; when
the bucket runs dry, requests are denied until it refills. Unlike a fixed
window, it allows a controlled burst (up to `capacity`) and then settles to a
steady rate — and it has no window-boundary cliff.

## Your task

Implement `configure`/`take` for `algorithm: "token_bucket"`:

```json
→ {"method":"configure","params":{"key":"api","algorithm":"token_bucket","capacity":5,"refill_tokens":1,"refill_interval_ms":200}}
→ {"method":"take","params":{"key":"api","cost":2}}
← {"result":{"allowed":true,"remaining":3,"limit":5,"retry_after_ms":0}}
```

- The bucket starts **full** (`capacity` tokens).
- Refill is continuous: after `t` ms, `t / refill_interval_ms × refill_tokens`
  tokens have accrued, capped at `capacity`. Track fractional tokens
  internally; report `remaining` as the floor.
- A `take` of `cost` is admitted iff available tokens `≥ cost`; then remove
  `cost`.
- When denied, `retry_after_ms` is the time for the deficit (`cost − tokens`)
  to refill.

## What the tester checks

- Draining the full bucket: `capacity` worth of takes succeed, the next is
  denied with a `retry_after_ms` consistent with the refill rate.
- After waiting one refill interval, exactly one refill's worth is available.
- `cost > 1` consumes multiple tokens in one call.

## Notes

- Don't refill on a timer — compute accrued tokens lazily from the elapsed
  wall-clock time on each `take`/`peek`. That single trick is also what makes
  durability easy later.
