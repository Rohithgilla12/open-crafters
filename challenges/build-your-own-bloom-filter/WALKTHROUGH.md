# Walkthrough — Build your own bloom filter

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint bloom-filter` prints just the hint for your next stage;
`crafters walkthrough bloom-filter --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Same newline-delimited JSON loop as every challenge: read line,
> decode, dispatch, respond, flush. `ping` returns `pong` — wire up transport
> once, then add bloom-filter methods stage by stage.

**How it works:** The reference isolates RPC dispatch from TCP handling. Each
connection is independent. No persistence — everything lives in memory.

## create — Create a filter

> **Hint:** `map[filter_id]*filter` where each filter holds an `m`-bit array
> and `k`. Reject duplicates with `FILTER_EXISTS`; validate `m >= 8` and
> `k >= 1` with `INVALID_PARAMS`.

**How it works:** `create` allocates a fresh bit slice (or bool array) of length
`m`, all zeros. Filter IDs are unique strings; a second `create` with the same
ID is an error.

## add — Add an item

> **Hint:** Hash the item's UTF-8 bytes with the PROTOCOL's FNV-1a scheme to get
> k positions, then set those bits to 1. Unknown filter → `FILTER_NOT_FOUND`.

**How it works:** `add` looks up the filter, computes k indices via double
hashing, and ORs them into the bit array. Adding the same item again is a no-op
at the bit level.

## positive — Positive lookup

> **Hint:** `contains` checks that **all** k bit positions are set. An item you
> added must return `maybe_present: true` — bloom filters never lie about
> membership in the positive direction.

**How it works:** After `add`, every hash position for that item is 2, so
`contains` returns true. This is the no-false-negatives guarantee.

## negative — Negative lookup

> **Hint:** With a large `m`, few inserts, and modest `k`, items you never
> added should usually return `maybe_present: false`. False positives exist in
> theory but are unlikely when the filter is sparse.

**How it works:** The tester uses `m=1024`, `k=3`, and at most two inserts, then
probes ten items that were never added. A correct implementation returns false
for all of them in practice.

## no-false-negatives — No false negatives

> **Hint:** This is the contract that makes bloom filters useful: if you added
> it, `contains` must say true — always, even after hundreds of inserts.

**How it works:** The tester adds 200 distinct items and verifies every one
returns `maybe_present: true`. A bug in hashing or bit-setting that clears
bits will fail here.

## multi-filter — Independent filters

> **Hint:** Each `filter_id` gets its own bit array. Adding to filter `a` must
> not flip bits in filter `b`.

**How it works:** The reference stores filters in a map keyed by ID. Lookup,
add, and contains all scope to one filter at a time.

## hash-functions — K hash functions

> **Hint:** Do not stop at `h1 % m`. You need `(h1 + i*h2) % m` for every
> `i` in `0..k-1`. The tester finds pairs where a single-hash cheat would
> falsely report present but the full k-hash check correctly says absent.

**How it works:** Double hashing derives k distinct positions from two FNV runs.
Using only the first hash makes `k>1` filters behave like `k=1`, which the
tester catches with crafted collision pairs.

## gauntlet — The gauntlet

> **Hint:** Protect the filter map and bit arrays with a lock (or equivalent)
> so concurrent connections can add and probe safely. `delete_filter` is
> optional but handy for cleanup under churn.

**How it works:** Multiple connections create filters, add items, probe them,
and occasionally delete filters. After the storm, every item that was
successfully added must still probe true. No crashes — pure concurrency.
