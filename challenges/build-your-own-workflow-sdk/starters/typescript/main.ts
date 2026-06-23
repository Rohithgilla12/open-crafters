// Starter template for "Build your own workflow SDK" (TypeScript, run with Bun).
//
// Boots a TCP server speaking newline-delimited JSON and answers `ping` —
// enough to pass the first stage. Extend handleRequest stage by stage.

import { createServer, type Socket } from "node:net";

class EngineError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function handleRequest(method: string, _params: Record<string, unknown>): unknown {
  if (method === "ping") return { message: "pong" };
  // TODO (stage 2): replay — greet workflow → COMPLETE_WORKFLOW
  // TODO (stage 3): fetch workflow → SCHEDULE_ACTIVITY
  // TODO (stage 4): fetch after ACTIVITY_TASK_COMPLETED → COMPLETE_WORKFLOW
  // TODO (stage 5): waiting states → empty commands
  // TODO (stage 6): timer_wait workflow
  // TODO (stage 7): signal_wait workflow
  // TODO (stage 8): determinism — no randomness or wall clock in replay
  // TODO (stage 9): pipeline workflow (gauntlet)
  throw new EngineError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
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
      const req = JSON.parse(line) as { id?: string; method?: string; params?: Record<string, unknown> };
      try {
        const result = handleRequest(req.method ?? "", req.params ?? {});
        socket.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as EngineError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
const dataDir = process.argv[process.argv.indexOf("--data-dir") + 1];
void dataDir;

createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
