# Authoring a challenge

The bar for an open-crafters challenge: **a senior engineer should come out
the other side understanding how a production system actually works** — not
how to use it, how to *build* it.

## 1. Pick a system with an observable contract

The non-negotiable: every stage must be gradeable from outside the process
(see [architecture.md](architecture.md)). Good candidates have a natural wire
protocol or file format: brokers, databases, schedulers, proxies, consensus
nodes. If the interesting part is in-process (a replay engine, a query
planner), first design a protocol that makes its behavior observable.

## 2. Design the stage ladder

Aim for 8–12 stages with this shape:

1. **Stage 1 is a hello-world**: boot, bind, answer a ping. Everyone passes
   in 30 minutes and has a working skeleton.
2. **Each stage adds one concept** and builds on the previous data model. If
   a stage requires throwing away the previous stage's design, the ladder is
   wrong.
3. **The hard ideas get their own stages.** In the Temporal challenge,
   "empty completion must not re-deliver" (stage 4) and "claims die, tasks
   don't" (stage 8) are the two insights — each gets a dedicated stage and
   dedicated test assertions.
4. **The final stage tests integration under stress** (concurrency,
   interleaving, crashes) with nothing new to implement — it catches design
   shortcuts that survived the happy path.

## 3. Write PROTOCOL.md first

It is the authoritative spec; stage instructions only motivate and sequence
it. Specify exact field names, error codes, and edge-case semantics (what
*must not* happen is as important as what must). Prefer newline-delimited
JSON over TCP unless the system's essence demands otherwise — it's
implementable in any language's stdlib in minutes.

## 4. Implement tests + reference solution together

Add a package under `tester/internal/challenges/<slug>` and register it in
`cmd/tester/main.go`. Write the reference solution in parallel — it will
find every ambiguity in your protocol before learners do. A challenge ships
only when the reference solution passes all stages.

Test-writing conventions:

- **Error messages teach.** `"attempt 2 was delivered after 3ms — too early
  for the backoff schedule (expected ≥ 150ms)"`, not `"timing assertion
  failed"`.
- **Be lenient where the spec is silent** (extra JSON fields, message text),
  exact where it speaks (field names, error codes, event ordering).
- **Timing: lower bounds with slack, generous upper bounds.** Flaky graders
  destroy trust faster than anything else.
- **Test the negative space**: claimed tasks not re-delivered, no workflow
  task while an activity is outstanding, no event for a retried failure.

## 5. Write the prose

- `challenge.yaml` — metadata + ordered stages (see the Temporal one).
- `stages/NN-<slug>.md` — for each stage: why this exists in real systems,
  the task with wire examples, what the tester checks, and hints that point
  at the right *design* without handing over code.
- `starters/<language>/` — must pass exactly stage 1, with TODO markers
  mapping stages to methods.
- `WALKTHROUGH.md` (optional but encouraged) — one `## <stage-slug> — <title>`
  section per stage, each opening with a `> **Hint:**` blockquote (a
  spoiler-free nudge) followed by design-level prose on how the reference
  solves it. `crafters hint` surfaces the blockquote (and the grader prints it
  inline on a failed stage); `crafters walkthrough` prints the full sections
  after a learner passes. If you ship one it must cover **every** stage with a
  hint — `go test ./cmd/crafters` enforces this.

## Checklist before merging

- [ ] Reference solution passes all stages (`tester --program <ref>`)
- [ ] Each starter passes stage 1 and fails stage 2 with a clear message
- [ ] PROTOCOL.md covers every method/field/error code the tests assert on
- [ ] No test asserts on anything PROTOCOL.md doesn't specify
- [ ] Stage instructions explain *why* before *what*
