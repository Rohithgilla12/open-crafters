// Reference job platform gateway (TypeScript). Passes all 9 stages.
import { createConnection, createServer, type Socket } from "node:net";

const QUEUE_NAME = "jobs";
const LOCK_NAME = "dispatcher";
const HOLDER_ID = "gateway";

class GWError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function rpc(addr: string, method: string, params: Record<string, unknown>): Promise<Record<string, unknown>> {
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
        result?: Record<string, unknown>;
        error?: { code: string; message: string };
      };
      if (resp.error) reject(new GWError(resp.error.code, resp.error.message));
      else resolve(resp.result ?? {});
    });
    conn.on("error", reject);
    conn.write(JSON.stringify({ id: "1", method, params }) + "\n");
  });
}

class Engine {
  scheduler = process.env.SCHEDULER_ADDR ?? "";
  queue = process.env.QUEUE_ADDR ?? "";
  lock = process.env.LOCK_ADDR ?? "";

  async dispatchDue() {
    let token = "";
    try {
      const tr = await rpc(this.lock, "try_acquire", { name: LOCK_NAME, holder_id: HOLDER_ID, lease_ms: 3000 });
      if (!tr.acquired) return;
      token = String(tr.token ?? "");
      for (;;) {
        const pr = await rpc(this.scheduler, "poll", {});
        const job = pr.job as { job_id: string; payload: unknown; lease_token: string } | null | undefined;
        if (!job) break;
        const body = JSON.stringify({
          job_id: job.job_id,
          payload: job.payload,
          lease_token: job.lease_token,
        });
        await rpc(this.queue, "send", { queue: QUEUE_NAME, body });
      }
    } catch {
      return;
    } finally {
      if (token) {
        await rpc(this.lock, "release", { name: LOCK_NAME, token }).catch(() => {});
      }
    }
  }

  async handle(method: string, params: Record<string, unknown>) {
    if (method === "ping") return { message: "pong" };
    if (method === "submit_job") {
      if (params.payload === undefined || params.delay_ms === undefined) {
        throw new GWError("INVALID_PARAMS", "submit_job requires payload and delay_ms");
      }
      const res = await rpc(this.scheduler, "schedule", {
        payload: params.payload,
        delay_ms: params.delay_ms,
      });
      return { job_id: res.job_id };
    }
    if (method === "receive_work") {
      await this.dispatchDue();
      const recv = await rpc(this.queue, "receive", { queue: QUEUE_NAME, visibility_timeout_ms: 30000 });
      const msg = recv.message as { body: string; receipt: string } | null | undefined;
      if (!msg) return { work: null };
      const body = JSON.parse(msg.body) as { job_id: string; payload: unknown; lease_token: string };
      return {
        work: {
          job_id: body.job_id,
          payload: body.payload,
          lease_token: body.lease_token,
          receipt: msg.receipt,
        },
      };
    }
    if (method === "complete_work") {
      const lt = params.lease_token as string;
      const rc = params.receipt as string;
      if (!lt || !rc) throw new GWError("INVALID_PARAMS", "complete_work requires lease_token and receipt");
      await rpc(this.scheduler, "complete", { lease_token: lt });
      await rpc(this.queue, "ack", { receipt: rc });
      return {};
    }
    if (method === "cancel_job") {
      const jid = params.job_id as string;
      if (!jid) throw new GWError("INVALID_PARAMS", "cancel_job requires job_id");
      const res = await rpc(this.scheduler, "cancel", { job_id: jid });
      return { cancelled: res.cancelled };
    }
    if (method === "get_job") {
      const jid = params.job_id as string;
      if (!jid) throw new GWError("INVALID_PARAMS", "get_job requires job_id");
      const res = await rpc(this.scheduler, "get_job", { job_id: jid });
      return { status: res.status };
    }
    throw new GWError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
  }
}

const engine = new Engine();

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
