# Stage 5: Negative acknowledgement

A visibility timeout is the *implicit* "I failed" — the consumer just goes
quiet and waits out the clock. But often a consumer knows *immediately* that it
can't handle a message: a transient downstream error, a malformed body it wants
to retry later. Making it wait out a 30-second timeout to retry is wasteful.
`nack` is the explicit "I failed, put it back now."

## Your task

Implement `nack`: given a `queue` and a `receipt`, make the in-flight message
**visible again immediately** (no waiting on the timeout).

```
→ {"id": "1", "method": "nack", "params": {"queue": "jobs", "receipt": "r1"}}
← {"id": "1", "result": {"nacked": true}}     (it was in-flight under r1)
← {"id": "2", "result": {"nacked": false}}    (no such in-flight receipt)
```

A nacked message can be received again right away. Like a timeout expiry, a
nack ends that delivery — so the old receipt is dead afterwards.

## Tests

- `send` one message, `receive` it (with the long default timeout, so only a
  nack could bring it back), then `nack` it.
- It must be receivable **immediately** — `receives: 2`, fresh receipt — with
  no sleep.
- The nacked receipt is now stale: both `nack` and `ack` with it return
  `false`. Acking with the *new* receipt succeeds.

## Notes

- `nack` is "expire this delivery right now." If you built timeout expiry as a
  state transition (in-flight → visible), `nack` is the same transition fired
  by hand. Most of this stage is plumbing you already have.
- The nack itself isn't an extra delivery — `receives` counts deliveries
  (`receive`s), not failures. It went up to 1 when you received; the redelivery
  after the nack makes it 2.
