# Design a notification platform

Design **Stripe / Airbnb-style notifications**: product events trigger email, mobile push, SMS, and in-app alerts — respecting user preferences.

## Functional requirements

1. **Send** — API: `notify(user_id, template, payload, channels?)`.
2. **Preferences** — per-user channel toggles, quiet hours, locale.
3. **Templates** — parameterized bodies per channel.
4. **Delivery** — integrate third parties (SendGrid, APNs, Twilio).
5. **Status** — delivered, bounced, failed (per channel).
6. **Digest** — batch low-priority items into daily email.

## Scale

| Metric | Value |
|--------|-------|
| Events / day | 500M |
| Peak send rate | 50k notifications/sec |
| Users | 30M |
| Templates | 2k |

Most traffic is **async** — milliseconds of API latency OK.

## Non-functional

- At-least-once delivery with dedupe at consumer.
- Provider rate limits must not drop notifications silently.
- Preference check on every send — can't spam users who opted out.

## Your task

Whiteboard **35–45 minutes**:

1. Event ingestion → outbound message pipeline.
2. Preference resolution before enqueue.
3. Per-channel workers and retry policy.
4. Dedupe: "password reset sent twice in 1 minute."
5. Digest scheduler vs immediate send architecture.

## Stretch

- Priority lanes (fraud alert vs marketing).
- Multi-region with user data residency.
