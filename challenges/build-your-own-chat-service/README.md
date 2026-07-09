# Build your own chat service

**Meta-compose capstone** — you write the **gateway** only. The harness spawns reference
id-generator, append log, and delivery queue services.

## What you build

A messaging gateway: `send_message`, `read_messages`, `poll_delivery`, and `ack_delivery`
over a durable per-conversation log with async fan-out.

## Environment

| Variable | Service |
|----------|---------|
| `IDGEN_ADDR` | Reference snowflake ID generator |
| `LOG_ADDR` | Reference append log |
| `QUEUE_ADDR` | Reference message queue |

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | bind | Gateway `ping` + stack boots |
| 2 | send | Mint ID, append log, enqueue delivery |
| 3 | read | Read conversation history |
| 4 | delivery | Fan-out via queue `delivery` |
| 5 | ack | Ack removes queue message |
| 6 | ordering | Log preserves append order |
| 7 | two-chats | Topics isolate conversations |
| 8 | concurrent | Parallel sends |
| 9 | gauntlet | Send, deliver, ack, multi-read |

```sh
crafters start chat-service
```
