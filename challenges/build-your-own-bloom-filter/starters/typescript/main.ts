// Starter template for "Build your own bloom filter" (TypeScript, Bun). Passes stage 1 only.

import { createServer, type Socket } from "node:net";

class BloomFilterError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handle(method: string): unknown {
  if (method === "ping") return { message: "pong" };
  // TODO (stage 2): create filter (m bits, k hashes) + FILTER_EXISTS / INVALID_PARAMS
  // TODO (stage 3): add item via FNV-1a double hash + FILTER_NOT_FOUND
  // TODO (stage 4): contains → maybe_present (all k bits set)
  // TODO (stage 5): sparse filter — never-added items usually false
  // TODO (stage 6): no false negatives under bulk insert
  // TODO (stage 7): independent filters per filter_id
  // TODO (stage 8): all k positions — (h1 + i*h2) % m, not just h1 % m
  // TODO (stage 9): concurrent add/contains + optional delete_filter
  throw new BloomFilterError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
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
        const err = e as BloomFilterError;
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
