// Starter URL shortener gateway (TypeScript). Bind only.
import { createServer, type Socket } from "node:net";

class GWError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handleConn(socket: Socket) {
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
          throw new GWError("UNKNOWN_METHOD", "not implemented");
        }
      } catch (e) {
        const err = e as GWError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
