# Build your own distributed lock

Build a **distributed lock service** — the primitive behind leader election,
mutexes in microservices, and every "only one worker at a time" system.

Your lock service will:

- **acquire** named locks with time-bounded leases,
- **try_acquire** without blocking on contention,
- **release** with a token so only the holder can unlock,
- **renew** leases to extend work safely,
- **expire** locks automatically when leases run out,
- **persist** active locks across crashes (absolute `expires_at_ms`),
- and stay correct under **concurrent** acquire/release on many lock names.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + `ping` |
| 2 | [Acquire a lock](stages/02-acquire.md) | `acquire`, `status` |
| 3 | [Release a lock](stages/03-release.md) | `release` |
| 4 | [Lock contention](stages/04-conflict.md) | `LOCK_HELD` |
| 5 | [Try without blocking](stages/05-try-acquire.md) | `try_acquire` |
| 6 | [Lease expiry](stages/06-expiry.md) | automatic release |
| 7 | [Renew a lease](stages/07-renew.md) | `renew`, `NOT_HOLDER` |
| 8 | [Survive a crash](stages/08-durability.md) | persist active locks |
| 9 | [The gauntlet](stages/09-gauntlet.md) | concurrent multi-lock stress |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) — it's the complete contract. Copy a starter
from [starters/](starters/), then:

```sh
crafters start distributed-lock
crafters test
```

Stuck? Reference solutions live in
[examples/solutions/build-your-own-distributed-lock/](../../examples/solutions/build-your-own-distributed-lock/).
