# open-crafters architecture

## Design principle: grade at the process boundary

Every open-crafters challenge is graded **black-box**. The submission is an
executable; the tester spawns it, talks to it over a wire protocol, and
asserts on externally observable behavior — including hostile behavior like
`SIGKILL`ing the process to test durability. The tester never parses, links,
or imports user code.

This single decision is what makes the platform language-agnostic at
near-zero marginal cost: supporting a new language means adding a starter
template, nothing more.

The corollary is a constraint on challenge design: **a stage may only test
what is observable from outside the process.** "Implements X with a B-tree"
is not a gradeable stage; "serves these queries correctly after this crash"
is. When a challenge's interesting machinery is internal (e.g. a
deterministic-replay engine), design the protocol so the machinery's
correctness becomes observable (e.g. "same history in → same commands out").

## Components

```
┌───────────────┐   spawns, kills, restarts   ┌──────────────────┐
│    tester     │ ───────────────────────────▶ │  user submission │
│  (Go binary)  │                              │  (any language)  │
│               │ ◀──────────────────────────▶ │                  │
└───────────────┘    wire protocol (TCP/NDJSON │  --port, --data-dir
        │             or whatever the          └──────────────────┘
        │             challenge specifies)
        ▼
  per-challenge stage tests
  (tester/internal/challenges/<slug>)
```

### `tester/internal/harness` — challenge-agnostic core

- **`Program`**: spawns the submission (`your_program.sh --port P --data-dir D`)
  in its own process group, waits for TCP readiness, streams its output with
  a `[your_program]` prefix, and supports `SIGKILL` + restart with the same
  port and data dir. The data dir outlives restarts within a stage — that's
  the durability contract.
- **`Client`**: a newline-delimited-JSON request/response client with
  request-id checking and descriptive transport-vs-protocol error reporting.
- **`Run`**: executes stages in order up to a target slug, first failure
  stops the run. Each stage gets a fresh process and data dir, so stages are
  independent and a later stage can't pass off earlier state.

### `tester/internal/challenges/<slug>` — stage tests

One package per challenge, exporting `Challenge() harness.Challenge`. Stage
tests are plain Go functions `func(*harness.Context) error`. Conventions:

- Error messages must *teach*: say what was expected, what was observed, and
  when relevant, why it matters ("intermediate failures must NOT be
  recorded").
- Timing assertions use generous lower bounds and slack to stay reliable on
  slow CI machines; never assert tight upper bounds on delivery latency.
- Log progress with `ctx.Logf` so a passing run reads like a narrative.

### `challenges/<slug>/` — the content

`challenge.yaml` (metadata + stage list), `PROTOCOL.md` (the wire contract —
the authoritative spec), `stages/*.md` (per-stage instructions that motivate
*why* before *what*), `starters/<language>/` (templates that pass exactly
stage 1).

### `examples/solutions/` — reference solutions

Every challenge must have at least one reference solution passing all stages.
It is the proof that the protocol is implementable and the tests are fair,
and it doubles as CI for the tester itself.

## Why a non-blocking poll protocol?

Long-polling is what real Temporal does, but it forces every submission to
juggle blocked connections from stage 3 onward. Non-blocking polls
(`{"task": null}` + client-side retry loop) keep the concurrency burden on
the tester, where it's written once, instead of on every learner in every
language. The trade-off (polling latency) is irrelevant at test scale.

## Roadmap

- **Hosted runner** (`cmd/runner`, `crafters submit`): sandboxed Docker grading on
  a VPS — see [hosted-runner.md](hosted-runner.md).
- **GitHub push webhooks** — auto-grade on push with GitHub Checks; see
  [github-webhook.md](github-webhook.md).
- `challenge.yaml`-driven test scaffolding so simple protocol assertions can
  be declared rather than coded.
- Web UI: stage instructions, run logs, leaderboards.
