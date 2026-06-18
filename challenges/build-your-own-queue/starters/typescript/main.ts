// Starter template for "Build your own message queue" (TypeScript, run with Bun).
//
// Boots a TCP server speaking newline-delimited JSON and answers `ping` —
// enough to pass the first stage. Extend handleRequest stage by stage.
// See PROTOCOL.md for the wire protocol and the delivery model (the real spec).

import { createServer, type Socket } from "node:net";

interface Request {
  id?: string;
  method?: string;
  params?: Record<string, unknown>;
}

class RpcError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handleRequest(method: string, params: Record<string, unknown>): unknown {
  switch (method) {
    case "ping":
      return { message: "pong" };

    // TODO (stage 2): send / receive / ack — an in-memory FIFO queue
    // TODO (stage 3): persist to --data-dir; un-acked messages survive SIGKILL
    // TODO (stage 4): visibility timeouts — redeliver if not acked in time
    // TODO (stage 5): nack — make a message visible again immediately
    // TODO (stage 6): receipt fencing — a receipt is valid for one delivery
    // TODO (stage 7): configure + dead-letter after max_receives failures
    // TODO (stage 8): many independent queues + stats

    default:
      throw new RpcError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
  }
}

function handleConnection(socket: Socket): void {
  let buffer = "";
  socket.on("data", (chunk) => {
    buffer += chunk.toString("utf8");
    let nl: number;
    while ((nl = buffer.indexOf("\n")) >= 0) {
      const line = buffer.slice(0, nl);
      buffer = buffer.slice(nl + 1);
      if (!line.trim()) continue;
      const req = JSON.parse(line) as Request;
      let response: unknown;
      try {
        response = { id: req.id, result: handleRequest(req.method ?? "", req.params ?? {}) };
      } catch (e) {
        const err = e instanceof RpcError ? e : new RpcError("BAD_REQUEST", String(e));
        response = { id: req.id, error: { code: err.code, message: err.message } };
      }
      socket.write(JSON.stringify(response) + "\n");
    }
  });
}

function parseArgs(): { port: number; dataDir: string } {
  const args = process.argv.slice(2);
  let port = 0;
  let dataDir = "";
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--port") port = Number(args[++i]);
    else if (args[i] === "--data-dir") dataDir = args[++i];
  }
  return { port, dataDir };
}

const { port } = parseArgs(); // dataDir is yours from the persistence stage on
createServer(handleConnection).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
