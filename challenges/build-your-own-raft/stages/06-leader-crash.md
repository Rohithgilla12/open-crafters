# Stage 6: Survive a leader crash

When the leader dies, followers must **elect a new leader** that continues from the
committed log.

## Your task

Again, no new client RPCs — prove recovery after the **leader** is killed:

1. Commit a write (`before=crash`).
2. Kill the leader.
3. A new leader emerges within the election window.
4. The new leader serves the old write and accepts new ones.

## Tests

After leader failure, `get("before")` on the new leader returns `"crash"`, and a
new `set` succeeds.

## Notes

- Candidates with stale logs must not win over more up-to-date peers (standard Raft
  log-comparison rules on `request_vote`).
- `leader_id` across live nodes should converge on the new leader.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** When the leader stops heartbeating, followers' election timers fire. A new term elects a new leader. Committed entries survive; the new leader replicates any uncommitted tail before accepting new writes.

Or run: <code>crafters hint raft --stage leader-crash</code>
</details>
<!-- /crafters-stage-hint -->
