# Stage 7: Survive a full crash

Real systems persist state to disk. After **every node** is killed, restarting with
the same `--data-dir` must restore committed data.

## Your task

Persist to `--data-dir` (atomic writes recommended):

- current term and `voted_for`
- the Raft log
- `commit_index`, applied KV state

Survive `SIGKILL` of all three nodes, then restart each with the **same** data dir
and port.

## Tests

After a committed write, total cluster crash, and restart, a new leader is elected
and `get` returns the previously committed value.

## Notes

- This stage requires durable persistence — in-memory state alone will fail.
- See [Build your own WAL](../../build-your-own-wal/) if you want a refresher on
  crash-safe writes.
