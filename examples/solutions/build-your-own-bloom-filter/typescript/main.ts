// Reference solution for "Build your own bloom filter" (TypeScript, Bun). Passes all 9 stages.

import { createServer, type Socket } from "node:net";

const FNV_OFFSET64 = 14695981039346656037n;
const FNV_PRIME64 = 1099511628211n;
const MASK64 = (1n << 64n) - 1n;

class BloomFilterError extends Error {
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

function hashPositions(item: string, m: number, k: number): number[] {
  const itemBytes = Buffer.from(item, "utf8");
  const h1 = fnv1a64(itemBytes);
  const h2 = fnv1a64(Buffer.concat([itemBytes, Buffer.from([0x01])]));
  const positions: number[] = [];
  for (let i = 0; i < k; i++) {
    positions.push(Number((h1 + BigInt(i) * h2) % BigInt(m)));
  }
  return positions;
}

class BloomFilter {
  bits: Buffer;
  constructor(public m: number, public k: number) {
    this.bits = Buffer.alloc(Math.ceil(m / 8));
  }

  setBit(i: number): void {
    this.bits[i >> 3] |= 1 << (i & 7);
  }

  getBit(i: number): boolean {
    return (this.bits[i >> 3] & (1 << (i & 7))) !== 0;
  }

  add(item: string): void {
    for (const pos of hashPositions(item, this.m, this.k)) this.setBit(pos);
  }

  contains(item: string): boolean {
    return hashPositions(item, this.m, this.k).every((pos) => this.getBit(pos));
  }
}

class Engine {
  filters = new Map<string, BloomFilter>();

  handle(method: string, params: Record<string, unknown>): unknown {
    if (method === "ping") return { message: "pong" };
    if (method === "create") {
      const filterId = params.filter_id as string;
      const m = params.m as number;
      const k = params.k as number;
      if (!filterId || m == null || k == null || m < 8 || k < 1) {
        throw new BloomFilterError("INVALID_PARAMS", "create requires filter_id, m>=8, k>=1");
      }
      if (this.filters.has(filterId)) {
        throw new BloomFilterError("FILTER_EXISTS", `filter ${JSON.stringify(filterId)} already exists`);
      }
      this.filters.set(filterId, new BloomFilter(m, k));
      return {};
    }
    if (method === "add") {
      const filterId = params.filter_id as string;
      const item = params.item as string;
      const bf = this.filters.get(filterId);
      if (!bf) throw new BloomFilterError("FILTER_NOT_FOUND", `no filter ${JSON.stringify(filterId)}`);
      bf.add(item);
      return {};
    }
    if (method === "contains") {
      const filterId = params.filter_id as string;
      const item = params.item as string;
      const bf = this.filters.get(filterId);
      if (!bf) throw new BloomFilterError("FILTER_NOT_FOUND", `no filter ${JSON.stringify(filterId)}`);
      return { maybe_present: bf.contains(item) };
    }
    if (method === "delete_filter") {
      const filterId = params.filter_id as string;
      if (!this.filters.has(filterId)) return { deleted: false };
      this.filters.delete(filterId);
      return { deleted: true };
    }
    throw new BloomFilterError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
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
      try {
        const result = engine.handle(req.method ?? "", req.params ?? {});
        socket.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as BloomFilterError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
