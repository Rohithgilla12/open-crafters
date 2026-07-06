# Build your own URL shortener

**Meta-compose capstone** — you write the **gateway** only. The harness spawns reference
id-generator, bloom-filter, and object-store processes and injects their addresses
into your environment.

## What you build

A TCP gateway that exposes `shorten`, `resolve`, and `record_click` by calling the
three child protocols documented in those challenges.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | bind | Gateway `ping` + stack boots |
| 2 | shorten | Mint codes via idgen + bloom + store |
| 3 | resolve | Lookup path |
| 4 | not-found | Miss semantics |
| 5 | analytics | Click logging to object store |
| 6 | bloom | Codes added to filter |
| 7 | multi-url | Several round-trips |
| 8 | concurrent | Unique codes under load |
| 9 | gauntlet | Mixed churn |

```sh
crafters start url-shortener
```

Prerequisites: pass id-generator, bloom-filter, and object-store (or use reference binaries via the harness).
