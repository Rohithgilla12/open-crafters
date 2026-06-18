# Stage 6: Receipt fencing

This is the stage that separates a toy queue from a correct one.

Picture it: consumer **A** receives a message with a 30s timeout and starts a
slow job. The job overruns. The timeout expires, the message goes visible, and
consumer **B** receives it and starts working. Now A finally finishes and calls
`ack`. **Whose message does A's ack remove?**

If the answer is "the message" — by id, or by 'whatever is in-flight' — then
A's late ack deletes the message *B is still working on*. B will finish, ack,
get `acked: false`, and shrug. The message is gone but B's result was never
recorded as done. That is how queue-backed systems silently lose work, and it
is almost always this exact bug.

The fix is **fencing**: an `ack` (or `nack`) names a specific *delivery* via its
receipt, not the message. The instant a message is redelivered, every earlier
receipt for it is permanently void.

## Your task

You almost certainly already have this if you've been matching acks by the
*current* receipt — this stage exists to make you prove it, and to stop you if
you took a shortcut like "ack removes the message with this id."

- An `ack`/`nack` succeeds **only** if the receipt is the one from the message's
  current in-flight delivery.
- A receipt from a delivery that has since expired or been nacked is dead:
  `acked: false` / `nacked: false`, and the message is **left untouched**.

## Tests

The tester runs the scenario above: A receives with a short timeout and stalls;
after expiry, B receives the redelivered message under a new receipt; then A
acks with its **stale** receipt.

- A's stale ack must return `acked: false`.
- B's ack with the **current** receipt must then return `acked: true` — proving
  A's ack did **not** remove B's in-flight message. (If it had, B's ack would
  return `false` and the test fails.)

## Notes

- Concretely: store the current receipt on the in-flight message and compare.
  When a message goes visible (timeout or nack), clear/rotate that receipt so
  the old one can never match again.
- Don't index in-flight state by message id alone — two different deliveries of
  one message must be distinguishable, and only the latest is live.
