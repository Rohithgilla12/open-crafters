# Build your own payment ledger

**Meta-compose capstone** — you write the **gateway** only. The harness spawns reference
WAL, id-generator, and MVCC processes and injects their addresses into your environment.

## What you build

A TCP gateway that exposes `open_account`, `transfer`, `get_balance`, and
`get_transfer` by coordinating the three child protocols.

## Stages

| # | Stage | What you build |
|---|---|---|
| 1 | bind | Gateway `ping` + stack boots |
| 2 | open | Create accounts |
| 3 | transfer | Move funds |
| 4 | balance | Read balances |
| 5 | insufficient | Reject overdrafts |
| 6 | idempotent | Replay same idempotency key |
| 7 | multi | Several transfers |
| 8 | concurrent | Parallel transfers |
| 9 | gauntlet | Mixed open, transfer, idempotency |

```sh
crafters start payment-ledger
```
