# Walkthrough — Build your own Raft

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint raft` prints just the hint for your next stage;
`crafters walkthrough raft --stage <slug>` prints one section.

## bind — Boot the cluster

> **Hint:** Parse `--node-id`, `--peers`, `--port`, `--data-dir`. Listen on
> the port and answer `ping` with your `node_id`. Peer addresses come from the
> `--peers` list (`1=host:port,2=host:port,…`).

**How it works:** Each process is one Raft node. The harness starts three nodes
with partition switches between them. Bind is wire-up only — no election yet.

## leader — Leader election

> **Hint:** Followers start election timers (≥300ms). When a timer fires,
> become candidate, increment term, vote for self, send `request_vote` to
> peers. A candidate with majority votes becomes leader and sends heartbeats.

**How it works:** The reference runs a background loop: followers timeout,
candidates request votes, leaders send periodic `append_entries` heartbeats.
`get_status` exposes `role`, `term`, and `leader_id`. One leader per term.

## replicate — Replicate a write

> **Hint:** Only the leader accepts `set`. Append to the local log, replicate
> via `append_entries` to followers, advance `commit_index` when a majority
> has replicated an entry, then apply to the state machine.

**How it works:** Client writes go to the leader's log. Replication piggybacks
on `append_entries`. Commit requires quorum match on an index; then `commit_index`
and `last_applied` advance. Followers reject stale leaders via term checks.

## read — Read your writes

> **Hint:** `get` on the leader reads from the applied state machine (committed
> entries only). Followers can return `NOT_LEADER` with the current leader hint,
> or you can serve linearizable reads from the leader after commit.

**How it works:** The reference applies committed log entries to an in-memory
KV map. `get` returns values from applied state. Uncommitted leader entries are
not visible to reads.

## follower-crash — Survive a follower crash

> **Hint:** A cluster needs a majority to commit. With three nodes, one follower
> down still leaves two — commits continue. The crashed node is just absent from
> quorum until it returns.

**How it works:** The harness kills one follower. Leader replicates to the
remaining follower; majority of 2/3 suffices. Writes commit and reads work.
No special case — quorum math does the work.

## leader-crash — Survive a leader crash

> **Hint:** When the leader stops heartbeating, followers' election timers fire.
> A new term elects a new leader. Committed entries survive; the new leader
> replicates any uncommitted tail before accepting new writes.

**How it works:** The reference persists the log. After leader kill, remaining
nodes elect. New leader's log must be at least as up-to-date as any majority
candidate. Committed entries are never lost.

## durability — Survive a full crash

> **Hint:** Persist `term`, `voted_for`, log, `commit_index`, and applied KV to
> `--data-dir` on every change. After `SIGKILL` and restart, reload and
> rejoin — same node id, same data dir, catch up from peers.

**How it works:** The reference writes state to disk before acknowledging RPCs
that mutate it. On boot, load snapshot + log, then participate in elections and
replication. A restarted node may need to catch up from the leader.

## partition — Partition safety

> **Hint:** A minority partition must not commit new writes. If the leader is
> isolated with one follower (2 nodes), it can't reach quorum — return
> `NOT_COMMITTED` or fail writes. The majority side keeps going.

**How it works:** The harness partitions the network. The reference only
commits when `match_index` on a majority is reached. Minority leaders may
be elected in a split term but can't commit client writes without quorum.

## gauntlet — The gauntlet

> **Hint:** Writes, leader crashes, follower crashes, restarts, partitions —
> the same rules throughout: majority commit, term monotonicity, log matching,
> durable state. Don't special-case the gauntlet.

**How it works:** The gauntlet randomizes failures. The reference relies on
persistent log + election safety + quorum commit. Recovery is always: load disk,
rejoin cluster, replicate, catch up `commit_index`.
