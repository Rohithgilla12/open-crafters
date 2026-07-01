# Walkthrough — Build your own hash ring

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint hash-ring` prints just the hint for the next stage;
`crafters walkthrough hash-ring --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Same newline-delimited JSON loop as every challenge: read line,
> decode, dispatch, respond, flush. `ping` returns `pong` — wire up transport
> once, then add hash-ring methods stage by stage.

**How it works:** The reference isolates RPC dispatch from TCP handling. Each
connection is independent. No persistence — everything lives in memory.

## create — Create a ring

> **Hint:** `map[ring_id]*ring` where each ring stores `replicas` and a set of
> physical node IDs. Reject duplicates with `RING_EXISTS`; validate
> `replicas >= 1` with `INVALID_PARAMS`.

**How it works:** `create_ring` records the replica count and an empty node set.
Ring IDs are unique; a second create with the same ID is an error. Vnodes are
computed on the fly from node_id + replica index when looking up — no need to
store every vnode position unless you want to cache them.

## add-node — Add a node

> **Hint:** Add the physical `node_id` to the ring's node set. With one node,
> every lookup returns it. Duplicate node → `NODE_EXISTS`; unknown ring →
> `RING_NOT_FOUND`.

**How it works:** Lookup builds the sorted vnode list from all nodes × replicas,
walks clockwise from `hash_key(key)`, and returns the owning physical node.
With one node, every key maps to it regardless of hash.

## deterministic — Deterministic lookup

> **Hint:** Lookup is a pure function of ring membership and the key — no
> randomness, no mutation. Call it fifty times; same answer every time, and it
> must match the PROTOCOL hash walk (FNV-1a positions, sort, clockwise search with wrap. The tester compares against a reference oracle — wrong hash = fail.

**How it works:** The reference rebuilds vnodes on each lookup, sorts by position
(with tie-break on node_id), finds the first position ≥ h, wraps if needed.
Fifty identical calls must match the oracle once.

## spread — Key spread

> **Hint:** With three physical nodes and `replicas=1`, three vnodes sit on the
> ring. Two thousand keys should land on each node at least ~15% of the time
> if hashing is correct — gross imbalance means broken vnode positions or
> lookup walk.

**How it works:** The tester counts owners for 2000 keys and checks each node
gets ≥ 300. It also compares every lookup to the reference oracle.

## minimal-move — Add node (minimal move)

> **Hint:** Consistent hashing: adding a node should only move keys that would
> now belong to it — roughly 1/n of keys for n nodes, not half the ring. After
> adding a fourth node to three, fewer than 45% of keys should change owner.

**How it works:** Record 200 lookups, add `n4`, recount. Changed keys must be
< 90 and every new lookup must match the reference oracle with four nodes.

## remove-node — Remove a node

> **Hint:** Drop the physical node and its vnodes. Lookups must never return
> the removed ID; surviving keys remap via the same clockwise walk on the
> smaller ring.

**How it works:** After remove, probe keys — none return the removed node.
Each result matches the reference with the remaining nodes.

## virtual-nodes — Virtual nodes

> **Hint:** Higher `replicas` places more vnodes per physical node, flattening
> load. With three nodes and 600 keys, `replicas=50` should have a smaller
> (max−min) key count spread than `replicas=1`.

**How it works:** Two rings, same nodes, different replica counts. Count keys
per node; compare spread. Virtual nodes should tighten the range.

## gauntlet — The gauntlet

> **Hint:** Protect ring state with a lock (or equivalent) for concurrent
> add/remove/lookup from many connections across two rings. Pure concurrency —
> no crash restart.

**How it works:** Multiple goroutines/threads hammer two rings with mixed
ops, then a verification pass checks consistency against the reference oracle
for recorded keys.
