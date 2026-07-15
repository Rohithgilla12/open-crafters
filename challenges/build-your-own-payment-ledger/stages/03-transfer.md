# Stage 3: Transfer funds

Implement `transfer` — mint an id, update both balances in one MVCC transaction, persist the envelope and idempotency key in the WAL.
