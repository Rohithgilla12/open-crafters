// Reference notification platform gateway (TypeScript). Passes all 9 stages.
import { createConnection, createServer, type Socket } from "node:net";

const QUEUE_NAME = "notifications";
let seq = 0;

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
  queue = process.env.QUEUE_ADDR ?? "";
  scheduler = process.env.SCHEDULER_ADDR ?? "";
  ratelimiter = process.env.RATE_LIMITER_ADDR ?? "";

  nextId() {
    seq += 1;
    return `n-${seq}`;
  }

  async dispatchDue() {
    for (;;) {
      const pr = await rpc(this.scheduler, "poll", {});
      const job = pr.job as { payload: unknown; lease_token: string } | null | undefined;
      if (!job) break;
      await rpc(this.queue, "send", { queue: QUEUE_NAME, body: JSON.stringify(job.payload) });
      await rpc(this.scheduler, "complete", { lease_token: job.lease_token });
    }
  }

  async handle(method: string, params: Record<string, unknown>) {
    if (method === "ping") return { message: "pong" };
    if (method === "configure_limit") {
      const userId = params.user_id;
      const limit = params.limit;
      const windowMs = params.window_ms;
      if (!userId || !limit || !windowMs) {
        throw new GWError("INVALID_PARAMS", "configure_limit requires user_id, limit, window_ms");
      }
      await rpc(this.ratelimiter, "configure", {
        key: `user:${userId}`,
        algorithm: "fixed_window",
        limit,
        window_ms: windowMs,
      });
      return {};
    }
    if (method === "notify") {
      const userId = String(params.user_id ?? "");
      const channel = String(params.channel ?? "");
      const body = params.body;
      if (!userId || !channel) {
        throw new GWError("INVALID_PARAMS", "notify requires user_id, channel, body");
      }
      let take: Record<string, unknown>;
      try {
        take = await rpc(this.ratelimiter, "take", { key: `user:${userId}`, cost: 1 });
      } catch (e) {
        const err = e as GWError;
        if (err.code === "KEY_NOT_FOUND") {
          throw new GWError("INVALID_PARAMS", "limit not configured for user");
        }
        throw e;
      }
      if (!take.allowed) {
        return { notification_id: null, queued: false, rate_limited: true };
      }
      const nid = this.nextId();
      const env = { notification_id: nid, user_id: userId, channel, body };
      await rpc(this.queue, "send", { queue: QUEUE_NAME, body: JSON.stringify(env) });
      return { notification_id: nid, queued: true };
    }
    if (method === "schedule_notify") {
      const userId = String(params.user_id ?? "");
      const channel = String(params.channel ?? "");
      const body = params.body;
      const delayMs = params.delay_ms;
      if (!userId || !channel || delayMs === undefined) {
        throw new GWError("INVALID_PARAMS", "schedule_notify requires user_id, channel, body, delay_ms");
      }
      const nid = this.nextId();
      const env = { notification_id: nid, user_id: userId, channel, body };
      const res = await rpc(this.scheduler, "schedule", { payload: env, delay_ms: delayMs });
      return { notification_id: nid, job_id: res.job_id };
    }
    if (method === "receive") {
      await this.dispatchDue();
      const recv = await rpc(this.queue, "receive", {
        queue: QUEUE_NAME,
        visibility_timeout_ms: 30000,
      });
      const msg = recv.message as { body: string; receipt: string } | null | undefined;
      if (!msg) return { notification: null };
      const env = JSON.parse(msg.body) as {
        notification_id: string;
        user_id: string;
        channel: string;
        body: string;
      };
      return {
        notification: {
          notification_id: env.notification_id,
          user_id: env.user_id,
          channel: env.channel,
          body: env.body,
          receipt: msg.receipt,
        },
      };
    }
    if (method === "ack") {
      const receipt = String(params.receipt ?? "");
      if (!receipt) throw new GWError("INVALID_PARAMS", "ack requires receipt");
      await rpc(this.queue, "ack", { receipt });
      return {};
    }
    throw new GWError("UNKNOWN_METHOD", `unknown method ${method}`);
  }
}

const engine = new Engine();

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
      const req = JSON.parse(line) as { id?: string; method?: string; params?: Record<string, unknown> };
      engine
        .handle(req.method ?? "", req.params ?? {})
        .then((result) => {
          socket.write(JSON.stringify({ id: req.id, result }) + "\n");
        })
        .catch((e: GWError) => {
          socket.write(
            JSON.stringify({
              id: req.id,
              error: { code: e.code ?? "INTERNAL", message: e.message },
            }) + "\n",
          );
        });
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
