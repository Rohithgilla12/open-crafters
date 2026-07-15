# Stage 6: Idempotent transfer

Replaying the same `idempotency_key` returns the same `transfer_id` with `replayed=true` and does not move money twice.
