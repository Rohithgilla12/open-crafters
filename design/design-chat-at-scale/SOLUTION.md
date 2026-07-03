# Reference architecture — chat at scale

## Send path

```
Client ──WS──▶ Gateway ──▶ Chat service (shard for conv_id)
                                │
                    1. assign seq (per-conv counter)
                    2. append to conv log
                    3. ACK client
                    4. publish conv:{id} event
                                │
              ┌─────────────────┼─────────────────┐
              ▼                 ▼                 ▼
         Gateway push      Push worker       Indexer (search)
         (online users)    (APNs/FCM)        (async)
```

## Storage

| Store | Contents |
|-------|----------|
| **Conversation log** | `(conv_id, seq) → {sender, body, ts}` append-only |
| **Inbox pointers** | `user_id → [(conv_id, last_seq_read)]` |
| **Membership** | `conv_id → [user_ids]` cached |

Log is source of truth; inbox is derived (can rebuild).

Sharding: `shard = hash(conv_id) % N`. For 500-member groups, single shard still works at stated QPS if log append is O(1).

## Ordering model

**Total order per conversation** via server-assigned `seq`. No clock sync needed.

Cross-conversation order irrelevant. Display merges by conversation list recency.

## Offline catch-up

Client holds `last_seq` per conversation. On reconnect:

```
GET /conv/{id}/messages?after_seq=1842&limit=100
```

Paginate until caught up. Gap detection: if `seq` jumps, client requests range.

## Delivery guarantees

| Stage | Guarantee |
|-------|-----------|
| After server ACK | Durable (WAL + replicate) |
| Push to device | At-least-once (retry push) |
| Client display | Dedupe by `message_id` |

Exactly-once UI = idempotent client rendering.

## Presence

Heartbeat every 30s → Redis `presence:{user_id}=online`. Best-effort; don't block send path.

Typing indicators: pub/sub only, no durability.

## Multi-region

- **Home region** per user for gateway affinity.
- Conversation has **home shard**; cross-region members read via federation (higher latency OK) or replicate hot convs.

Keep it simple in interviews: single region first, mention CRDTs / conflict-free only if asked about offline edits.

## Build challenges mapping

| Piece | Primitive |
|-------|-----------|
| Durable conv log | `build-your-own-log` |
| Delivery queue | `build-your-own-queue` |
| Per-shard leader | `build-your-own-raft` |
| Global message IDs | `build-your-own-id-generator` |

Chat is where **ordering**, **durability**, and **fan-out** meet — the same three themes as your build challenges, at product scale.
