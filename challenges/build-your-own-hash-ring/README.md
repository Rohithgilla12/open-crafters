# Build your own hash ring

Build a **consistent hash ring** — the placement primitive behind distributed
caches, shard routers, and any system that adds nodes without reshuffling every
key.

The vehicle is a small TCP server, but the substance is real ring behavior:

- **create_ring** with virtual-node count per physical node,
- **add_node** / **remove_node** for membership changes,
- **lookup** via clockwise FNV-1a vnode walk,
- **list_nodes** for inspection,
- the exact **hash specification** from the protocol, and
- a final **gauntlet** of concurrent churn across two rings.

All state is in-memory — no durability stage.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + newline-delimited JSON, `ping` |
| 2 | [Create a ring](stages/02-create.md) | `create_ring`, errors |
| 3 | [Add a node](stages/03-add-node.md) | `add_node`, single-node lookup |
| 4 | [Deterministic lookup](stages/04-deterministic.md) | same key → same node |
| 5 | [Key spread](stages/05-spread.md) | even distribution across nodes |
| 6 | [Add node (minimal move)](stages/06-minimal-move.md) | consistent hashing on scale-out |
| 7 | [Remove a node](stages/07-remove-node.md) | keys remap to survivors |
| 8 | [Virtual nodes](stages/08-virtual-nodes.md) | replicas flatten imbalance |
| 9 | [The gauntlet](stages/09-gauntlet.md) | concurrent multi-ring churn |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) for the full wire contract. Copy a starter from
[starters/](starters/), then:

```sh
./crafters grade --challenge build-your-own-hash-ring \
    --program path/to/your_program.sh --stage bind
```

A reference solution lives in
[examples/solutions/build-your-own-hash-ring/go/](../../examples/solutions/build-your-own-hash-ring/go/).
