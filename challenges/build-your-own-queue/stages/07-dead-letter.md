# Stage 7: Dead-letter queues

At-least-once delivery has a dark side. A **poison message** — one that makes
every consumer fail, because the body is malformed or triggers a bug — is
redelivered forever. Worse, recall the ordering rule: a redelivered message
keeps its original sequence number, so it sorts *ahead* of everything sent
after it. One poison message parks itself at the head of the queue and blocks
every message behind it, retried into eternity. This is **head-of-line
blocking**, and dead-letter queues are the standard cure.

## Your task

Implement `configure`, and the dead-letter behavior it turns on.

```
→ {"id":"1","method":"configure","params":{"queue":"jobs","max_receives":5,"dead_letter_queue":"jobs-dead"}}
← {"id":"1","result":{}}
```

Once a queue has a policy, a message that has been delivered `max_receives`
times and **fails again** (its visibility timeout expires, or it is `nack`ed) is
**moved to the dead-letter queue** instead of becoming visible again. In the
DLQ it is an ordinary, visible message — fresh `receives` count of `0`, sorted
after whatever is already there.

With `max_receives = 2`: the message is delivered as `receives` 1, then 2, and
the *next* failure (which would be delivery 3) dead-letters it instead. It
leaves the source queue at most `max_receives` times.

A queue with **no** policy redelivers forever — never drop a message you
weren't told to.

## Tests

- `configure` a queue with `max_receives: 2` and a DLQ. Send a `poison` (first,
  so it blocks the head) and a `good` message.
- Fail `poison` twice (the tester uses `nack` to keep it deterministic). On the
  second failure it must move to the DLQ.
- The source queue must now deliver `good` — proof the head is unblocked.
- The DLQ must now contain `poison`, as a fresh delivery (`receives: 1` when you
  receive it there).
- A separate, unconfigured queue must keep redelivering its message no matter
  how many times it's nacked.

## Notes

- The dead-letter decision happens at the moment a delivery *fails* (timeout
  sweep or nack), not when the message is received. The check is "have we
  already delivered this `max_receives` times? then this failure is terminal."
- "Move to the DLQ" = remove from this queue + insert into the DLQ as a new
  visible message. Since `send`/`ack` are your durable events, treat the move
  as both: it must survive a crash like any other.
- The DLQ is just another queue. You can receive from it, ack it, even give
  *it* a dead-letter policy. Don't special-case it.
