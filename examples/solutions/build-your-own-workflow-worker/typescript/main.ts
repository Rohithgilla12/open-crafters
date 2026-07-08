// Reference gateway for build-your-own-workflow-worker (TypeScript). Passes all 9 stages.
import { createConnection, createServer, type Socket } from "node:net";

const DEFAULT_QUEUE = "default";
const POLL_INTERVAL_MS = 50;
const DRIVE_TIMEOUT_MS = 30_000;

class GWError extends Error {
  constructor(
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

class RPCError extends Error {
  constructor(
    public code: string,
    message: string,
  ) {
    super(message);
  }
}

function rpc(addr: string, method: string, params: Record<string, unknown>): Promise<unknown> {
  return new Promise((resolve, reject) => {
    const [host, portStr] = addr.split(":");
    const conn = createConnection({ host, port: Number(portStr) });
    let buf = "";
    conn.setEncoding("utf8");
    conn.on("data", (chunk) => {
      buf += chunk;
      const idx = buf.indexOf("\n");
      if (idx === -1) return;
      conn.end();
      const resp = JSON.parse(buf.slice(0, idx)) as {
        result?: unknown;
        error?: { code: string; message: string };
      };
      if (resp.error) reject(new RPCError(resp.error.code, resp.error.message));
      else resolve(resp.result);
    });
    conn.on("error", reject);
    conn.write(JSON.stringify({ id: "1", method, params }) + "\n");
  });
}

function activityResult(activityType: string, _input: unknown): Record<string, unknown> {
  if (activityType === "fetch") return { status: 200, body: "ok" };
  if (activityType === "work") return { done: true };
  return { ok: true };
}

type WorkflowTask = {
  task_token: string;
  workflow_id: string;
  workflow_type: string;
  history: Record<string, unknown>[];
};

type ActivityTask = {
  task_token: string;
  workflow_id: string;
  activity_type: string;
  input: unknown;
};

class Engine {
  constructor(
    private temporal: string,
    private sdk: string,
  ) {}

  async describe(workflowId: string) {
    const res = (await rpc(this.temporal, "describe_workflow", { workflow_id: workflowId })) as {
      status: string;
      result?: unknown;
      error?: unknown;
    };
    return { status: res.status, result: res.result, error: res.error };
  }

  async replay(workflowType: string, history: Record<string, unknown>[]) {
    const res = (await rpc(this.sdk, "replay", { workflow_type: workflowType, history })) as {
      commands?: Record<string, unknown>[];
    };
    return res.commands ?? [];
  }

  async driveRound() {
    for (let i = 0; i < 8; i++) {
      const wtRes = (await rpc(this.temporal, "poll_workflow_task", { task_queue: DEFAULT_QUEUE })) as {
        task?: WorkflowTask | null;
      };
      if (!wtRes.task) break;
      const cmds = await this.replay(wtRes.task.workflow_type, wtRes.task.history);
      await rpc(this.temporal, "complete_workflow_task", {
        task_token: wtRes.task.task_token,
        commands: cmds,
      });
    }
    for (let i = 0; i < 8; i++) {
      const atRes = (await rpc(this.temporal, "poll_activity_task", { task_queue: DEFAULT_QUEUE })) as {
        task?: ActivityTask | null;
      };
      if (!atRes.task) break;
      const result = activityResult(atRes.task.activity_type, atRes.task.input);
      await rpc(this.temporal, "complete_activity_task", {
        task_token: atRes.task.task_token,
        result,
      });
    }
  }

  async driveUntilDone(workflowId: string) {
    const deadline = Date.now() + DRIVE_TIMEOUT_MS;
    while (Date.now() < deadline) {
      const { status, result, error } = await this.describe(workflowId);
      if (status === "COMPLETED") return { status, result, error: null };
      if (status === "FAILED") return { status, result: null, error };
      await this.driveRound();
      await new Promise((r) => setTimeout(r, POLL_INTERVAL_MS));
    }
    throw new Error(`workflow "${workflowId}" did not finish within timeout`);
  }

  async handle(method: string, params: Record<string, unknown>) {
    if (method === "ping") return { message: "pong" };
    if (method === "start_workflow") {
      const workflowId = params.workflow_id as string;
      const workflowType = params.workflow_type as string;
      if (!workflowId || !workflowType) {
        throw new GWError("INVALID_PARAMS", "start_workflow requires workflow_id and workflow_type");
      }
      const taskQueue = (params.task_queue as string) || DEFAULT_QUEUE;
      try {
        const res = (await rpc(this.temporal, "start_workflow", {
          workflow_id: workflowId,
          workflow_type: workflowType,
          input: params.input,
          task_queue: taskQueue,
        })) as { run_id: string };
        return { run_id: res.run_id };
      } catch (e) {
        if (e instanceof RPCError) throw new GWError(e.code, e.message);
        throw e;
      }
    }
    if (method === "signal_workflow") {
      const workflowId = params.workflow_id as string;
      const signalName = params.signal_name as string;
      if (!workflowId || !signalName) {
        throw new GWError("INVALID_PARAMS", "signal_workflow requires workflow_id and signal_name");
      }
      try {
        await rpc(this.temporal, "signal_workflow", {
          workflow_id: workflowId,
          signal_name: signalName,
          input: params.input,
        });
        return {};
      } catch (e) {
        if (e instanceof RPCError) throw new GWError(e.code, e.message);
        throw e;
      }
    }
    if (method === "await_workflow") {
      const workflowId = params.workflow_id as string;
      if (!workflowId) throw new GWError("INVALID_PARAMS", "await_workflow requires workflow_id");
      return this.driveUntilDone(workflowId);
    }
    if (method === "run_workflow") {
      const workflowId = params.workflow_id as string;
      const workflowType = params.workflow_type as string;
      if (!workflowId || !workflowType) {
        throw new GWError("INVALID_PARAMS", "run_workflow requires workflow_id and workflow_type");
      }
      const taskQueue = (params.task_queue as string) || DEFAULT_QUEUE;
      try {
        await rpc(this.temporal, "start_workflow", {
          workflow_id: workflowId,
          workflow_type: workflowType,
          input: params.input,
          task_queue: taskQueue,
        });
      } catch (e) {
        if (e instanceof RPCError) throw new GWError(e.code, e.message);
        throw e;
      }
      return this.driveUntilDone(workflowId);
    }
    throw new GWError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
  }
}

const temporal = process.env.TEMPORAL_ADDR ?? "";
const sdk = process.env.SDK_ADDR ?? "";
if (!temporal || !sdk) {
  console.error("TEMPORAL_ADDR and SDK_ADDR must be set by the harness");
  process.exit(1);
}
const engine = new Engine(temporal, sdk);

function handleConn(socket: Socket) {
  let buf = "";
  socket.setEncoding("utf8");
  socket.on("data", async (chunk) => {
    buf += chunk;
    let idx: number;
    while ((idx = buf.indexOf("\n")) !== -1) {
      const line = buf.slice(0, idx);
      buf = buf.slice(idx + 1);
      if (!line.trim()) continue;
      const req = JSON.parse(line) as { id?: string; method?: string; params?: Record<string, unknown> };
      try {
        const result = await engine.handle(req.method ?? "", req.params ?? {});
        socket.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as GWError | RPCError;
        socket.write(
          JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n",
        );
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`workflow worker gateway listening on 127.0.0.1:${port}`);
});
