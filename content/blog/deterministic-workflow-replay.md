---
title: Deterministic replay — the other half of Temporal
description: Same event history in, same commands out. Why workflow code must be deterministic and how the worker SDK is graded.
date: 2026-07-06
tags: workflow, sdk, replay
related: build-your-own-workflow-sdk, build-your-own-temporal, build-your-own-workflow-worker
---

A workflow server can store perfect histories and still produce wrong results if the **worker** is nondeterministic. Replay is the contract that closes that gap.

## The rule

Given history `H`, workflow code must emit the same command sequence every time: schedule activity A, set timer T, complete with result R. If `now()`, `rand()`, or a network call sneaks into workflow code, two workers diverge and the system cannot recover safely.

## How open-crafters tests it

[Build your own workflow SDK](/challenges/build-your-own-workflow-sdk) feeds crafted histories and asserts the command stream. Activities, timers, and signals are all in scope — pure functions over event logs.

## Full stack

1. Server: [Temporal](/challenges/build-your-own-temporal)
2. Replay: [Workflow SDK](/challenges/build-your-own-workflow-sdk)
3. Gateway: [Workflow worker](/challenges/build-your-own-workflow-worker)

```sh
crafters start workflow-sdk
```
