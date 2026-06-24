// Starter template for "Build your own scheduler" (TypeScript, Bun). Passes stage 1 only.

import { createServer, type Socket } from "node:net";

class SchedulerError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handle(method: string): unknown {
  if (method === "ping") return { message: "pong" };
  throw new SchedulerError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
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
        const err = e as SchedulerError;
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
