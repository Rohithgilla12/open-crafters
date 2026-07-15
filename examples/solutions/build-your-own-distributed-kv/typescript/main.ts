// Reference distributed KV gateway (TypeScript). Passes all 9 stages.
import { createConnection, createServer, type Socket } from "node:net";

const RING_ID = "kv";
const RAFT_SHARD = "raft-shard";
const LSM_SHARD = "lsm-shard";
const RING_REPLICAS = 64;

class GWError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

class RPCError extends Error {
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
      if (resp.error) reject(new RPCError(resp.error.code, resp.error.message));
      else resolve(resp.result ?? {});
    });
    conn.on("error", reject);
    conn.write(JSON.stringify({ id: "1", method, params }) + "\n");
  });
}

class Engine {
  ready = false;
  ring = process.env.HASHRING_ADDR ?? "";
  lsm = process.env.LSM_ADDR ?? "";
  raft = [process.env.RAFT1_ADDR ?? "", process.env.RAFT2_ADDR ?? "", process.env.RAFT3_ADDR ?? ""];
  private stackReady: Promise<void> | null = null;

  async ensureStack() {
    if (this.ready) return;
    if (!this.stackReady) {
      this.stackReady = this.bootstrapStack();
    }
    await this.stackReady;
  }

  private async bootstrapStack() {
    if (!this.ring || !this.lsm || !this.raft[0]) {
      throw new GWError("INTERNAL", "missing HASHRING_ADDR, LSM_ADDR, or RAFT*_ADDR");
    }
    try {
      await rpc(this.ring, "create_ring", { ring_id: RING_ID, replicas: RING_REPLICAS });
    } catch (e) {
      if (!(e instanceof RPCError) || e.code !== "RING_EXISTS") throw e;
    }
    for (const node of [RAFT_SHARD, LSM_SHARD]) {
      try {
        await rpc(this.ring, "add_node", { ring_id: RING_ID, node_id: node });
      } catch (e) {
        if (!(e instanceof RPCError) || e.code !== "NODE_EXISTS") throw e;
      }
    }
    this.ready = true;
  }

  async lookupNode(key: string) {
    return (await rpc(this.ring, "lookup", { ring_id: RING_ID, key })).node_id as string;
  }

  async raftCall(method: string, params: Record<string, unknown>) {
    // Raft election can take a beat after compose boot; retry NOT_LEADER.
    const deadline = Date.now() + 5000;
    let last: RPCError | null = null;
    while (true) {
      for (const addr of this.raft) {
        if (!addr) continue;
        try {
          return await rpc(addr, method, params);
        } catch (e) {
          if (e instanceof RPCError && e.code === "NOT_LEADER") {
            last = e;
            continue;
          }
          throw e;
        }
      }
      if (Date.now() >= deadline) break;
      await new Promise((r) => setTimeout(r, 50));
    }
    if (last) throw last;
    throw new GWError("NOT_LEADER", "no raft leader available");
  }

  async put(key: string, value: string) {
    const node = await this.lookupNode(key);
    if (node === RAFT_SHARD) await this.raftCall("set", { key, value });
    else if (node === LSM_SHARD) await rpc(this.lsm, "put", { key, value });
    else throw new GWError("INTERNAL", `unknown shard ${node}`);
  }

  async get(key: string) {
    const node = await this.lookupNode(key);
    if (node === RAFT_SHARD) {
      const res = await this.raftCall("get", { key });
      if (!res.found) return { found: false as const, value: null };
      const val = res.value;
      return { found: true as const, value: typeof val === "string" ? val : String(val) };
    }
    if (node === LSM_SHARD) {
      const res = await rpc(this.lsm, "get", { key });
      return { found: Boolean(res.found), value: (res.value as string) ?? "" };
    }
    throw new GWError("INTERNAL", `unknown shard ${node}`);
  }

  async delete(key: string) {
    const node = await this.lookupNode(key);
    if (node !== LSM_SHARD) {
      throw new GWError("UNSUPPORTED", "delete only supported on LSM shard keys");
    }
    return Boolean((await rpc(this.lsm, "del", { key })).deleted);
  }

  async handle(method: string, params: Record<string, unknown>) {
    if (method === "ping") {
      await this.ensureStack();
      return { message: "pong" };
    }
    await this.ensureStack();
    if (method === "put") {
      const key = params.key as string;
      const value = params.value as string;
      if (!key || value === undefined) throw new GWError("INVALID_PARAMS", "put requires key and value");
      try {
        await this.put(key, value);
      } catch (e) {
        if (e instanceof RPCError) throw new GWError(e.code, e.message);
        throw e;
      }
      return {};
    }
    if (method === "get") {
      const key = params.key as string;
      if (!key) throw new GWError("INVALID_PARAMS", "get requires key");
      try {
        const res = await this.get(key);
        if (!res.found) return { found: false, value: null };
        return { found: true, value: res.value };
      } catch (e) {
        if (e instanceof RPCError) throw new GWError(e.code, e.message);
        throw e;
      }
    }
    if (method === "delete") {
      const key = params.key as string;
      if (!key) throw new GWError("INVALID_PARAMS", "delete requires key");
      try {
        return { deleted: await this.delete(key) };
      } catch (e) {
        if (e instanceof RPCError) throw new GWError(e.code, e.message);
        throw e;
      }
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
