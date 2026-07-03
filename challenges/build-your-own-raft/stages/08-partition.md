# Stage 8: Partition safety

Raft must not commit writes without a **quorum**. A leader isolated from the
majority must not succeed on client writes.

## Your task

When the leader is **network-partitioned** from the other two nodes:

- `set` on the isolated leader must fail with `NOT_COMMITTED` or `NOT_LEADER`.
- The **majority partition** must elect a leader and commit a new write.

The tester uses in-process TCP switches (see harness) to block traffic between
node pairs; your nodes use the `--peers` addresses as usual.

## Tests

1. Partition the current leader from the rest.
2. Expect the isolated leader's `set` to fail.
3. Expect the majority side to commit `majority=wins`.

## Notes

- Do not acknowledge `set` until a quorum replicates — a partitioned leader should
  time out with `NOT_COMMITTED`.
- After the test, partitions are healed automatically.
---

<!-- crafters-stage-hint -->
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** A minority partition must not commit new writes. If the leader is isolated with one follower (2 nodes), it can't reach quorum — return `NOT_COMMITTED` or fail writes. The majority side keeps going.

Or run: <code>crafters hint raft --stage partition</code>
</details>
<!-- /crafters-stage-hint -->
