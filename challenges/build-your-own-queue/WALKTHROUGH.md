# Walkthrough — Build your own message queue

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint queue` prints just the hint for your next stage;
`crafters walkthrough queue --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Same newline-delimited JSON loop as every challenge: read line,
> decode, dispatch, respond, flush. `ping` returns `pong` — wire up transport
> once, then add queue methods stage by stage.

**How it works:** The reference isolates RPC dispatch from TCP handling. Each
connection is independent. `--data-dir` is ignored until durability.

## send-receive — Send and receive

> **Hint:** A queue is a FIFO of messages with unique ids. `send` appends;
> `receive` returns the oldest *visible* message and marks it in-flight with a
> receipt. Empty queue → block or return nothing per the protocol timing.

**How it works:** Messages live in a per-queue list/map with monotonic sequence
numbers. `receive` picks the head visible message, generates a unique receipt,
and marks it invisible until the visibility timeout expires. Stats expose
`visible` vs `inflight` counts.

## durability — Survive a crash

> **Hint:** Persist the full broker state after every mutation — all queues,
> all messages, sequence counters. On startup reload and continue. Acked
> messages must stay gone; un-acked inflight messages must come back.

**How it works:** The reference snapshots queues to JSON (temp + rename) after
each `send`, `ack`, `nack`, etc. Receipts and message ids use crypto-random
hex so restarts never collide with recovered ids. Inflight messages without ack
survive `SIGKILL`.

## redelivery — Visibility timeout redelivery

> **Hint:** Each received message has `invisible_until`. When `now` passes
> that deadline without an ack, the message becomes visible again — same body,
> new receipt. The old receipt must stop working.

**How it works:** `receive` stamps `invisible_until = now + vis_ms`. A background
tick or lazy check on receive promotes expired inflight messages back to
visible and issues a fresh receipt. The old receipt is invalidated — that's
at-least-once delivery.

## nack — Negative acknowledge

> **Hint:** `nack` is "put it back now" — make the message visible immediately
> without waiting for the visibility timeout. Return whether the receipt was
> still valid.

**How it works:** Valid nack clears inflight state and returns the message to
the head of the visible queue (preserving order). Invalid/expired receipts
return `found: false`. Nack is how workers signal "I can't process this yet."

## fencing — Receipt fencing

> **Hint:** A receipt is a lease on one specific delivery. After ack, nack, or
> visibility expiry, that receipt must never ack again — even if the worker is
> slow and tries later.

**How it works:** The reference stores the current receipt on the message.
`ack` succeeds only when the receipt matches and the message is inflight; then
the message is deleted and the receipt burned. Late acks after redelivery get
`found: false` — classic fencing against duplicate processing.

## dead-letter — Dead-letter queue

> **Hint:** Track `receives` per message. When it exceeds `max_receives`,
> move the message to the configured DLQ instead of redelivering to the main
> queue. `configure` sets the policy per queue.

**How it works:** Each receive increments a counter. On expiry or nack, if
`receives >= max_receives` the message is removed from the source queue and
appended to the DLQ (creating it if needed). Healthy messages still redeliver
normally.

## queues — Multiple queues

> **Hint:** `map[queueName]*queue` — each queue owns its own messages and DLQ
> policy. Queue names are opaque strings; no cross-queue ordering guarantees.

**How it works:** The broker holds independent queue structs. `send`/`receive`
/`ack`/`stats` all take a `queue` parameter. DLQ routing is per-source-queue
configuration. Isolation is automatic because state is partitioned by name.

## gauntlet — The gauntlet

> **Hint:** Compose everything: durable snapshot, visibility redelivery,
> receipt fencing, DLQ after max receives, multiple queues. Persist after
> every state change and never resurrect an acked message.

**How it works:** The gauntlet interleaves sends, receives, acks, nacks, kills,
and restarts. The reference relies on the same snapshot-on-mutation path
throughout — fencing and DLQ rules are enforced in memory and survive because
the full broker state is reloaded faithfully.
