# open-crafters

**Open-source "build your own X" challenges for serious infrastructure.**

In the spirit of [CodeCrafters](https://codecrafters.io) and
[Boot.dev](https://boot.dev) — but aimed at the production-infrastructure
primitives senior engineers actually wrestle with: workflow engines,
write-ahead logs, consensus, schedulers. You implement the system in **any
language**; a test harness grades you stage by stage, entirely over the wire.

## How it works

1. Pick a challenge and read its `PROTOCOL.md` — the wire contract your
   program must speak.
2. Start from a starter template (or from scratch). Your submission is just
   an executable: `./your_program.sh --port <port> --data-dir <path>`.
3. Run the tester. It spawns your program, drives it through staged tests —
   including killing it to test durability — and tells you exactly what's
   wrong when a stage fails.

```sh
# build the tester (once)
cd tester && go build ./cmd/tester && cd ..

# copy a starter and start hacking
cp -r challenges/build-your-own-temporal/starters/python my-solution

# run it — the tester resumes automatically: it re-verifies the stages
# you've passed and attempts the next one, then points you at the next
# stage's instructions
./tester/tester --challenge build-your-own-temporal \
    --program my-solution/your_program.sh

# see your progress checklist
./tester/tester --challenge build-your-own-temporal \
    --program my-solution/your_program.sh --status

# other modes
#   --stage <slug>   run up to and including a specific stage
#   --all            run every stage
```

Progress is tracked in `<your solution dir>/.open-crafters/progress.json`,
so it travels with your solution repo.

The tester never reads your code. If you speak the protocol, you pass — in
Python, Go, Rust, Zig, or anything that can open a TCP socket.

## Challenges

| Challenge | Stages | Status |
|---|---|---|
| [Build your own Temporal](challenges/build-your-own-temporal/) — a durable workflow engine: task queues, event-sourced histories, activity retries, durable timers, signals, crash recovery | 10 | ✅ ready |
| [Build your own WAL](challenges/build-your-own-wal/) — a byte-exact write-ahead log: CRC framing, torn-write recovery, corruption detection, checkpointing | 9 | ✅ ready |
| [Build your own message queue](challenges/build-your-own-queue/) — a durable broker: at-least-once delivery, visibility timeouts, receipt fencing, dead-letter queues | 9 | ✅ ready |
| Build your own workflow SDK — deterministic replay, the other half of Temporal | — | planned |
| Build your own Raft | — | planned |

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
