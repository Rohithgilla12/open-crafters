# Design chat at scale

Design **WhatsApp / Discord-style messaging**: 1:1 chats, group channels, delivery to online and offline clients.

## Functional requirements

1. **Send message** — text (+ optional media metadata); appears in conversation for all members.
2. **Receive** — real-time push to online devices; pull history for offline catch-up.
3. **Ordering** — messages in a conversation appear in a consistent order for all participants.
4. **Read receipts** — optional, best-effort.
5. **Groups** — up to 500 members per channel; 1:1 is the common case.
6. **Presence** — online / last-seen (can be approximate).

## Scale

| Metric | Value |
|--------|-------|
| DAU | 50M |
| Messages / day | 5B (~58k msg/sec avg) |
| Peak | 300k msg/sec |
| Avg group size | 8 |
| Connections | 10M concurrent WebSockets |

## Non-functional

- Delivery latency p99 **&lt; 500ms** for online users in-region.
- Messages **never lost** once server ACKs send.
- Multi-region with home region per user.

## Your task

Whiteboard **30–45 minutes**:

1. **Send ACK path** — when is it safe to tell the client "sent"?
2. **Shard key** — how conversations map to servers.
3. **Ordering** in groups with concurrent senders.
4. **Offline sync** — client returns after 3 days offline.
5. **Fan-out** to N group members on one send.

## Stretch

- End-to-end encryption — what changes in your design?
- Editing / deleting messages.
