# Stage 5: Survive a follower crash

Raft tolerates minority failure: a cluster of three needs only **two** nodes to
commit.

## Your task

No new RPCs — keep the cluster working when one **follower** is killed (`SIGKILL`).

The tester:

1. Waits for a stable leader.
2. Kills one follower.
3. Commits a new write via the remaining majority.

## Tests

`set` on the leader must succeed and replicate while one follower is down.

## Notes

- The dead follower's data dir is left on disk; it may catch up when restarted later.
- Heartbeats and replication must skip or time out on unreachable peers without blocking the leader forever.
