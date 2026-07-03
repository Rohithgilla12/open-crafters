# Hints — notification platform

## Pipeline shape

```
Notify API → validate prefs → enqueue NotificationJob → channel workers → providers
```

API returns `notification_id` fast; delivery is async.

## Preference service

`user_prefs(user_id) → {email: bool, push: bool, quiet_hours: ...}`

Cached in Redis. Check **before** enqueue — legal requirement.

## Per-channel queues

Separate queues: `email`, `push`, `sms`. Workers scale independently. Failed push doesn't block email.

## Dedupe

Key: `(user_id, template_id, dedupe_window)` in Redis or DB. Skip if seen in last N minutes.

## Digests

**Scheduler** accumulates events in `digest_buffer(user_id, day)`; cron job at 8am flushes one email.

## open-crafters tie-in

- **Queue** — durable notification jobs with visibility timeout
- **Scheduler** — digest cron + delayed sends
- **Rate limiter** — per-provider and per-user caps
