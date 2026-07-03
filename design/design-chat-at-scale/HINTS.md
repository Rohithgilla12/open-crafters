# Hints — chat at scale

## ACK timing

ACK after **durable append** to the conversation log — not after every device delivers. Client "sent" ✓ means the server won't lose it.

## Shard by conversation_id

All messages for `conv_id` go to one partition / Raft group. Avoids distributed ordering hell. Hot groups are the exception — may need sub-shards or rate limits.

## Ordering

Server assigns monotonic `seq` per conversation (your **ID generator** or simple counter). Clients sort by `seq`. Concurrent sends get total order from the single writer per shard.

## Fan-out

1. Append message to log (single write).
2. Push to online connection registry for each member (async).
3. Offline members read on reconnect via `since_seq`.

## Connection layer

Separate **gateway** (WebSockets) from **chat service** (business logic). Gateway subscribes to pub/sub channel `conv:{id}`.

## open-crafters tie-in

- **Queue** — fan-out delivery work
- **Raft** — per-shard consensus if you need strong ordering across failures
- **ID generator** — `message_id` / snowflake for global debuggability
