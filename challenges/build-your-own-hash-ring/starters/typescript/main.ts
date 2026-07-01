// Starter template for "Build your own hash ring" (TypeScript, Bun). Passes stage 1 only.

import { createServer, type Socket } from "node:net";

class HashRingError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handle(method: string): unknown {
  if (method === "ping") return { message: "pong" };
  // TODO (stage 2): create_ring + RING_EXISTS / INVALID_PARAMS
  // TODO (stage 3): add_node, lookup + RING_NOT_FOUND / NODE_EXISTS / NO_NODES
  // TODO (stage 4): deterministic FNV-1a vnode walk per PROTOCOL.md
  // TODO (stage 5): even key spread across 3 nodes
  // TODO (stage 6): add 4th node — fewer than 45% of keys move
  // TODO (stage 7): remove_node — keys remap, never return removed node
  // TODO (stage 8): replicas flatten load (virtual nodes)
  // TODO (stage 9): concurrent add/remove/lookup across 2 rings
  throw new HashRingError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
}

function handleConn(socket: Socket): void {
  let buf = "";
  socket.setEncoding("utf8");
  socket.on("data", (chunk) => {
    buf += chunk;
    let idx: number;
    while ((idx = buf.indexOf("\n")) !== -1) {
      const line = buf.slice(0, idx);
      buf = buf.slice(idx + 1);
      if (!line.trim()) continue;
      const req = JSON.parse(line) as { id?: string; method?: string };
      try {
        const result = handle(req.method ?? "");
        socket.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as HashRingError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const portIdx = process.argv.indexOf("--port");
const port = Number(process.argv[portIdx + 1]);
void process.argv[process.argv.indexOf("--data-dir") + 1]; // ignored
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
