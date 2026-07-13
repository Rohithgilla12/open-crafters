---
title: At-least-once delivery is a visibility timeout
description: Persist before ack, fence with receipts, and redeliver when leases expire — how durable message queues actually work.
date: 2026-07-10
tags: queue, messaging, durability
related: build-your-own-queue, design-distributed-scheduler
---

A durable queue is not “FIFO in memory.” It is a promise: **every message is delivered at least once**, even if consumers crash mid-processing.

## The contract

- **Persist** the message before acknowledging the producer
- Hand out work with a **visibility timeout** (lease) so only one consumer sees it at a time
- **Fence** completions with receipts so stale consumers cannot ack after a redelivery
- Route poison messages to a **dead-letter** path after too many failures

## What we grade

[Build your own message queue](/challenges/build-your-own-queue) talks to your broker over TCP and `SIGKILL`s it mid-flight. Unacked work must reappear; acked work must not. On-disk format is yours — behavior is not.

## Start here

```sh
crafters start queue
```

Then continue the [durability roadmap](/roadmaps/durability): WAL → queue → log → LSM → MVCC → object store.
