# Stage 1: Boot the server

Prove your program boots, binds a port, and speaks the wire protocol.

## Your task

Parse `--port` and `--data-dir`, listen on `127.0.0.1:<port>`, and answer
`ping` over newline-delimited JSON.

```
→ {"id":"1","method":"ping","params":{}}
← {"id":"1","result":{"message":"pong"}}
```

## Tests

Two concurrent connections, each pinged a few times. Accept connections within
10 seconds.

## Notes

- `--data-dir` is yours; ignore it until the Durability stage.
- The starter templates already pass this stage.
