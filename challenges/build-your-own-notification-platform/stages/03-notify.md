# Stage 3: Immediate notify

Implement `notify` — `take` against `user:<user_id>`; when allowed, enqueue a JSON envelope on queue `notifications` and return `notification_id` + `queued=true`.
