# Hints — payment ledger

## Double-entry

Every transfer creates **two entries** (debit A, credit B) under one `transaction_id`. Sum of entries per currency = 0.

Never update `balance` without an entry — or make `balance` a cached aggregate rebuilt from entries.

## Idempotency

`idempotency_keys(client_key, account_id) → transaction_id, response`.

On retry, return stored response without re-executing.

## Atomicity

Single DB transaction:
1. Lock accounts (ordered by ID to prevent deadlock).
2. Check balances.
3. Insert transaction + entries.
4. Update balance snapshots.
5. Commit.

Your **WAL** discipline: fsync before ACK.

## MVCC angle

Balance reads at a timestamp = sum entries visible to that snapshot — useful for statements without blocking writes.

## open-crafters tie-in

- **WAL** — append entries before acknowledging transfer
- **MVCC** — snapshot-isolated balance reads
- **ID generator** — globally unique `transaction_id`
