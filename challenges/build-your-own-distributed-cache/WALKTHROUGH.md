# Walkthrough — Build your own distributed cache

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** followed by **How it works** (read after you pass).
No code — the point is the design.

`crafters hint distributed-cache` prints just the hint for your next stage;
`crafters walkthrough distributed-cache --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** Same newline-delimited JSON loop as every challenge: read line,
> decode, dispatch, respond, flush. `ping` returns `pong` — wire transport once,
> then add cache methods stage by stage.

**How it works:** The reference isolates RPC dispatch from TCP handling. Each
connection is independent. No persistence — everything lives in memory.

## set-get — Set and get

> **Hint:** `map[key]entry` with `value`, `version`, and optional `expires_at`.
> `get` on a missing key returns `hit: false`. First `set` starts at version 1;
> each overwrite increments version.

**How it works:** Versions power later CAS. A hit returns value + version; a miss
is just `hit: false` with no value field.

## delete — Delete a key

> **Hint:** `delete` removes the key if present and returns `deleted: true`.
> Double-delete returns `deleted: false`. After delete, `get` misses.

**How it works:** Expired keys are treated as absent — delete on an already-expired
key returns false.

## ttl — TTL expiration

> **Hint:** Store `expires_at = now + ttl_ms` on set. On every `get`, if
> `now >= expires_at`, remove the entry and return a miss.

**How it works:** The tester uses `ttl_ms=200`, verifies an immediate hit, sleeps
~280ms, then expects a miss.

## setnx — Set if not exists

> **Hint:** Only store when the key is missing or expired. Return
> `stored: false` if a live entry exists — do not overwrite.

**How it works:** This models `SET NX` / lock acquisition. After `delete`, `setnx`
succeeds again.

## cas — Compare and swap

> **Hint:** Swap only when `expected_version` matches the live entry's version.
> On success increment version and return `swapped: true`. Stale version or
> missing key → `swapped: false` without changing state.

**How it works:** Optimistic concurrency for hot keys — failed CAS leaves the
value untouched.

## mget — Multi-get

> **Hint:** Return `entries` in the same order as the input `keys` array. Each
> element is either a hit (value + version) or `hit: false`. Reject empty or
> >50 keys with `INVALID_PARAMS`.

**How it works:** Batch reads reduce round trips; each hit updates LRU recency.

## lru — LRU eviction

> **Hint:** After `configure max_keys`, track recency (linked list or
> `OrderedDict`). `get` and successful writes mark a key most-recent. Inserting
> a **new** key at capacity evicts the least-recently-used key.

**How it works:** The tester fills 3 slots, touches `k1`, inserts `k4`, and
expects `k2` (the coldest key) to be evicted.

## gauntlet — The gauntlet

> **Hint:** Mutex around the map + LRU structure. Many connections set/get
> distinct keys concurrently; final verify reads them all. Finish with TTL
> expiry and `setnx` on a live key.

**How it works:** The gauntlet catches races that lose writes or corrupt LRU
order under parallel load.
