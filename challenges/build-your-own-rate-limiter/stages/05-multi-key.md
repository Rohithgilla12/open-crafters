# Stage 5: Independent keys

Real limiters are keyed — per user, per API token, per IP, per route. Each key
gets its own independent budget: one client exhausting its limit must not
affect another. This stage makes sure your limiters are truly isolated and
that reconfiguring or naming an unknown key behaves sanely.

## Your task

You already key limiters by `key`; now prove the edges:

1. Two keys with different algorithms coexist; draining one leaves the other
   untouched.
2. `configure` on an existing key **replaces** it and resets its consumption
   (a token bucket goes back to full, window counters clear).
3. `take` on a key that was never configured returns error `KEY_NOT_FOUND`.

```json
→ {"method":"take","params":{"key":"never-configured"}}
← {"error":{"code":"KEY_NOT_FOUND","message":"..."}}
```

## What the tester checks

- Exhausting key A still leaves key B fully available.
- Re-`configure`-ing a drained key restores its full budget.
- `take` on an unknown key → `KEY_NOT_FOUND`.

## Notes

- A map from `key` to limiter state is all you need. Keep the per-key state
  small — the next stages will hammer it concurrently and persist it.
