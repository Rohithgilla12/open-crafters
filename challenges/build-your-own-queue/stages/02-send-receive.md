# Stage 2: Send, receive, ack

A queue is a handoff between producers and consumers. A producer `send`s a
message; a consumer `receive`s it, does the work, then `ack`s it to say "done,
you can forget this." That three-step shape — **receive, work, ack** — is the
whole reason queues are reliable, and you'll build the rest of the challenge on
top of it. This stage is the happy path, in memory.

## Your task

Implement `send`, `receive`, and `ack` against an in-memory FIFO queue.

**`send`** — append a message, return a unique id.

```
→ {"id": "1", "method": "send", "params": {"queue": "jobs", "body": "resize-42"}}
← {"id": "1", "result": {"id": "a1b2c3"}}
```

**`receive`** — hand out the oldest visible message, or `null` if there is
none. The returned message becomes **in-flight**: it is hidden from later
`receive`s (until a much later stage brings it back), so two consumers never
get the same message.

```
← {"id": "2", "result": {"message": {"id": "a1b2c3", "body": "resize-42", "receipt": "r-9", "receives": 1}}}
← {"id": "3", "result": {"message": null}}        (nothing visible)
```

**`ack`** — remove an in-flight message for good, named by its `receipt`.

```
← {"id": "4", "result": {"acked": true}}      (it was in-flight)
← {"id": "5", "result": {"acked": false}}     (no such in-flight receipt)
```

## Tests

- Receiving from an empty queue returns `{"message": null}` — not an error.
- Messages come back **oldest first** (FIFO by send order), each with a unique
  `id`, a unique `receipt`, and `receives: 1`.
- A received message is in-flight: receiving again skips it and returns the
  *next* message; once all are in-flight, `receive` returns `null`.
- `ack` of an in-flight receipt returns `acked: true` and removes the message;
  acking the same receipt again, or an unknown one, returns `acked: false`.

## Notes

- A list (or deque) per queue, plus a map from `receipt` → message for the
  in-flight ones, is enough here.
- `id` and `receipt` are both server-assigned opaque strings. They are
  *different things*: an `id` names the message for its whole life; a `receipt`
  names one *delivery* of it (this matters a lot at the Fencing stage).
- If connections are handled on separate threads, guard your state with a lock
  now — the gauntlet will not forgive a data race.
