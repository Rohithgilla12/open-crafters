// Starter template for "Build your own object store" (TypeScript, Bun). Passes stage 1 only.

import { createServer, type Socket } from "node:net";

class ObjectStoreError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handle(method: string): unknown {
  if (method === "ping") return { message: "pong" };
  // TODO (stage 2): put / get with SHA-256 etags
  // TODO (stage 3): head — metadata only
  // TODO (stage 4): overwrite on put
  // TODO (stage 5): delete
  // TODO (stage 6): list by prefix
  // TODO (stage 7): multipart upload
  // TODO (stage 8): persist to --data-dir
  throw new ObjectStoreError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
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
        const err = e as ObjectStoreError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
void process.argv[process.argv.indexOf("--data-dir") + 1];
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
