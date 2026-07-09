// Reference chat service gateway (TypeScript). Passes all 9 stages.
import { createConnection, createServer, type Socket } from "node:net";

const DELIVERY_QUEUE = "delivery";

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
  private ready = false;
  idgen = process.env.IDGEN_ADDR ?? "";
  log = process.env.LOG_ADDR ?? "";
  queue = process.env.QUEUE_ADDR ?? "";

  async ensureStack() {
    if (this.ready) return;
    if (!this.idgen || !this.log || !this.queue) {
      throw new GWError("INTERNAL", "missing IDGEN_ADDR, LOG_ADDR, or QUEUE_ADDR");
    }
    await rpc(this.idgen, "configure", { worker_id: 1 });
    this.ready = true;
  }

  async handle(method: string, params: Record<string, unknown>) {
    if (method === "ping") {
      await this.ensureStack();
      return { message: "pong" };
    }
    await this.ensureStack();
    if (method === "send_message") {
      const conv = params.conversation_id;
      const sender = params.sender;
      const body = params.body;
      if (typeof conv !== "string" || typeof sender !== "string" || typeof body !== "string") {
        throw new GWError("INVALID_PARAMS", "send_message requires conversation_id, sender, body");
      }
      const idRes = await rpc(this.idgen, "next_id", {});
      const env = JSON.stringify({
        message_id: idRes.id,
        conversation_id: conv,
        sender,
        body,
      });
      const logRes = await rpc(this.log, "append", { topic: conv, value: env });
      await rpc(this.queue, "send", { queue: DELIVERY_QUEUE, body: env });
      return { message_id: idRes.id, offset: logRes.offset };
    }
    if (method === "read_messages") {
      const conv = params.conversation_id;
      if (typeof conv !== "string") {
        throw new GWError("INVALID_PARAMS", "read_messages requires conversation_id");
      }
      const offset = (params.offset as number) ?? 0;
      const max = (params.max as number) ?? 100;
      const logRes = await rpc(this.log, "read", { topic: conv, offset, max });
      return { records: logRes.records ?? [] };
    }
    if (method === "poll_delivery") {
      const qRes = await rpc(this.queue, "receive", { queue: DELIVERY_QUEUE, visibility_timeout_ms: 30000 });
      const msg = qRes.message as { body: string; receipt: string } | null | undefined;
      if (!msg) return { message: null };
      const env = JSON.parse(msg.body) as {
        message_id: string;
        conversation_id: string;
        sender: string;
        body: string;
      };
      return {
        message: {
          message_id: env.message_id,
          conversation_id: env.conversation_id,
          sender: env.sender,
          body: env.body,
          receipt: msg.receipt,
        },
      };
    }
    if (method === "ack_delivery") {
      const receipt = params.receipt;
      if (typeof receipt !== "string" || !receipt) {
        throw new GWError("INVALID_PARAMS", "ack_delivery requires receipt");
      }
      await rpc(this.queue, "ack", { receipt });
      return {};
    }
    throw new GWError("UNKNOWN_METHOD", `unknown method ${method}`);
  }
}

const engine = new Engine();

function handleConn(sock: Socket) {
  let buf = "";
  sock.setEncoding("utf8");
  sock.on("data", async (chunk) => {
    buf += chunk;
    let idx: number;
    while ((idx = buf.indexOf("\n")) !== -1) {
      const line = buf.slice(0, idx);
      buf = buf.slice(idx + 1);
      const req = JSON.parse(line) as { id?: string; method: string; params?: Record<string, unknown> };
      try {
        const result = await engine.handle(req.method, req.params ?? {});
        sock.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const ge = e as GWError;
        const code = ge.code ?? "INTERNAL";
        const message = ge.message ?? String(e);
        sock.write(JSON.stringify({ id: req.id, error: { code, message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1] ?? 0);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
