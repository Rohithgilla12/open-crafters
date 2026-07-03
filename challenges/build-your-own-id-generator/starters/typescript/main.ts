// Starter for Build your own ID generator (TypeScript, Bun). Passes stage 1 only.

import { createServer, type Socket } from "node:net";

class IDError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function dispatch(method: string): unknown {
  if (method === "ping") return { message: "pong" };
  throw new IDError("UNKNOWN_METHOD", `unknown method ${method}`);
}

function handle(sock: Socket) {
  let buf = "";
  sock.on("data", (chunk) => {
    buf += chunk.toString();
    let idx: number;
    while ((idx = buf.indexOf("\n")) >= 0) {
      const line = buf.slice(0, idx);
      buf = buf.slice(idx + 1);
      if (!line.trim()) continue;
      const req = JSON.parse(line) as { id?: string; method?: string };
      try {
        const result = dispatch(req.method ?? "");
        sock.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as IDError;
        sock.write(
          JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n",
        );
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handle).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
