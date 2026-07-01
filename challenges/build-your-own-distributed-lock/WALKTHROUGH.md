# Walkthrough — Build your own distributed lock

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** (a design nudge for when you're stuck) followed by **How
it works** (read this *after* you pass the stage, to check your model against
the reference's). No code — the point is the design.

`crafters hint distributed-lock` prints just the hint for your next stage;
`crafters walkthrough distributed-lock --stage <slug>` prints one section.

## bind — Boot the server

> **Hint:** A line-delimited JSON server is a `for` loop over lines: read one,
> decode, dispatch on `method`, write one line back. Get the envelope right
> and every later stage is just a new `case`.

**How it works:** The reference keeps a single dispatch function keyed on
`method`, with `ping` returning `{"message":"pong"}` and everything else
falling through to an `UNKNOWN_METHOD` error. The transport (read a line,
`json` decode, `json` encode, write a line, flush) is written once and never
touched again — each stage only adds a method handler. `--data-dir` is parsed
and ignored until durability.

## acquire — Acquire a lock

> **Hint:** One map from lock `name` to `{holder_id, token, expires_at_ms}`.
> On acquire, if the entry is missing or `now >= expires_at_ms`, grant: mint a
> new token and set `expires_at_ms = now + lease_ms`.

**How it works:** The reference stores locks in a `name → state` map. `status`
recomputes `held` as `expires_at_ms > now` on every read — no timers. A fresh
grant always issues a new random token so stale holders cannot release after
re-acquisition. Parameter validation is centralized: missing fields or
`lease_ms < 1` → `INVALID_PARAMS`.

## release — Release a lock

> **Hint:** Compare the caller's `token` to the stored token *and* check the
> lease is still unexpired. Match → clear holder fields and return
> `released: true`; anything else → `released: false` with no RPC error.

**How it works:** Release is intentionally soft-fail: wrong token, expired
lease, or already-free lock all return `released: false`. Only a matching
active holder clears the lock. The map entry may remain (empty/free) so the
name is cheap to re-acquire.

## conflict — Lock contention

> **Hint:** Before granting, ask "is someone else holding this with
> `expires_at_ms > now`?" If yes, `acquire` errors `LOCK_HELD`; the stored
> holder and token stay untouched.

**How it works:** Contention is a single branch in the grant path. Expiry is
checked first — an expired lease is treated as free, so a crashed holder does
not block forever. The same holder trying to acquire again without releasing
also gets `LOCK_HELD` (one active lease per name).

## try-acquire — Try without blocking

> **Hint:** Share the grant function with `acquire`; on contention return
> `{"acquired": false}` instead of an error.

**How it works:** `try_acquire` and `acquire` call the same internal helper
with a flag for error-vs-false. Invalid params still error; only contention
differs. This keeps semantics identical for the success path.

## expiry — Lease expiry

> **Hint:** Never run a sweeper. On every acquire/status/release/renew, treat
> `expires_at_ms <= now` as "free". Wall-clock `expires_at_ms` makes downtime
> count toward expiry automatically.

**How it works:** Lazy expiry means restart + durability work for free: load
state from disk, compare to `now`, and expired rows behave as unlocked. No
background goroutines or timers to reconcile after a crash.

## renew — Renew a lease

> **Hint:** Verify token matches an unexpired holder, then set
> `expires_at_ms = max(now, current_expires_at_ms) + lease_ms`.

**How it works:** Renew stacks time onto the existing lease when still valid
(`max` with current expiry), so a worker can heartbeat without losing the
slack already on the clock. Wrong token or expired/free lock → `NOT_HOLDER`
(hard error, unlike release). Successful renew persists like acquire.

## durability — Survive a crash

> **Hint:** Persist the whole `name → {holder_id, token, expires_at_ms}` map
> on every mutating RPC. Atomic rename of a small JSON file is enough for
> SIGKILL.

**How it works:** The reference writes `state.json` via temp file + rename after
each `acquire`, `release`, and `renew`. On boot it loads the map and applies
lazy expiry. Because `expires_at_ms` is absolute, a lock held across a short
restart stays held; one that would have expired during downtime is free on
load.

## gauntlet — The gauntlet

> **Hint:** Grant must be atomic: read state, check expiry, decide, write token
> — all under one mutex per engine (or per lock). JSON I/O stays outside the
> lock if you want, but check-and-grant cannot be split.

**How it works:** The gauntlet fires many concurrent `acquire` calls on the
same few names; without atomicity two winners slip through. Per-engine mutex
around the map is sufficient. Throughput is kept high by avoiding fsync and
keeping persistence to a small JSON blob rewritten in memory then renamed once
per mutation — the same pattern as the rate limiter durability stage.
