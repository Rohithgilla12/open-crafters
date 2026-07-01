// Reference solution for "Build your own hash ring" (TypeScript, Bun). Passes all 9 stages.

import { createServer, type Socket } from "node:net";

const FNV_OFFSET64 = 14695981039346656037n;
const FNV_PRIME64 = 1099511628211n;
const MASK64 = (1n << 64n) - 1n;

class HashRingError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function fnv1a64(data: Buffer): bigint {
  let h = FNV_OFFSET64;
  for (const b of data) {
    h ^= BigInt(b);
    h = (h * FNV_PRIME64) & MASK64;
  }
  return h;
}

function hashKey(key: string): bigint {
  return fnv1a64(Buffer.from(key, "utf8"));
}

function vnodePosition(nodeId: string, replica: number): bigint {
  return fnv1a64(Buffer.from(`${nodeId}#${replica}`, "utf8"));
}

class Ring {
  nodes = new Set<string>();
  constructor(public replicas: number) {}

  sortedNodes(): string[] {
    return [...this.nodes].sort();
  }

  lookup(key: string): string {
    if (this.nodes.size === 0) {
      throw new HashRingError("NO_NODES", "ring has no nodes");
    }
    const vnodes: { position: bigint; nodeId: string }[] = [];
    for (const nodeId of this.sortedNodes()) {
      for (let i = 0; i < this.replicas; i++) {
        vnodes.push({ position: vnodePosition(nodeId, i), nodeId });
      }
    }
    vnodes.sort((a, b) => (a.position === b.position ? a.nodeId.localeCompare(b.nodeId) : a.position < b.position ? -1 : 1));
    const h = hashKey(key);
    for (const v of vnodes) {
      if (v.position >= h) return v.nodeId;
    }
    return vnodes[0]!.nodeId;
  }
}

class Engine {
  rings = new Map<string, Ring>();
  private lock = Promise.resolve();

  private withLock<T>(fn: () => T | Promise<T>): Promise<T> {
    const run = this.lock.then(fn);
    this.lock = run.then(
      () => undefined,
      () => undefined,
    );
    return run;
  }

  async handle(method: string, params: Record<string, unknown>): Promise<unknown> {
    if (method === "ping") return { message: "pong" };
    if (method === "create_ring") {
      const ringId = params.ring_id as string;
      const replicas = params.replicas as number;
      if (!ringId || replicas == null || replicas < 1) {
        throw new HashRingError("INVALID_PARAMS", "create_ring requires ring_id and replicas>=1");
      }
      return this.withLock(() => {
        if (this.rings.has(ringId)) {
          throw new HashRingError("RING_EXISTS", `ring ${JSON.stringify(ringId)} already exists`);
        }
        this.rings.set(ringId, new Ring(replicas));
        return {};
      });
    }
    if (method === "add_node") {
      const ringId = params.ring_id as string;
      const nodeId = params.node_id as string;
      if (!ringId || !nodeId) {
        throw new HashRingError("INVALID_PARAMS", "add_node requires ring_id and node_id");
      }
      return this.withLock(() => {
        const ring = this.rings.get(ringId);
        if (!ring) throw new HashRingError("RING_NOT_FOUND", `no ring ${JSON.stringify(ringId)}`);
        if (ring.nodes.has(nodeId)) {
          throw new HashRingError("NODE_EXISTS", `node ${JSON.stringify(nodeId)} already on ring`);
        }
        ring.nodes.add(nodeId);
        return {};
      });
    }
    if (method === "remove_node") {
      const ringId = params.ring_id as string;
      const nodeId = params.node_id as string;
      if (!ringId || !nodeId) {
        throw new HashRingError("INVALID_PARAMS", "remove_node requires ring_id and node_id");
      }
      return this.withLock(() => {
        const ring = this.rings.get(ringId);
        if (!ring) throw new HashRingError("RING_NOT_FOUND", `no ring ${JSON.stringify(ringId)}`);
        if (!ring.nodes.has(nodeId)) return { removed: false };
        ring.nodes.delete(nodeId);
        return { removed: true };
      });
    }
    if (method === "lookup") {
      const ringId = params.ring_id as string;
      const key = params.key as string;
      if (!ringId || !key) {
        throw new HashRingError("INVALID_PARAMS", "lookup requires ring_id and key");
      }
      return this.withLock(() => {
        const ring = this.rings.get(ringId);
        if (!ring) throw new HashRingError("RING_NOT_FOUND", `no ring ${JSON.stringify(ringId)}`);
        return { node_id: ring.lookup(key) };
      });
    }
    if (method === "list_nodes") {
      const ringId = params.ring_id as string;
      if (!ringId) throw new HashRingError("INVALID_PARAMS", "list_nodes requires ring_id");
      return this.withLock(() => {
        const ring = this.rings.get(ringId);
        if (!ring) throw new HashRingError("RING_NOT_FOUND", `no ring ${JSON.stringify(ringId)}`);
        return { nodes: ring.sortedNodes() };
      });
    }
    throw new HashRingError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
  }
}

const engine = new Engine();

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
      void engine
        .handle(req.method ?? "", req.params ?? {})
        .then((result) => {
          socket.write(JSON.stringify({ id: req.id, result }) + "\n");
        })
        .catch((e: HashRingError) => {
          socket.write(JSON.stringify({ id: req.id, error: { code: e.code, message: e.message } }) + "\n");
        });
    }
  });
}

const portIdx = process.argv.indexOf("--port");
const port = Number(process.argv[portIdx + 1]);
void process.argv[process.argv.indexOf("--data-dir") + 1];
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
