# Stage 10: Concurrent workflows

The final stage. A real engine doesn't run one workflow at a time — it
multiplexes thousands over a shared task queue, keeping every execution's
state isolated. This stage hunts for the bugs that only show up under
interleaving: crossed task tokens, shared history lists, results delivered to
the wrong workflow.

## Your task

Nothing new to implement — everything this stage tests, you've already built.
The question is whether you built it *per-workflow*.

## Tests

The tester starts **5 workflows** on the same task queue, each with a
distinct input, then deliberately interleaves:

1. claims all 5 first workflow tasks **before completing any** (your server
   must track 5 outstanding claims at once),
2. schedules one activity per workflow, claims all 5 activity tasks, then
   completes them in arbitrary order,
3. completes every workflow with a result derived from its own input.

It verifies that each workflow's history contains only its own events, every
activity got the input of *its* workflow, no task was delivered twice, and
each result matches.

## Notes

- Common failure modes: a single global "current workflow task" variable
  (works for 9 stages!), task tokens that don't encode which claim they
  belong to, history lists aliased between workflow records.
- If this stage passes on your first try, your data model was right all
  along. Congratulations — you've built a durable workflow engine. The
  sequel challenge, *Build your own workflow SDK*, puts you on the other
  side of this protocol: deterministic replay against the histories your
  server just learned to serve.
