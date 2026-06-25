// Starter template for "Build your own LSM-tree" (TypeScript, run with Bun).
//
// Boots a TCP server speaking newline-delimited JSON and answers ping — enough
// for stage 1. Extend handleRequest stage by stage. See PROTOCOL.md for the
// wire protocol AND the on-disk SST format (graded!).

import { createServer, type Socket } from "node:net";

class RpcError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handleRequest(method: string, _params: Record<string, unknown>): unknown {
  if (method === "ping") return { message: "pong" };

  // TODO (stage 2): put, get, del — in memory
  // TODO (stage 3): flush memtable to <data-dir>/sst/NNNNNN.sst (SST1 format)
  // TODO (stage 4): recovery — load SST files on startup
  // TODO (stage 5): scan — range query across memtable + SST
  // TODO (stage 6): compact — merge all SST files into one
  // TODO (stage 7): tombstones — value_len=0 on flush after del
  throw new RpcError("UNKNOWN_METHOD", `unknown method ${method}`);
}

function serveSocket(socket: Socket): void {
  let buffer = "";
  socket.setEncoding("utf8");
  socket.on("data", (chunk) => {
    buffer += chunk;
    let idx: number;
    while ((idx = buffer.indexOf("\n")) >= 0) {
      const line = buffer.slice(0, idx).trim();
      buffer = buffer.slice(idx + 1);
      if (!line) continue;
      let requestId: string | undefined;
      try {
        const request = JSON.parse(line) as {
          id?: string;
          method?: string;
          params?: Record<string, unknown>;
        };
        requestId = request.id;
        const result = handleRequest(request.method ?? "", request.params ?? {});
        socket.write(JSON.stringify({ id: requestId, result }) + "\n");
      } catch (e) {
        const err = e as RpcError;
        socket.write(
          JSON.stringify({
            id: requestId,
            error: { code: err.code ?? "BAD_REQUEST", message: String(err.message ?? e) },
          }) + "\n",
        );
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
const dataDir = process.argv[process.argv.indexOf("--data-dir") + 1];
void dataDir;

createServer(serveSocket).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
