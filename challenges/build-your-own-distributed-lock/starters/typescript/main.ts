// Starter template for "Build your own distributed lock" (TypeScript, Bun). Passes stage 1 only.

import { createServer, type Socket } from "node:net";

class DistLockError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handle(method: string): unknown {
  if (method === "ping") return { message: "pong" };
  // TODO (stage 2): acquire + status
  // TODO (stage 3): release with token check
  // TODO (stage 4): LOCK_HELD on contended acquire
  // TODO (stage 5): try_acquire (acquired:false, no error on contention)
  // TODO (stage 6): lazy expiry via expires_at_ms
  // TODO (stage 7): renew + NOT_HOLDER
  // TODO (stage 8): persist active locks to --data-dir
  throw new DistLockError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
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
        const err = e as DistLockError;
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
