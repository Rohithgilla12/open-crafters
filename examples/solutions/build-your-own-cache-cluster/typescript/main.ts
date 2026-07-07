// Reference cache cluster gateway (TypeScript). Passes all 9 stages.
import { createConnection, createServer, type Socket } from "node:net";

const RING_ID = "cache";
const NODE1 = "node1";
const NODE2 = "node2";
const FILTER_ID = "keys";

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
  ready = false;
  ring = process.env.HASHRING_ADDR ?? "";
  bloom = process.env.BLOOM_ADDR ?? "";
  limiter = process.env.LIMITER_ADDR ?? "";
  cache1 = process.env.CACHE_NODE1_ADDR ?? "";
  cache2 = process.env.CACHE_NODE2_ADDR ?? "";

  async ensureStack() {
    if (this.ready) return;
    try {
      await rpc(this.ring, "create_ring", { ring_id: RING_ID, replicas: 64 });
    } catch (e) {
      if (!(e instanceof GWError) || e.code !== "RING_EXISTS") throw e;
    }
    for (const n of [NODE1, NODE2]) {
      try {
        await rpc(this.ring, "add_node", { ring_id: RING_ID, node_id: n });
      } catch (e) {
        if (!(e instanceof GWError) || e.code !== "NODE_EXISTS") throw e;
      }
    }
    try {
      await rpc(this.bloom, "create", { filter_id: FILTER_ID, m: 8192, k: 4 });
    } catch (e) {
      if (!(e instanceof GWError) || e.code !== "FILTER_EXISTS") throw e;
    }
    await rpc(this.cache1, "configure", { max_keys: 4096 });
    await rpc(this.cache2, "configure", { max_keys: 4096 });
    this.ready = true;
  }

  cacheAddr(nodeId: string) {
    if (nodeId === NODE1) return this.cache1;
    if (nodeId === NODE2) return this.cache2;
    throw new GWError("INTERNAL", `unknown node ${nodeId}`);
  }

  async lookup(key: string) {
    return (await rpc(this.ring, "lookup", { ring_id: RING_ID, key })).node_id as string;
  }

  async admit(key: string) {
    const rlKey = `rl:${key}`;
    let res: Record<string, unknown>;
    try {
      res = await rpc(this.limiter, "take", { key: rlKey, cost: 1 });
    } catch (e) {
      if (!(e instanceof GWError) || e.code !== "KEY_NOT_FOUND") throw e;
      await rpc(this.limiter, "configure", {
        key: rlKey,
        algorithm: "token_bucket",
        capacity: 100,
        refill_tokens: 100,
        refill_interval_ms: 1000,
      });
      res = await rpc(this.limiter, "take", { key: rlKey, cost: 1 });
    }
    if (!res.allowed) throw new GWError("RATE_LIMITED", "rate limit exceeded");
  }

  async bloomMaybe(key: string) {
    return Boolean((await rpc(this.bloom, "contains", { filter_id: FILTER_ID, item: key })).maybe_present);
  }

  async handle(method: string, params: Record<string, unknown>) {
    if (method === "ping") return { message: "pong" };
    await this.ensureStack();
    if (method === "set") {
      const key = params.key as string;
      const value = params.value as string;
      if (!key || value === undefined) throw new GWError("INVALID_PARAMS", "set requires key and value");
      await this.admit(key);
      const node = await this.lookup(key);
      const p: Record<string, unknown> = { key, value };
      if (Number(params.ttl_ms) > 0) p.ttl_ms = params.ttl_ms;
      const ver = (await rpc(this.cacheAddr(node), "set", p)).version;
      await rpc(this.bloom, "add", { filter_id: FILTER_ID, item: key });
      return { version: ver };
    }
    if (method === "get") {
      const key = params.key as string;
      if (!key) throw new GWError("INVALID_PARAMS", "get requires key");
      if (!(await this.bloomMaybe(key))) return { hit: false };
      await this.admit(key);
      const node = await this.lookup(key);
      const res = await rpc(this.cacheAddr(node), "get", { key });
      if (!res.hit) return { hit: false };
      return { hit: true, value: res.value, version: res.version };
    }
    if (method === "delete") {
      const key = params.key as string;
      if (!key) throw new GWError("INVALID_PARAMS", "delete requires key");
      await this.admit(key);
      const node = await this.lookup(key);
      return { deleted: (await rpc(this.cacheAddr(node), "delete", { key })).deleted };
    }
    if (method === "mget") {
      const keys = params.keys as string[];
      if (!keys?.length) throw new GWError("INVALID_PARAMS", "mget requires keys");
      const entries = [];
      for (const key of keys) {
        const r = (await this.handle("get", { key })) as Record<string, unknown>;
        entries.push({ key, ...r });
      }
      return { entries };
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
