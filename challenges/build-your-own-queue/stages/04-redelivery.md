# Stage 4: Visibility timeouts

So far a received message is hidden *forever* until it's acked. But consumers
crash, hang, and get OOM-killed mid-job. If a message vanished the moment it
was handed out, a dead consumer would take the message to the grave. The fix is
the **visibility timeout**: a received message is hidden only for a while; if
the consumer doesn't ack in time, the message reappears for someone else.

This is what makes delivery *at-least-once* during normal operation, not just
across crashes.

## Your task

Give `receive` an optional `visibility_timeout_ms` (default **30000**). The
returned message is hidden for that long. If it is not `ack`ed within the
window, it becomes visible again, and the next `receive` redelivers it:

- with `receives` incremented (2 on the first redelivery, 3 on the next…),
- carrying a **new** `receipt`,
- as the **same** message (same `id`, same `body`).

```
→ {"id":"1","method":"receive","params":{"queue":"jobs","visibility_timeout_ms":500}}
← {"id":"1","result":{"message":{"id":"m1","body":"...","receipt":"r1","receives":1}}}
   ... 500ms pass with no ack ...
← {"id":"2","result":{"message":{"id":"m1","body":"...","receipt":"r2","receives":2}}}
```

Once the message is redelivered, the **old receipt is dead**: an `ack` with
`r1` must return `acked: false` and must not remove the message.

## Tests

The tester receives a message with a 500ms timeout, then:

- polls during the window and asserts it is **not** redelivered early,
- asserts it **is** redelivered after the timeout (with generous slack),
  with `receives: 2` and a fresh receipt,
- asserts the first (expired) receipt no longer acks it, but the new one does.

## Notes

- You don't need a background timer. Compute visibility lazily: stamp each
  in-flight message with `invisible_until = now + timeout`, and when `receive`
  runs, first sweep the queue turning any in-flight message whose deadline has
  passed back into a visible one. A monotonic clock is the right clock here.
- "Hidden until acked" was just a visibility timeout of infinity. You are
  generalizing the previous stage, not rewriting it.
- Redelivery and ordering interact: a redelivered message keeps its original
  sequence number, so it sorts ahead of newer messages. Hold onto that — the
  Dead-letter stage turns it into a problem.
