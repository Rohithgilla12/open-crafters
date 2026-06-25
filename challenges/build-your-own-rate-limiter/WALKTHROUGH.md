# Walkthrough — Build your own rate limiter

How the reference solution approaches each stage. Each section opens with a
spoiler-free **Hint** (a design nudge for when you're stuck) followed by **How
it works** (read this *after* you pass the stage, to check your model against
the reference's). No code — the point is the design.

`crafters hint rate-limiter` prints just the hint for your next stage;
`crafters walkthrough rate-limiter --stage <slug>` prints one section.

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

## fixed-window — Fixed-window counter

> **Hint:** The window for a timestamp is `floor(now / window_ms)`. Store that
> index alongside a count; when the index you compute differs from the one you
> stored, the window rolled over — zero the count.

**How it works:** Each limiter holds `(window_index, count)`. On every `take`,
the reference recomputes `idx = now / window_ms`; if `idx != window_index` it
resets `count` to 0 and adopts the new index. Admission is `count + cost <=
limit`. The reset is *lazy* — computed on access, never on a timer — which is
the same trick that makes refill and durability easy later. `retry_after_ms`
on a denial is `(window_index + 1) * window_ms - now`: the time until the
current window ends.

## token-bucket — Token bucket

> **Hint:** Don't run a refill timer. Store `(tokens, as_of_ms)`; on each
> access add `(now - as_of) / refill_interval_ms * refill_tokens`, cap at
> capacity, and set `as_of = now`. Then decide.

**How it works:** The reference treats refill as *accrual computed on read*.
Before any decision it calls one `refill` helper that advances `tokens` to
`now` (capped at `capacity`) and stamps `as_of_ms = now`. Admission is then a
plain `tokens >= cost`, followed by `tokens -= cost`. Tokens are a float so
fractional accrual is exact; `remaining` is reported as the floor. A denial's
`retry_after_ms` is `ceil((cost - tokens) / refill_tokens * refill_interval_ms)`
— exactly how long until the deficit accrues. Because the whole state is
`(tokens, as_of_ms)`, lazy accrual is also what makes the bucket survive a
crash unchanged.

## sliding-window — Sliding window

> **Hint:** Keep the timestamps of admitted units. "How many in the last
> `window_ms`?" is "how many timestamps are newer than `now - window_ms`?"
> Trim the rest as you go so the list can't grow without bound.

**How it works:** The reference stores a log of `[timestamp, cost]` entries.
On each access it drops entries older than `now - window_ms`, sums the
surviving cost, and admits iff `used + cost <= limit`. This is what removes
the fixed-window boundary burst: there is no grid to reset against, so no
`window_ms`-wide span ever exceeds `limit`. The denial `retry_after_ms` walks
the oldest entries until enough cost would age out, and returns that entry's
expiry (`ts + window_ms - now`). Trimming on every access keeps `take` O(in-
window entries), which matters for the throughput floor.

## multi-key — Independent keys

> **Hint:** One map from `key` to limiter state. `configure` overwrites the
> entry (fresh state); `take`/`peek` on a missing entry is an error, not an
> implicit create.

**How it works:** All limiters live in a single `key -> state` map. `configure`
builds a brand-new limiter struct and assigns it, which is why reconfiguring
resets consumption for free — the old state is simply replaced. `take` and
`peek` look the key up and raise `KEY_NOT_FOUND` when it's absent; there is no
default limiter. Isolation is automatic because each key owns its own state.

## peek — Peek without consuming

> **Hint:** Factor "how much is available at `now`" into one function that both
> `take` and `peek` call. `peek` runs the accrual but skips the deduction.

**How it works:** The reference's `refill` (accrual) and `retry_after`
(wait-estimate) helpers are pure with respect to consumption, so `peek` calls
exactly the same code as `take` minus the `consume` step. That shared core is
what keeps `take`, `peek`, and `retry_after_ms` in agreement — there's only
one definition of "available," so they can't drift. `peek` advancing accrual
(tokens only ever increase) is harmless, which is why two peeks agree modulo
real refill.

## concurrency — Atomic admission

> **Hint:** "Is there room?" and "take the room" must be one indivisible step.
> If another request can slip between them, you'll admit one too many.

**How it works:** The check-and-consume runs under a lock held for the whole
decision: refill, compare, deduct, all before the lock is released. The JSON
decode/encode happens outside it. With per-key state a per-key lock suffices,
but the reference uses one mutex for simplicity — correctness first. (The
TypeScript reference gets atomicity from the single-threaded event loop: each
handler runs to completion.) The invariant the tester pins is "exactly
`capacity` admitted from a full bucket under a storm of concurrent takes" —
the classic check-then-act race produces `capacity + N`, and the lock is what
forecloses it.

## durability — Survive a crash

> **Hint:** You already store everything as `(value, as_of_ms)` or
> timestamped entries. Persist that, and on load recompute from `now`. Persist
> on each mutation — `SIGKILL` can't run a shutdown hook.

**How it works:** Because every algorithm's state is absolute (token balance
plus the time it was current; window index; timestamped log), persistence is
just serialising the `key -> state` map and reloading it. The reference writes
the whole (tiny) map to a temp file and `rename`s it over the real one after
each mutating call. It deliberately skips `fsync`: the threat model is
`SIGKILL`, not power loss, and a process kill leaves the written bytes in the
page cache — so the atomic rename is enough and the hot path stays fast. On
boot it loads the map; the first access recomputes accrual from the persisted
`as_of_ms` to `now`, so legitimately-accrued tokens come back while a drained
bucket stays drained.

## gauntlet — The gauntlet

> **Hint:** If you persist correctly and trim sliding logs, the throughput
> floor takes care of itself. The traps are fsync-per-take, rescanning an
> untrimmed log, and serialising every key behind one contended lock.

**How it works:** Nothing new — the gauntlet just runs the mix under a crash
and then times a few thousand takes. The reference clears the floor with large
margin precisely because of earlier choices: state is small so rewriting it
per take is cheap, no `fsync` is on the hot path, sliding logs are trimmed so
`take` stays bounded, and accrual is O(1). The performance tier is the lesson
that a limiter on every request's hot path must be *fast*, not just correct —
the same state that makes it durable also makes it cheap to persist.
