// Reference solution for "Build your own workflow SDK" (TypeScript, run with Bun).
//
// A deterministic workflow replay engine. Passes all 9 stages.

import { createServer, type Socket } from "node:net";

class EngineError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

interface Event {
  event_id: number;
  type: string;
  attributes: Record<string, unknown>;
}

type Command = { type: string; attributes: Record<string, unknown> };

const KNOWN = new Set(["greet", "fetch", "timer_wait", "signal_wait", "pipeline"]);

function validateHistory(history: Event[]): void {
  if (history.length === 0) {
    throw new EngineError("INVALID_HISTORY", "history must not be empty");
  }
  for (let i = 0; i < history.length; i++) {
    if (history[i].event_id !== i + 1) {
      throw new EngineError(
        "INVALID_HISTORY",
        `event_id must be sequential starting at 1, got ${history[i].event_id} at index ${i}`,
      );
    }
  }
}

function replay(workflowType: string, history: Event[]): Command[] {
  if (!KNOWN.has(workflowType)) {
    throw new EngineError("WORKFLOW_TYPE_NOT_FOUND", `unknown workflow type "${workflowType}"`);
  }
  validateHistory(history);

  const last = history[history.length - 1].type;
  if (last === "WORKFLOW_EXECUTION_COMPLETED" || last === "WORKFLOW_EXECUTION_FAILED") {
    return [];
  }

  switch (workflowType) {
    case "greet": {
      if (last !== "WORKFLOW_EXECUTION_STARTED") {
        throw new EngineError("INVALID_HISTORY", `unexpected last event ${last} for greet`);
      }
      const inp = (history[0].attributes.input ?? {}) as Record<string, unknown>;
      const name = String(inp.name ?? "");
      return [{ type: "COMPLETE_WORKFLOW", attributes: { result: { greeting: `hello ${name}` } } }];
    }
    case "fetch": {
      const inp = history[0].attributes.input;
      if (last === "WORKFLOW_EXECUTION_STARTED") {
        return [{ type: "SCHEDULE_ACTIVITY", attributes: { activity_id: "fetch", activity_type: "fetch", input: inp } }];
      }
      if (last === "ACTIVITY_TASK_SCHEDULED") return [];
      if (last === "ACTIVITY_TASK_COMPLETED") {
        return [{ type: "COMPLETE_WORKFLOW", attributes: { result: history[history.length - 1].attributes.result } }];
      }
      throw new EngineError("INVALID_HISTORY", `unexpected last event ${last} for fetch`);
    }
    case "timer_wait": {
      if (last === "WORKFLOW_EXECUTION_STARTED") {
        return [{ type: "START_TIMER", attributes: { timer_id: "t1", duration_ms: 500 } }];
      }
      if (last === "TIMER_STARTED") return [];
      if (last === "TIMER_FIRED") {
        return [{ type: "COMPLETE_WORKFLOW", attributes: { result: "timer fired" } }];
      }
      throw new EngineError("INVALID_HISTORY", `unexpected last event ${last} for timer_wait`);
    }
    case "signal_wait": {
      if (last === "WORKFLOW_EXECUTION_STARTED") return [];
      if (last === "WORKFLOW_EXECUTION_SIGNALED") {
        return [{ type: "COMPLETE_WORKFLOW", attributes: { result: history[history.length - 1].attributes.input } }];
      }
      throw new EngineError("INVALID_HISTORY", `unexpected last event ${last} for signal_wait`);
    }
    case "pipeline": {
      if (last === "WORKFLOW_EXECUTION_STARTED") {
        return [{ type: "SCHEDULE_ACTIVITY", attributes: { activity_id: "step1", activity_type: "work", input: null } }];
      }
      if (last === "ACTIVITY_TASK_SCHEDULED") return [];
      if (last === "ACTIVITY_TASK_COMPLETED") {
        return [{ type: "START_TIMER", attributes: { timer_id: "pause", duration_ms: 100 } }];
      }
      if (last === "TIMER_STARTED") return [];
      if (last === "TIMER_FIRED") {
        return [{ type: "COMPLETE_WORKFLOW", attributes: { result: "done" } }];
      }
      throw new EngineError("INVALID_HISTORY", `unexpected last event ${last} for pipeline`);
    }
  }
  throw new EngineError("WORKFLOW_TYPE_NOT_FOUND", `unknown workflow type "${workflowType}"`);
}

function handleRequest(method: string, params: Record<string, unknown>): unknown {
  if (method === "ping") return { message: "pong" };
  if (method === "replay") {
    const workflowType = params.workflow_type as string;
    const history = (params.history ?? []) as Event[];
    return { commands: replay(workflowType, history) };
  }
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
