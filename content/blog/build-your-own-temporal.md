---
title: Build a Temporal-style workflow engine from scratch
description: Event histories, task queues, durable timers, and crash recovery — what a workflow server owns versus the worker SDK.
date: 2026-07-08
tags: workflow, temporal, orchestration
related: build-your-own-temporal, design-workflow-platform, build-your-own-workflow-sdk
---

Workflow engines look magical until you split them in two: a **durable server** that owns histories and timers, and a **deterministic SDK** that replays code against those histories.

## What the server owns

- Workflow and activity **task queues**
- An append-only **event history** per run
- **Timers** that wake workflows later
- Leases so crashed workers do not double-complete work

[Build your own Temporal](/challenges/build-your-own-temporal) grades exactly that surface over TCP — including SIGKILL mid-activity.

## What the SDK owns

Given the same history, emit the same commands every time. No wall-clock, no random, no raw I/O inside workflow code. That is [Build your own workflow SDK](/challenges/build-your-own-workflow-sdk).

## Wire them together

The [workflow worker](/challenges/build-your-own-workflow-worker) compose challenge polls reference Temporal, replays on the SDK, and completes tasks until workflows finish.

Whiteboard first: [Design a workflow orchestration platform](/design/design-workflow-platform).
