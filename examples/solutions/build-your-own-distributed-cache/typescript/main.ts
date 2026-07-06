// Reference solution for "Build your own distributed cache" (TypeScript, Bun). Passes all 9 stages.

import { createServer, type Socket } from "node:net";

class CacheError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

type Entry = { value: string; version: number; expiresAt: number };

class Cache {
  maxKeys = 0;
  data = new Map<string, Entry>();
  order: string[] = [];

  now(): number {
    return Date.now();
  }

  expired(e: Entry, now: number): boolean {
    return e.expiresAt > 0 && now >= e.expiresAt;
  }

  remove(key: string): void {
    if (!this.data.has(key)) return;
    this.data.delete(key);
    this.order = this.order.filter((k) => k !== key);
  }

  touch(key: string): void {
    this.order = this.order.filter((k) => k !== key);
    this.order.push(key);
  }

  getEntry(key: string): Entry | null {
    const now = this.now();
    const e = this.data.get(key);
    if (!e || this.expired(e, now)) {
      if (e) this.remove(key);
      return null;
    }
    this.touch(key);
    return e;
  }

  evict(): void {
    while (this.maxKeys > 0 && this.data.size >= this.maxKeys) {
      const victim = this.order.shift();
      if (!victim) break;
      this.data.delete(victim);
    }
  }

  set(key: string, value: string, ttlMs = 0): number {
    if (!key || !value) throw new CacheError("INVALID_PARAMS", "key and value required");
    const now = this.now();
    const expiresAt = ttlMs > 0 ? now + ttlMs : 0;
    const existing = this.data.get(key);
    if (existing && !this.expired(existing, now)) {
      existing.value = value;
      existing.version += 1;
      existing.expiresAt = expiresAt;
      this.touch(key);
      return existing.version;
    }
    if (existing) this.remove(key);
    this.evict();
    this.data.set(key, { value, version: 1, expiresAt });
    this.touch(key);
    return 1;
  }
}

class Engine {
  cache = new Cache();

  handle(method: string, params: Record<string, unknown>): unknown {
    if (method === "ping") return { message: "pong" };
    if (method === "configure") {
      const maxKeys = params.max_keys as number;
      if (!maxKeys || maxKeys < 1) throw new CacheError("INVALID_PARAMS", "max_keys >= 1");
      this.cache.maxKeys = maxKeys;
      return {};
    }
    if (method === "set") {
      const ver = this.cache.set(params.key as string, params.value as string, (params.ttl_ms as number) ?? 0);
      return { version: ver };
    }
    if (method === "get") {
      const e = this.cache.getEntry(params.key as string);
      if (!e) return { hit: false };
      return { hit: true, value: e.value, version: e.version };
    }
    if (method === "delete") {
      const e = this.cache.getEntry(params.key as string);
      if (!e) return { deleted: false };
      this.cache.remove(params.key as string);
      return { deleted: true };
    }
    if (method === "setnx") {
      const key = params.key as string;
      const value = params.value as string;
      if (!key || !value) throw new CacheError("INVALID_PARAMS", "key and value required");
      if (this.cache.getEntry(key)) return { stored: false };
      const ver = this.cache.set(key, value, (params.ttl_ms as number) ?? 0);
      return { stored: true, version: ver };
    }
    if (method === "cas") {
      const key = params.key as string;
      const value = params.value as string;
      const expected = params.expected_version as number;
      if (!key || !value) throw new CacheError("INVALID_PARAMS", "cas fields required");
      const e = this.cache.getEntry(key);
      if (!e || e.version !== expected) return { swapped: false };
      e.value = value;
      e.version += 1;
      const ttl = (params.ttl_ms as number) ?? 0;
      e.expiresAt = ttl > 0 ? this.cache.now() + ttl : 0;
      return { swapped: true, version: e.version };
    }
    if (method === "mget") {
      const keys = params.keys as string[];
      if (!keys?.length || keys.length > 50) throw new CacheError("INVALID_PARAMS", "1..50 keys");
      return {
        entries: keys.map((key) => {
          const e = this.cache.getEntry(key);
          if (!e) return { key, hit: false };
          return { key, hit: true, value: e.value, version: e.version };
        }),
      };
    }
    throw new CacheError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
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
        const err = e as CacheError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
