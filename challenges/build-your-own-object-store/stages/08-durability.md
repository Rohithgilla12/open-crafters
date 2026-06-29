# Stage 8: Survive a crash

In-memory storage evaporates when the process dies. From here on, acknowledged
writes must survive `SIGKILL`.

## Your task

Persist your object store (and any in-progress multipart state you choose to
support) under `--data-dir`. Reload on startup.

## Tests

- `put` two objects, `delete` a third, then the tester `SIGKILL`s your process
  and restarts it with the same `--data-dir`.
- The two objects must still be readable with the correct bodies.
- The deleted key must still be gone (`NOT_FOUND` on `get`).

## Notes

- The on-disk format is yours — the tester only checks behavior.
- Write durability as if power can fail: persist before you acknowledge the
  RPC. Atomic rename of a temp file is a common pattern.
