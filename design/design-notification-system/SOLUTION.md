# Reference architecture — notification platform

## Diagram

```
Product services ──▶ Notify API ──▶ Pref cache
                           │
                           ▼
                    Notification DB (state)
                           │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
         email-q      push-q       sms-q
              │            │            │
              ▼            ▼            ▼
         SendGrid       APNs        Twilio
```

## Notify API

1. Resolve template + channels (or use defaults).
2. Load preferences — filter channels.
3. Check dedupe key.
4. Insert `notifications` row (`status=pending`).
5. Publish to channel queue(s).
6. Return `202 Accepted` + `notification_id`.

## Worker loop

Same pattern as your **queue** challenge:

1. Poll message with lease.
2. Render template for channel.
3. Call provider API.
4. Update status; ACK or retry with backoff.
5. Dead-letter after N failures.

## Rate limiting

| Limit | Where |
|-------|-------|
| Per user | Before enqueue — "max 10 SMS/day" |
| Per provider | Worker side — token bucket on APNs calls |
| Global burst | API gateway |

## Digest path

Low-priority templates tagged `digest=true` → write to `digest_items` only.

**Scheduler** job: `SELECT users WHERE local_time=8am` → merge items → single email → clear buffer.

## Observability

Metrics: enqueue rate, delivery latency per channel, provider error codes, opt-out rate.

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Durable job delivery | `build-your-own-queue` |
| Digest / delayed send | `build-your-own-scheduler` |
| Provider + user caps | `build-your-own-rate-limiter` |
| Compose gateway | `build-your-own-notification-platform` |

Notifications are a **fan-out queue problem** with policy in front — you've built every layer, then wired them in the compose gateway.
