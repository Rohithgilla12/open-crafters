# Wire Protocol — Build your own chat service (gateway)

You implement the **messaging gateway** only. The harness spawns three
**reference** services (id-generator, log, queue) and injects their
addresses into your environment before starting your program.

## Your process

```
./your_program.sh --port <port>
```

Environment variables set by the harness:

| Variable | Service |
|----------|---------|
| `IDGEN_ADDR` | Reference `build-your-own-id-generator` |
| `LOG_ADDR` | Reference `build-your-own-log` |
| `QUEUE_ADDR` | Reference `build-your-own-queue` |

Read each child's existing `PROTOCOL.md` in those challenges — speak NDJSON to
those addresses over TCP. Your gateway exposes a **new** protocol below.

## Architecture

Clients call your gateway, not the primitives directly:

1. **`send_message`** → `next_id` on id-generator → `append` on log (topic =
   `conversation_id`) → `send` on queue `delivery`
2. **`read_messages`** → `read` on log for a conversation topic
3. **`poll_delivery`** → `receive` on queue `delivery` (non-blocking)
4. **`ack_delivery`** → `ack` on queue

On first use, call id-generator `configure` with `{"worker_id": 1}` (or any
valid worker id in `0..1023`).

Use queue name **`delivery`** for fan-out notifications.

Message envelope JSON (stored in log and queued):

```json
{
  "message_id": "<from idgen>",
  "conversation_id": "<topic>",
  "sender": "<string>",
  "body": "<string>"
}
```

## Gateway transport

Newline-delimited JSON, same shape as other challenges.

## Gateway methods

### `ping`

- **result:** `{"message": "pong"}`

### `send_message`

Persist a message and enqueue delivery.

- **params:** `{"conversation_id": "<string>", "sender": "<string>", "body": "<string>"}`
- **result:** `{"message_id": "<string>", "offset": <int>}`

`offset` is the log append offset for that conversation topic.

### `read_messages`

Read conversation history from the durable log.

- **params:** `{"conversation_id": "<string>", "offset": <int>, "max": <int>}`
  (`offset` defaults to `0`, `max` defaults to `100`)
- **result:** `{"records": [{"offset": <int>, "value": "<envelope json>"}, ...]}`

Proxy to log `read` with `topic` = `conversation_id`.

### `poll_delivery`

Poll for the next fan-out notification. **Non-blocking.**

- **params:** `{}`
- **result (message available):**
  ```json
  {
    "message": {
      "message_id": "...",
      "conversation_id": "...",
      "sender": "...",
      "body": "...",
      "receipt": "<from queue receive>"
    }
  }
  ```
- **result (idle):** `{"message": null}`

### `ack_delivery`

Remove a delivered message from the queue.

- **params:** `{"receipt": "<string>"}`
- **result:** `{}`

## Error codes

| Code | When |
|------|------|
| `UNKNOWN_METHOD` | Unrecognized gateway method |
| `INVALID_PARAMS` | Missing required fields |

Child service errors may propagate as gateway `INTERNAL`.
