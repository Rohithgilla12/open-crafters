# Build your own bloom filter

Build a **probabilistic set membership** service — the compact data structure
behind "might this username exist?" checks, database join filters, and CDN
cache priming.

The vehicle is a small TCP server, but the substance is real bloom-filter
behavior:

- **create** filters with `m` bits and `k` hash functions,
- **add** UTF-8 items by setting k bit positions,
- **contains** with no false negatives (and occasional false positives),
- **independent filters** keyed by `filter_id`,
- the exact **FNV-1a double-hash** scheme from the protocol, and
- a final **gauntlet** of concurrent add/lookup churn.

All state is in-memory — no durability stage.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | [Boot the server](stages/01-bind.md) | TCP + newline-delimited JSON, `ping` |
| 2 | [Create a filter](stages/02-create.md) | `create`, errors |
| 3 | [Add an item](stages/03-add.md) | `add` |
| 4 | [Positive lookup](stages/04-positive.md) | `contains` → true for added items |
| 5 | [Negative lookup](stages/05-negative.md) | sparse filter, false positives rare |
| 6 | [No false negatives](stages/06-no-false-negatives.md) | 200 adds, all must probe true |
| 7 | [Independent filters](stages/07-multi-filter.md) | separate `filter_id` state |
| 8 | [K hash functions](stages/08-hash-functions.md) | all k positions, not just h1 |
| 9 | [The gauntlet](stages/09-gauntlet.md) | concurrent connections + churn |

## Getting started

Read [PROTOCOL.md](PROTOCOL.md) for the full wire contract. Copy a starter from
[starters/](starters/), then:

```sh
./crafters grade --challenge build-your-own-bloom-filter \
    --program path/to/your_program.sh --stage bind
```

A reference solution lives in
[examples/solutions/build-your-own-bloom-filter/go/](../../examples/solutions/build-your-own-bloom-filter/go/).
