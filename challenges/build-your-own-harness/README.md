# Build your own harness

Build a **mini grader** — the same primitive behind open-crafters: spawn user
programs, proxy NDJSON RPC to them, and assert outcomes with `run_case`.

This is a **meta** challenge: the real harness tests your harness black-box.
Your harness, in turn, spawns a toy KV fixture to exercise spawn, call, and
assertion logic.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the harness](stages/01-bind.md) | TCP + NDJSON, `ping` |
| 2 | [Spawn a program](stages/02-spawn.md) | `spawn` subprocess, wait for TCP |
| 3 | [Proxy a call](stages/03-call.md) | `call` forwards RPC to child |
| 4 | [Set and get via proxy](stages/04-set-get.md) | `call` set/get on toy KV |
| 5 | [Run one case](stages/05-run-case.md) | `run_case` with subset equality |
| 6 | [Run a suite](stages/06-run-suite.md) | multiple `run_case` calls |
| 7 | [Spawn again](stages/07-respawn.md) | independent second spawn |
| 8 | [Parallel calls](stages/08-concurrent.md) | concurrent `call` RPCs |
| 9 | [The gauntlet](stages/09-gauntlet.md) | isolation + multi-key proxy |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) for the full wire contract. Copy a starter from
[starters/](starters/), then:

```sh
./crafters grade --challenge build-your-own-harness \
    --program path/to/your_program.sh --stage bind
```

A reference solution lives in
[examples/solutions/build-your-own-harness/go/](../../examples/solutions/build-your-own-harness/go/).

The toy KV fixture used in tests is at
[fixtures/toy-kv/go/](fixtures/toy-kv/go/).
