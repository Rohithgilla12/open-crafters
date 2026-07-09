// Starter for chat service gateway (TypeScript). Passes bind only.
import { createServer, type Socket } from "node:net";

class GWError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handle(method: string, _params: Record<string, unknown>) {
  if (method === "ping") return { message: "pong" };
  process.env.IDGEN_ADDR;
  process.env.LOG_ADDR;
  process.env.QUEUE_ADDR;
  // TODO: send_message, read_messages, poll_delivery, ack_delivery
  throw new GWError("UNKNOWN_METHOD", `unknown method ${method}`);
}

function handleConn(sock: Socket) {
  let buf = "";
  sock.setEncoding("utf8");
  sock.on("data", (chunk) => {
    buf += chunk;
    let idx: number;
    while ((idx = buf.indexOf("\n")) !== -1) {
      const line = buf.slice(0, idx);
      buf = buf.slice(idx + 1);
      const req = JSON.parse(line) as { id?: string; method: string; params?: Record<string, unknown> };
      try {
        const result = handle(req.method, req.params ?? {});
        sock.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const ge = e as GWError;
        sock.write(JSON.stringify({ id: req.id, error: { code: ge.code, message: ge.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1] ?? 0);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
