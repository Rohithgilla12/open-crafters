# Design a payment ledger

Design a **ledger service** for internal wallets or neobank accounts: users hold balances, transfer money, and auditors need a perfect history.

No credit card processing — focus on **ledger correctness** under crashes and retries.

## Functional requirements

1. **Create account** — `account_id` with currency, starting balance 0.
2. **Deposit / withdraw** — single-sided entries (external funding).
3. **Transfer** — move amount A→B atomically; reject if insufficient funds.
4. **Balance query** — current balance + optional pending holds.
5. **Statement** — paginated history of entries for an account.

## Non-functional

| Dimension | Target |
|-----------|--------|
| Transfers / sec | 5k sustained, 20k burst |
| Correctness | No negative balances, no lost money |
| Idempotency | Client `idempotency_key` on every transfer |
| Audit | Immutable history; no silent edits |
| Durability | Committed = survives datacenter loss |

## Constraints

- Network retries are guaranteed — same transfer request may arrive twice.
- Partial failures happen — crash after debit, before credit.
- Regulators care about **double-entry** — every debit has matching credit.

## Your task

Whiteboard **40–50 minutes**:

1. Schema: accounts, transactions, entries (or equivalent).
2. Transfer algorithm step-by-step in one DB transaction (or equivalent).
3. Idempotency table design.
4. Crash recovery story for in-flight transfers.
5. How MVCC / snapshot reads interact with balance queries.

## Stretch

- Multi-currency with FX (separate problem — mention boundaries).
- Pending two-phase transfers (authorize / capture).
