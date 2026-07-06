// Reference URL shortener gateway (TypeScript). Passes all 9 stages.
import { createConnection } from "node:net";
import { createServer, type Socket } from "node:net";

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
      const resp = JSON.parse(buf.slice(0, idx)) as { result?: Record<string, unknown>; error?: { code: string; message: string } };
      if (resp.error) reject(new GWError(resp.error.code, resp.error.message));
      else resolve(resp.result ?? {});
    });
    conn.on("error", reject);
    conn.write(JSON.stringify({ id: "1", method, params }) + "\n");
  });
}

class Engine {
  ready = false;
  idgen = process.env.IDGEN_ADDR ?? "";
  bloom = process.env.BLOOM_ADDR ?? "";
  store = process.env.STORE_ADDR ?? "";

  async ensureBloom() {
    if (this.ready) return;
    try {
      await rpc(this.bloom, "create", { filter_id: "codes", m: 8192, k: 4 });
    } catch (e) {
      if (!(e instanceof GWError) || e.code !== "FILTER_EXISTS") throw e;
    }
    this.ready = true;
  }

  async handle(method: string, params: Record<string, unknown>) {
    if (method === "ping") return { message: "pong" };
    if (method === "shorten") {
      await this.ensureBloom();
      const url = params.url as string;
      if (!url) throw new GWError("INVALID_PARAMS", "url required");
      const code = (await rpc(this.idgen, "next_id", {})).id as string;
      await rpc(this.bloom, "add", { filter_id: "codes", item: code });
      await rpc(this.store, "put", { key: `links/${code}`, body: url });
      return { code };
    }
    if (method === "resolve") {
      await this.ensureBloom();
      const code = params.code as string;
      if (!code) throw new GWError("INVALID_PARAMS", "code required");
      const bp = await rpc(this.bloom, "contains", { filter_id: "codes", item: code });
      if (!bp.maybe_present) return { found: false };
      try {
        const got = await rpc(this.store, "get", { key: `links/${code}` });
        if (!got.found) return { found: false };
        return { found: true, url: got.body };
      } catch (e) {
        if (e instanceof GWError && e.code === "NOT_FOUND") return { found: false };
        throw e;
      }
    }
    if (method === "record_click") {
      await this.ensureBloom();
      const code = params.code as string;
      if (!code) throw new GWError("INVALID_PARAMS", "code required");
      await rpc(this.store, "put", { key: `clicks/${code}`, body: String(Date.now()) });
      return {};
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
