// Reference solution for "Build your own rate limiter" (TypeScript, Bun). Passes all 9 stages.
//
// Keyed limiters with three algorithms (fixed window, token bucket, sliding
// window), atomic admission under concurrency, and crash-durable state.

import { createServer, type Socket } from "node:net";
import { mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { join } from "node:path";

class RateLimiterError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

const nowMs = (): number => Date.now();

interface Limiter {
  algorithm: "token_bucket" | "fixed_window" | "sliding_window";
  capacity?: number;
  refill_tokens?: number;
  refill_interval_ms?: number;
  tokens?: number;
  as_of_ms?: number;
  limit?: number;
  window_ms?: number;
  window_index?: number;
  count?: number;
  log?: [number, number][]; // [timestamp_ms, cost], oldest first
}

interface Params {
  key?: string;
  algorithm?: string;
  capacity?: number;
  refill_tokens?: number;
  refill_interval_ms?: number;
  limit?: number;
  window_ms?: number;
  cost?: number;
}

class Engine {
  private statePath: string;
  private limiters: Record<string, Limiter> = {};

  constructor(dataDir: string) {
    this.statePath = join(dataDir, "state.json");
    this.load();
  }

  private load(): void {
    try {
      const data = JSON.parse(readFileSync(this.statePath, "utf8"));
      this.limiters = data.limiters ?? {};
    } catch {
      /* no prior state */
    }
  }

  // SIGKILL (not power loss) is the threat model, so an atomic rename is enough
  // — no fsync, which keeps the hot path fast. State is tiny.
  private persist(): void {
    const tmp = this.statePath + ".tmp";
    writeFileSync(tmp, JSON.stringify({ limiters: this.limiters }));
    renameSync(tmp, this.statePath);
  }

  // Bring a limiter up to `now`; return currently available units.
  private refill(l: Limiter, now: number): number {
    if (l.algorithm === "token_bucket") {
      const elapsed = Math.max(0, now - (l.as_of_ms ?? now));
      const accrued = (elapsed / l.refill_interval_ms!) * l.refill_tokens!;
      l.tokens = Math.min(l.capacity!, (l.tokens ?? 0) + accrued);
      l.as_of_ms = now;
      return l.tokens;
    }
    if (l.algorithm === "fixed_window") {
      const idx = Math.floor(now / l.window_ms!);
      if (idx !== l.window_index) {
        l.window_index = idx;
        l.count = 0;
      }
      return l.limit! - (l.count ?? 0);
    }
    // sliding_window
    const cutoff = now - l.window_ms!;
    l.log = (l.log ?? []).filter((e) => e[0] > cutoff);
    const used = l.log.reduce((s, e) => s + e[1], 0);
    return l.limit! - used;
  }

  private limitVal(l: Limiter): number {
    return l.algorithm === "token_bucket" ? l.capacity! : l.limit!;
  }

  private retryAfter(l: Limiter, now: number, cost: number, available: number): number {
    if (available >= cost) return 0;
    if (l.algorithm === "token_bucket") {
      const deficit = cost - (l.tokens ?? 0);
      return Math.ceil((deficit / l.refill_tokens!) * l.refill_interval_ms!);
    }
    if (l.algorithm === "fixed_window") {
      return (l.window_index! + 1) * l.window_ms! - now;
    }
    // sliding_window
    const need = cost - available;
    let freed = 0;
    for (const [ts, c] of l.log ?? []) {
      freed += c;
      if (freed >= need) return ts + l.window_ms! - now;
    }
    return l.window_ms!;
  }

  private consume(l: Limiter, now: number, cost: number): void {
    if (l.algorithm === "token_bucket") l.tokens! -= cost;
    else if (l.algorithm === "fixed_window") l.count = (l.count ?? 0) + cost;
    else (l.log ??= []).push([now, cost]);
  }

  private get(key: string | undefined): Limiter {
    const l = key !== undefined ? this.limiters[key] : undefined;
    if (!l) throw new RateLimiterError("KEY_NOT_FOUND", `no limiter for key ${JSON.stringify(key)}`);
    return l;
  }

  ping(): unknown {
    return { message: "pong" };
  }

  configure(p: Params): unknown {
    if (!p.key) throw new RateLimiterError("INVALID_PARAMS", "configure requires key");
    let l: Limiter;
    if (p.algorithm === "token_bucket") {
      if (p.capacity == null || p.refill_tokens == null || p.refill_interval_ms == null)
        throw new RateLimiterError("INVALID_PARAMS", "token_bucket requires capacity, refill_tokens, refill_interval_ms");
      l = {
        algorithm: "token_bucket",
        capacity: p.capacity,
        refill_tokens: p.refill_tokens,
        refill_interval_ms: p.refill_interval_ms,
        tokens: p.capacity,
        as_of_ms: nowMs(),
      };
    } else if (p.algorithm === "fixed_window" || p.algorithm === "sliding_window") {
      if (p.limit == null || p.window_ms == null)
        throw new RateLimiterError("INVALID_PARAMS", `${p.algorithm} requires limit, window_ms`);
      l = { algorithm: p.algorithm, limit: p.limit, window_ms: p.window_ms };
      if (p.algorithm === "fixed_window") l.window_index = Math.floor(nowMs() / p.window_ms);
      else l.log = [];
    } else if (p.algorithm == null) {
      throw new RateLimiterError("INVALID_PARAMS", "configure requires algorithm");
    } else {
      throw new RateLimiterError("INVALID_ALGORITHM", `unknown algorithm ${JSON.stringify(p.algorithm)}`);
    }
    this.limiters[p.key] = l;
    this.persist();
    return {};
  }

  take(p: Params): unknown {
    const cost = p.cost ?? 1;
    const l = this.get(p.key);
    const now = nowMs();
    let available = this.refill(l, now);
    const limit = this.limitVal(l);
    if (available >= cost) {
      this.consume(l, now, cost);
      available -= cost;
      this.persist();
      return { allowed: true, remaining: Math.floor(available), limit, retry_after_ms: 0 };
    }
    return {
      allowed: false,
      remaining: Math.floor(available),
      limit,
      retry_after_ms: this.retryAfter(l, now, cost, available),
    };
  }

  peek(p: Params): unknown {
    const cost = p.cost ?? 1;
    const l = this.get(p.key);
    const now = nowMs();
    const available = this.refill(l, now);
    return {
      remaining: Math.floor(available),
      limit: this.limitVal(l),
      retry_after_ms: this.retryAfter(l, now, cost, available),
    };
  }

  handle(method: string | undefined, params: Params): unknown {
    switch (method) {
      case "ping":
        return this.ping();
      case "configure":
        return this.configure(params);
      case "take":
        return this.take(params);
      case "peek":
        return this.peek(params);
      default:
        throw new RateLimiterError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
    }
  }
}

// Bun/Node run this server single-threaded, so each request handler runs to
// completion without interleaving — admission is atomic for free.
let engine: Engine;

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
      let req: { id?: string; method?: string; params?: Params };
      try {
        req = JSON.parse(line) as { id?: string; method?: string; params?: Params };
      } catch {
        continue; // skip unparseable lines, like the Go reference
      }
      try {
        const result = engine.handle(req.method, req.params ?? {});
        socket.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as RateLimiterError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
const dataDir = process.argv[process.argv.indexOf("--data-dir") + 1];
mkdirSync(dataDir, { recursive: true });
engine = new Engine(dataDir);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
