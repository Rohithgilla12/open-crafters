# Stage 8: Survive a crash

A lock service that forgets held locks on restart lets two workers both believe
they are the leader. Active locks must survive `SIGKILL` + restart with the
same `--data-dir`.

## Your task

Persist enough state that, after a crash and restart:

1. An **unexpired** lock is still held with the same `holder_id` and `token`.
2. `release` with the saved token still works.
3. **Expired** locks are treated as free (absolute `expires_at_ms` on disk).

The tester acquires with a long lease, `SIGKILL`s your process, restarts it,
checks `status` is still held, then releases.

## What the tester checks

- After restart, `status` reports `held: true` with the original holder/token.
- `release` with the pre-crash token returns `released: true`.

## Notes

- Persist on each mutating call (`acquire`, `release`, `renew`). Atomic
  write-to-temp + rename is enough — `SIGKILL` cannot be caught.
- Store `expires_at_ms` as absolute time so downtime counts toward expiry.
