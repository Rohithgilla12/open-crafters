---
title: Append-only logs keep offsets absolute
description: Topics, permanent offsets, consumer-group commits, and retention that never renumbers — the Kafka-style log contract.
date: 2026-07-11
tags: log, streaming, kafka
related: build-your-own-log, design-event-streaming
---

Event streams are not queues you drain. They are **append-only logs**: producers write, consumers replay from an offset, and retention deletes old bytes without renumbering history.

## The contract

- Each record gets a **monotonic, absolute offset** (starting at 0) that never moves
- Reads are **replayable** — consuming does not delete
- **Consumer groups** store committed offsets so workers resume after crashes
- **Retention / truncate** may drop a prefix, but surviving offsets stay the same numbers

## What we grade

[Build your own log](/challenges/build-your-own-log) checks append/read, multi-topic isolation, consumer-group commits, retention, and crash survival. The harness restarts your process with the same `--data-dir` and expects the same offsets back.

## Start here

```sh
crafters start log
```

Whiteboard the bigger picture: [Design an event streaming platform](/design/design-event-streaming).
