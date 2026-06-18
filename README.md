# open-crafters

**Open-source "build your own X" challenges for serious infrastructure.**

In the spirit of [CodeCrafters](https://codecrafters.io) and
[Boot.dev](https://boot.dev) — but aimed at the production-infrastructure
primitives senior engineers actually wrestle with: workflow engines,
write-ahead logs, consensus, schedulers. You implement the system in **any
language**; a test harness grades you stage by stage, entirely over the wire.

## Prerequisites

- **Go ≥ 1.21** — to build the tester (the grader). It's the only fixed
  dependency; you don't write any Go unless you want to.
- **Any language that can open a TCP socket** — to write your solution.
  Python and Go starters ship with every challenge; Rust, Zig, C, Node,
  anything works.

## Quickstart

Build the grader once, copy a starter into your own directory, and run it.
This example uses **Build your own WAL** — a good first challenge (see
[below](#which-challenge-should-i-start-with)).

```sh
# 1. build the grader (once)
cd tester && go build ./cmd/tester && cd ..

# 2. copy a starter into your own working directory
cp -r challenges/build-your-own-wal/starters/python my-wal

# 3. grade it
./tester/tester --challenge build-your-own-wal --program my-wal/your_program.sh
```

The starter already passes stage 1, so you'll see something like:

```
[stage 1/9] bind — Boot the server
✓ bind passed (0.21s)

Next up: kv — An in-memory key-value store
Instructions: challenges/build-your-own-wal/stages/02-kv.md
```

That last line is your prompt. The loop from here:

1. **Read** the stage instructions it pointed you at, plus the challenge's
   `PROTOCOL.md` (the authoritative wire spec).
2. **Implement** the next method in your copy (`my-wal/main.py`).
3. **Re-run the same command.** The tester re-verifies the stages you've
   already passed, then attempts the next one — no flags needed to advance.
4. When a stage fails, it tells you exactly what it **expected vs. observed**.
   Fix it and repeat.

Your submission is just an executable the tester runs as
`./your_program.sh --port <port> --data-dir <path>`. The tester spawns it,
talks to it over TCP, and even `SIGKILL`s and restarts it to test durability —
but it **never reads your code**. If you speak the protocol, you pass, in any
language.

Progress is saved in `my-wal/.open-crafters/progress.json`, so it travels with
your solution repo — stop and resume whenever.

### Tester modes

```sh
--status            # print your progress checklist and exit (doesn't run anything)
--stage <slug>      # run up to and including one stage
--all               # run every stage from scratch
--list              # list all challenges and their stages
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
challenges/<slug>/          challenge definition
  challenge.yaml            metadata + ordered stage list
  PROTOCOL.md               the wire contract (the real spec)
  stages/NN-<slug>.md       per-stage instructions
  starters/<language>/      minimal templates that pass stage 1
examples/solutions/<slug>/  reference solutions that pass all stages
tester/                     the Go test harness
  internal/harness/         challenge-agnostic core (process mgmt, protocol client, runner)
  internal/challenges/      per-challenge stage tests
docs/                       architecture & challenge-authoring guides
```

## Contributing

New challenges welcome — see [docs/authoring-challenges.md](docs/authoring-challenges.md).
The bar: every stage must be gradeable from outside the process, with failure
messages that teach.
