# Wire Protocol — Build your own payment ledger (gateway)

You implement the **ledger gateway** only. The harness spawns three
**reference** services (WAL, id-generator, MVCC) and injects their
addresses into your environment before starting your program.

## Your process

```
./your_program.sh --port <port>
```

Environment variables set by the harness:

| Variable | Service |
|----------|---------|
| `WAL_ADDR` | Reference `build-your-own-wal` |
| `IDGEN_ADDR` | Reference `build-your-own-id-generator` |
| `MVCC_ADDR` | Reference `build-your-own-mvcc` |

Read each child's existing `PROTOCOL.md` — speak NDJSON to those addresses.
Your gateway exposes a **new** protocol below.

## Architecture

1. **Balances** live in MVCC under keys `bal:<account_id>` (string integer cents).
2. **Transfer records** and **idempotency** live in the WAL:
   - `xfer:<transfer_id>` → JSON envelope
   - `idem:<idempotency_key>` → `transfer_id` string
3. **Transfer IDs** come from id-generator `next_id` (call `configure` with
   `{"worker_id": 1}` once before first mint).

Transfer envelope:

```json
{
  "transfer_id": "<string>",
  "from_account": "<string>",
  "to_account": "<string>",
  "amount": <int>,
  "idempotency_key": "<string>"
}
```

Amounts are positive integers (cents). Debit `from`, credit `to`.

## Gateway methods

### `ping`
- **result:** `{"message": "pong"}`

### `open_account`
Create an account with an initial balance.
- **params:** `{"account_id": "<string>", "balance": <int>}`
- **result:** `{}`
- MVCC: `begin` → `set bal:<account_id>` → `commit`. Reject if the account
  already exists (`found=true` on a prior get) with error `ACCOUNT_EXISTS`.

### `get_balance`
- **params:** `{"account_id": "<string>"}`
- **result:** `{"balance": <int>, "found": true}` or `{"balance": 0, "found": false}`
- MVCC snapshot read (`begin` → `get` → `rollback`).

### `transfer`
Move funds atomically.
- **params:** `{"from_account": "...", "to_account": "...", "amount": <int>, "idempotency_key": "..."}`
- **result:** `{"transfer_id": "<string>", "replayed": false}` on a new transfer
- **result (idempotent replay):** `{"transfer_id": "<string>", "replayed": true}`

Flow for a new transfer:
1. WAL `get` on `idem:<key>` — if found, return that transfer_id with `replayed=true`
2. Mint `transfer_id` via idgen
3. MVCC `begin`; read both balances; if from missing/`found=false` or balance < amount → `rollback` and error `INSUFFICIENT_FUNDS` (or `ACCOUNT_NOT_FOUND` if either account missing)
4. `set` both new balances; `commit` (on `CONFLICT`, surface as `CONFLICT`)
5. WAL `set` `xfer:<id>` to envelope JSON and `idem:<key>` to transfer_id
6. Return transfer_id

### `get_transfer`
- **params:** `{"transfer_id": "<string>"}`
- **result:** `{"found": true, "transfer": {…envelope…}}` or `{"found": false, "transfer": null}`
- WAL `get` on `xfer:<transfer_id>` and parse JSON.

## Error codes

| Code | When |
|------|------|
| `UNKNOWN_METHOD` | Unrecognized method |
| `INVALID_PARAMS` | Missing/invalid fields; `amount` must be > 0; accounts must differ |
| `ACCOUNT_EXISTS` | `open_account` on existing id |
| `ACCOUNT_NOT_FOUND` | Transfer references unknown account |
| `INSUFFICIENT_FUNDS` | Debit would go negative |
| `CONFLICT` | MVCC commit conflict |
