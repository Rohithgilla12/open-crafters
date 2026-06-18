# open-crafters

**Open-source "build your own X" challenges for serious infrastructure.**

In the spirit of [CodeCrafters](https://codecrafters.io) and
[Boot.dev](https://boot.dev) — but aimed at the production-infrastructure
primitives senior engineers actually wrestle with: workflow engines,
write-ahead logs, consensus, schedulers. You implement the system in **any
language**; a test harness grades you stage by stage, entirely over the wire.

## Quickstart

One command installs everything — a prebuilt grader (no Go needed), the
challenge content, and the `crafters` launcher on your PATH:

```sh
curl -fsSL https://raw.githubusercontent.com/Rohithgilla12/open-crafters/main/install.sh | sh
```

Then go from zero to coding in two commands:

```sh
crafters start wal      # scaffold the WAL challenge (a good first one) and grade it
cd my-wal               # edit main.py here
crafters test           # re-grade — resumes exactly where you left off
```

`crafters start` already passes stage 1, so you'll see something like:

```
✓ created ./my-wal from the python starter for "build-your-own-wal"
  protocol: .../challenges/build-your-own-wal/PROTOCOL.md

[stage 1/9] bind — Boot the server
✓ bind passed (0.21s)
[stage 2/9] kv — An in-memory key-value store
✗ stage "kv" failed: get "fruit": UNKNOWN_METHOD ...
Stuck? Re-read the instructions: challenges/build-your-own-wal/stages/02-kv.md
```

That last line is your prompt. The loop from here:

1. **Read** the stage instructions it pointed you at, plus the challenge's
   `PROTOCOL.md` (the authoritative wire spec).
2. **Implement** the next method in your copy (`my-wal/main.py`).
3. **`crafters test`** — re-verifies the stages you've passed, then attempts
   the next one. When a stage fails it tells you exactly what it **expected vs.
   observed**. Fix it and repeat.

### The `crafters` launcher

```sh
crafters start <challenge> [dir] [--lang python|go]   # scaffold a solution + grade it
crafters test  [dir]                                  # re-grade (resume)
crafters status [dir]                                 # progress checklist
crafters list                                         # all challenges and stages
```

`<challenge>` takes a fuzzy name (`wal`, `queue`, `temporal`) or a full slug.
Progress is saved in `<solution>/.open-crafters/progress.json`, so it travels
with your solution — stop and resume whenever.

Your submission is just an executable the grader runs as
`./your_program.sh --port <port> --data-dir <path>`. The grader spawns it,
talks to it over TCP, and even `SIGKILL`s and restarts it to test durability —
but it **never reads your code**. If you speak the protocol, you pass, in any
language with a TCP socket (Python, Go, Rust, Zig, C, Node, …).

### Prefer to run from a clone?

No install needed — `./crafters` works straight from a checkout, building the
grader on first use (needs **Go ≥ 1.21**):

```sh
git clone https://github.com/Rohithgilla12/open-crafters && cd open-crafters
./crafters start wal
```

Or skip the launcher entirely and call the grader directly:

```sh
cd tester && go build ./cmd/tester && cd ..
./tester/tester --challenge build-your-own-wal --program my-wal/your_program.sh
#   --status   checklist    --stage <slug>   up to a stage
#   --all      every stage   --list           all challenges
```

## Challenges

| Challenge | Stages | Status |
|---|---|---|
| [Build your own Temporal](challenges/build-your-own-temporal/) — a durable workflow engine: task queues, event-sourced histories, activity retries, durable timers, signals, crash recovery | 10 | ✅ ready |
| [Build your own WAL](challenges/build-your-own-wal/) — a byte-exact write-ahead log: CRC framing, torn-write recovery, corruption detection, checkpointing | 9 | ✅ ready |
| [Build your own message queue](challenges/build-your-own-queue/) — a durable broker: at-least-once delivery, visibility timeouts, receipt fencing, dead-letter queues | 9 | ✅ ready |
| Build your own workflow SDK — deterministic replay, the other half of Temporal | — | planned |
| Build your own Raft | — | planned |

### Which challenge should I start with?

All three are meaty, but they build on a shared idea — **make a promise
durable before you acknowledge it** — in increasing order of scope:

1. **Build your own WAL** — *start here.* The most self-contained of the
   three, and it teaches the write-before-ack discipline the other two lean
   on. You get crisp, byte-level feedback (the tester parses and crafts your
   log), so mistakes are concrete and easy to localize.
2. **Build your own message queue** — next. Reuses that durability discipline
   but the lesson is *delivery semantics*: at-least-once, visibility timeouts,
   and the receipt-fencing bug that quietly loses work in real systems. Graded
   purely by behavior, so you choose your own on-disk format.
3. **Build your own Temporal** — the biggest. A full durable workflow engine —
   task queues, event-sourced histories, retries, durable timers, crash
   recovery. Easier to reason about once durability (WAL) and queue mechanics
   (message queue) are already second nature.

Each is independent — you can start with whichever system you most want to
understand. Stage 1 of any of them is a 30-minute "boot and answer a ping," so
it's cheap to try one and switch.

## Repository layout

```
crafters                    the launcher (start / test / status / list)
install.sh                  one-line installer (curl … | sh)
challenges/<slug>/          challenge definition
  challenge.yaml            metadata + ordered stage list
  PROTOCOL.md               the wire contract (the real spec)
  stages/NN-<slug>.md       per-stage instructions
  starters/<language>/      minimal templates that pass stage 1
examples/solutions/<slug>/  reference solutions that pass all stages
tester/                     the Go test harness (the grader)
  internal/harness/         challenge-agnostic core (process mgmt, protocol client, runner)
  internal/challenges/      per-challenge stage tests
docs/                       architecture & challenge-authoring guides
```

## Contributing

New challenges welcome — see [docs/authoring-challenges.md](docs/authoring-challenges.md).
The bar: every stage must be gradeable from outside the process, with failure
messages that teach.
