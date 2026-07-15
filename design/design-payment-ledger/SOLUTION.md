# Reference architecture — payment ledger

## Schema

```
accounts(id, currency, balance_cached, version)
transactions(id, idempotency_key, status, created_at)
entries(id, transaction_id, account_id, amount, direction)
idempotency(client_key, response_json, transaction_id)
```

`amount` signed: negative = debit, positive = credit. Or separate debit/credit columns.

## Transfer flow

```
POST /transfer {from, to, amount, idempotency_key}
  1. Lookup idempotency_key → if hit, return cached response
  2. BEGIN
  3. SELECT accounts WHERE id IN (from,to) FOR UPDATE (order by id)
  4. IF balance[from] < amount → ROLLBACK, insufficient funds
  5. INSERT transaction(status=committed)
  6. INSERT entries (debit from, credit to)
  7. UPDATE balances
  8. INSERT idempotency row
  9. COMMIT → ACK client
```

## Crash cases

| Crash point | Recovery |
|-------------|----------|
| Before commit | Transaction invisible; client retries safely via idempotency |
| After commit | Retry returns same result |
| Mid-commit | DB atomicity — all or nothing |

## Audit trail

Entries are **append-only**. Corrections = new compensating transaction, never UPDATE on entries.

## Read path

- **Balance**: read cached column (validated periodically) or `SUM(entries)`.
- **Statement**: `entries WHERE account_id=? ORDER BY id DESC LIMIT N` — indexed.

## Scaling

- Shard by `account_id` hash — transfers within shard are local; cross-shard transfers need 2PC or saga (mention as v2).
- Hot accounts (treasury): serial queue per account — your **queue** instinct.

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Append-before-ack | `build-your-own-wal` |
| Snapshot balance reads | `build-your-own-mvcc` |
| Transaction IDs | `build-your-own-id-generator` |
| Compose gateway | `build-your-own-payment-ledger` |

Money is the ultimate durability test — the ledger is a WAL with accounting semantics, wired through the compose gateway.
