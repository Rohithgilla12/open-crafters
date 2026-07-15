# Stage 6: Delayed digest

Implement `schedule_notify` via scheduler `schedule`. On `receive`, poll due jobs and move their payloads onto queue `notifications` before receiving.
