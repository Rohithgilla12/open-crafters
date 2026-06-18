# Wire Protocol — Build your own message queue

This document specifies what your program must implement: a durable message
broker with **at-least-once delivery**. The tester grades you entirely over
TCP, as a producer and a set of consumers — and by `SIGKILL`ing your process
to check that acknowledged work, and only acknowledged work, survives.

Unlike the WAL challenge, the on-disk format here is **not** graded: persist
however you like inside `--data-dir`. The whole contract is *behavioral* —
what a client observes across receives, acks, timeouts, and crashes.

## Process contract

```
./your_program.sh --port <port> --data-dir <path>
```

- `--port` — TCP port to listen on (`127.0.0.1`).
- `--data-dir` — directory for your durable state. The tester will `SIGKILL`
  your process and restart it with the same `--data-dir`.

Your server must accept connections within **10 seconds** and handle multiple
concurrent connections.

## Transport: newline-delimited JSON

One JSON object per line. Request
`{"id": "...", "method": "...", "params": {...}}`; response echoes `id` with
exactly one of `result` or `error` (`{"code": "...", "message": "..."}`).
Unknown methods → error code `UNKNOWN_METHOD`.

Queues are created lazily: the first `send` (or `configure`) naming a queue
creates it. Queue names are arbitrary strings.

## The delivery model

A message moves through three observable conditions inside a queue:

- **visible** — eligible to be returned by `receive`.
- **in-flight** — handed to a consumer by `receive`, hidden from other
  `receive`s until either it is `ack`ed (removed for good) or its **visibility
  timeout** expires (it becomes visible again).
- **gone** — `ack`ed, or moved to a dead-letter queue.

Delivery is **at-least-once**: a message is redelivered until a consumer
acknowledges it. A crash, an expired visibility timeout, or a `nack` all cause
redelivery.

**Ordering.** `receive` always returns the visible message with the **smallest
sequence number** — i.e. the oldest by send order. A redelivered message keeps
its original sequence number, so it sorts *ahead* of messages sent after it.
(This is why one poison message can block a queue — and why dead-letter queues
exist; see the DLQ stage.)

## Methods

### `ping`
- **params:** `{}` → **result:** `{"message": "pong"}`

### `send`
- **params:** `{"queue": "<name>", "body": "<string>"}`
- **result:** `{"id": "<message-id>"}` — a server-assigned unique string id.
- **durability:** from the Durability stage on, only acknowledge the `send`
  (return the id) after the message is durably stored. An acknowledged `send`
  must survive `SIGKILL`.

### `receive`
- **params:** `{"queue": "<name>", "visibility_timeout_ms": <int>}`
  — `visibility_timeout_ms` is optional; default **30000**.
- **result:** the next visible message, now in-flight:
  ```json
  {"message": {"id": "...", "body": "...", "receipt": "...", "receives": 1}}
  ```
  or `{"message": null}` when the queue has no visible message.
- `receive` is **non-blocking**: return `null` immediately rather than waiting.
  Consumers poll.
- `receipt` is a fresh, single-delivery handle (see fencing, below). `receives`
  is how many times this message has now been delivered (1 on first delivery).
- The message is hidden for `visibility_timeout_ms`. If it is not `ack`ed in
  that window, it becomes visible again and a later `receive` redelivers it
  with `receives` incremented and a **new** `receipt`.

### `ack`
- **params:** `{"queue": "<name>", "receipt": "<receipt>"}`
- **result:** `{"acked": true}` if that receipt named a still-in-flight
  delivery (the message is removed for good), else `{"acked": false}`.

### `nack`
- **params:** `{"queue": "<name>", "receipt": "<receipt>"}`
- **result:** `{"nacked": true}` if the receipt named a still-in-flight
  delivery, else `{"nacked": false}`.
- A `nack` makes the message visible again **immediately** (no waiting for the
  timeout). It counts as a delivery that did not succeed, so the message's
  `receives` is unaffected by the nack itself but the message is subject to the
  dead-letter rule just like a timeout (see DLQ).

### `stats`
- **params:** `{"queue": "<name>"}`
- **result:** `{"visible": <int>, "inflight": <int>}` for that queue, with
  visibility timeouts applied as of now (an in-flight message whose timeout has
  passed counts as visible). A never-seen queue reports zeroes.

### `configure`
- **params:**
  `{"queue": "<name>", "max_receives": <int>, "dead_letter_queue": "<name>"}`
- **result:** `{}`
- Sets the queue's dead-letter policy (see below). Both fields are required
  when configuring. Configuring creates the queue if needed.

## Receipt fencing (the rule that bites)

A `receipt` is valid for **exactly one delivery**. The moment a message is
redelivered — because its visibility timeout expired, or it was `nack`ed — any
earlier receipt for it is **permanently dead**.

So if consumer A receives a message, stalls past the timeout, the message is
redelivered to consumer B, and *then* A finally calls `ack` with its old
receipt: A's `ack` must return `{"acked": false}` and **must not** remove the
message — it is B's now, in-flight under B's receipt. Acking the wrong
delivery is the canonical way queue-backed systems silently lose work.

## Dead-letter queues

A poison message (one that never gets acked) would otherwise block its queue
forever, since it sorts ahead of everything sent later. A dead-letter policy
caps how many times it can be delivered:

- After `configure` sets `max_receives = N` and `dead_letter_queue = D`, a
  message that has been delivered `N` times from this queue and *fails again*
  (visibility timeout expires, or it is `nack`ed) is **moved to queue `D`**
  instead of becoming visible again here. In `D` it is an ordinary visible
  message (fresh `receives` count of 0, a new sequence number ordering it
  after whatever is already in `D`).
- Concretely with `N = 2`: the message is delivered as `receives` 1, then 2,
  and on the *next* failure (which would be delivery 3) it is dead-lettered
  rather than redelivered. It is delivered from the source queue at most `N`
  times.
- A queue with no configured policy redelivers forever.

## Durability

`send` and `ack` are the durable events:

- An acknowledged `send` survives `SIGKILL` and restart.
- An `ack`ed message stays gone across a crash.
- A message that was in-flight (received but not yet acked) when the process
  was killed becomes **visible again** after restart — that is at-least-once
  delivery: an un-acked message is never lost. (Visibility timers do not
  survive a crash; nothing has acked the message, so it is owed redelivery.)

`receives` counts and in-flight/visibility bookkeeping need not survive a crash
— only the set of un-acked messages and the queue configuration must.

## A note on fsync

As in the WAL challenge: `SIGKILL` cannot drop the OS page cache, so the tester
physically cannot catch a missing `fsync`. Write `send`/`ack` durability as if
the power can fail after any syscall — fsync before you acknowledge. The tester
checks everything it can observe; this part is on your honor.
