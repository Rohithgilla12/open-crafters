# Stage 6: Peek without consuming

Dashboards, `X-RateLimit-Remaining` headers, and admission planners all need to
*read* a limiter without spending from it. `peek` answers "how much is left,
and when could I retry?" without admitting anything — which forces you to
separate the *decision* from the *side effect*.

## Your task

Implement `peek`:

```json
→ {"method":"peek","params":{"key":"api","cost":1}}
← {"result":{"remaining":0,"limit":5,"retry_after_ms":140}}
```

- `remaining` — units available right now (token bucket: floored tokens;
  windows: `limit` minus current in-window cost).
- `retry_after_ms` — `0` if a `take` of `cost` would be admitted right now,
  else a lower bound on the wait until it would be.
- `peek` must **not** consume. Two `peek`s with no `take` between them return
  the same `remaining` (modulo refill).

## What the tester checks

- `peek` repeated does not change `remaining`.
- After draining a token bucket, `peek` reports `remaining:0` and a
  `retry_after_ms` that, once waited out, makes the next `take` succeed.
- A denied `take`'s `retry_after_ms` is a usable lower bound: sleeping that
  long and retrying succeeds.

## Notes

- Factor the "how many tokens / how much window cost is available at time
  `now`" computation into one place, then have `take`, `peek`, and
  `retry_after_ms` all call it. Duplicating that math is how the three drift
  out of agreement.
