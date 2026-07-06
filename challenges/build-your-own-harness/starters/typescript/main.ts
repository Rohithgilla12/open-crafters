// Starter for "Build your own harness" (TypeScript, Bun). Passes stage 1 only.

import { createServer, type Socket } from "node:net";

class HarnessError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
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
        if (req.method === "ping") {
          socket.write(JSON.stringify({ id: req.id, result: { message: "pong" } }) + "\n");
        } else {
          throw new HarnessError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(req.method)}`);
        }
      } catch (e) {
        const err = e as HarnessError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
