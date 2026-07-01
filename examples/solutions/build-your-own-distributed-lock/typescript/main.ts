// Reference solution for "Build your own distributed lock" (TypeScript, Bun). Passes all 9 stages.

import { createServer, type Socket } from "node:net";
import { mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { randomUUID } from "node:crypto";

class DistLockError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

type LockState = { holder_id: string; token: string; expires_at_ms: number };

const nowMS = () => Date.now();

class Engine {
  private locks = new Map<string, LockState>();
  private mutex = Promise.resolve();

  constructor(private statePath: string) {
    this.load();
  }

  private async withLock<T>(fn: () => T | Promise<T>): Promise<T> {
    const prev = this.mutex;
    let release!: () => void;
    this.mutex = new Promise<void>((r) => {
      release = r;
    });
    await prev;
    try {
      return await fn();
    } finally {
      release();
    }
  }

  private load(): void {
    try {
      const data = JSON.parse(readFileSync(this.statePath, "utf8")) as {
        locks?: Record<string, LockState>;
      };
      if (data.locks) {
        for (const [k, v] of Object.entries(data.locks)) {
          this.locks.set(k, v);
        }
      }
    } catch {
      /* fresh start */
    }
  }

  private persist(): void {
    const obj: Record<string, LockState> = {};
    for (const [k, v] of this.locks) obj[k] = v;
    const tmp = this.statePath + ".tmp";
    writeFileSync(tmp, JSON.stringify({ locks: obj }));
    renameSync(tmp, this.statePath);
  }

  private held(st: LockState | undefined, now: number): st is LockState {
    return st !== undefined && st.expires_at_ms > now;
  }

  private validateAcquire(p: Record<string, unknown>): { name: string; holder_id: string; lease_ms: number } {
    const name = p.name as string;
    const holder_id = p.holder_id as string;
    const lease_ms = p.lease_ms as number;
    if (!name || !holder_id || lease_ms === undefined) {
      throw new DistLockError("INVALID_PARAMS", "acquire requires name, holder_id, lease_ms");
    }
    if (lease_ms < 1) throw new DistLockError("INVALID_PARAMS", "lease_ms must be >= 1");
    return { name, holder_id, lease_ms };
  }

  private grant(name: string, holder_id: string, lease_ms: number, now: number): LockState {
    const st: LockState = { holder_id, token: randomUUID().replace(/-/g, ""), expires_at_ms: now + lease_ms };
    this.locks.set(name, st);
    this.persist();
    return st;
  }

  async acquire(p: Record<string, unknown>, tryMode = false): Promise<unknown> {
    const { name, holder_id, lease_ms } = this.validateAcquire(p);
    return this.withLock(() => {
      const now = nowMS();
      const cur = this.locks.get(name);
      if (this.held(cur, now)) {
        if (tryMode) return { acquired: false };
        throw new DistLockError("LOCK_HELD", `lock ${JSON.stringify(name)} is held`);
      }
      const st = this.grant(name, holder_id, lease_ms, now);
      if (tryMode) return { acquired: true, token: st.token, expires_at_ms: st.expires_at_ms };
      return { token: st.token, expires_at_ms: st.expires_at_ms };
    });
  }

  async release(p: Record<string, unknown>): Promise<unknown> {
    const name = p.name as string;
    const token = p.token as string;
    if (!name || !token) throw new DistLockError("INVALID_PARAMS", "release requires name and token");
    return this.withLock(() => {
      const now = nowMS();
      const cur = this.locks.get(name);
      if (!this.held(cur, now) || cur.token !== token) return { released: false };
      this.locks.delete(name);
      this.persist();
      return { released: true };
    });
  }

  async renew(p: Record<string, unknown>): Promise<unknown> {
    const name = p.name as string;
    const token = p.token as string;
    const lease_ms = p.lease_ms as number;
    if (!name || !token || lease_ms === undefined) {
      throw new DistLockError("INVALID_PARAMS", "renew requires name, token, lease_ms");
    }
    if (lease_ms < 1) throw new DistLockError("INVALID_PARAMS", "lease_ms must be >= 1");
    return this.withLock(() => {
      const now = nowMS();
      const cur = this.locks.get(name);
      if (!this.held(cur, now) || cur.token !== token) {
        throw new DistLockError("NOT_HOLDER", "token does not match current holder");
      }
      cur.expires_at_ms = Math.max(now, cur.expires_at_ms) + lease_ms;
      this.persist();
      return { expires_at_ms: cur.expires_at_ms };
    });
  }

  async status(p: Record<string, unknown>): Promise<unknown> {
    const name = p.name as string;
    if (!name) throw new DistLockError("INVALID_PARAMS", "status requires name");
    return this.withLock(() => {
      const now = nowMS();
      const cur = this.locks.get(name);
      if (!this.held(cur, now)) return { held: false };
      return { held: true, holder_id: cur.holder_id, expires_at_ms: cur.expires_at_ms, token: cur.token };
    });
  }

  async handle(method: string, params: Record<string, unknown>): Promise<unknown> {
    if (method === "ping") return { message: "pong" };
    switch (method) {
      case "acquire":
        return this.acquire(params);
      case "try_acquire":
        return this.acquire(params, true);
      case "release":
        return this.release(params);
      case "renew":
        return this.renew(params);
      case "status":
        return this.status(params);
      default:
        throw new DistLockError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
    }
  }
}

function handleConn(socket: Socket, engine: Engine): void {
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
      void (async () => {
        try {
          const result = await engine.handle(req.method ?? "", req.params ?? {});
          socket.write(JSON.stringify({ id: req.id, result }) + "\n");
        } catch (e) {
          const err = e as DistLockError;
          socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
        }
      })();
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
const dataDir = process.argv[process.argv.indexOf("--data-dir") + 1];
mkdirSync(dataDir, { recursive: true });
const engine = new Engine(join(dataDir, "state.json"));
createServer((s) => handleConn(s, engine)).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
