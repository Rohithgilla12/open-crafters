// Reference solution for Build your own ID generator (TypeScript, Bun). Passes all 9 stages.

import { createServer, type Socket } from "node:net";
import { readFileSync, renameSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const SNOWFLAKE_EPOCH_MS = 1577836800000;
const MAX_WORKER_ID = 1023;
const MAX_SEQUENCE = 4095;
const MAX_BATCH = 1024;

class IDError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

const nowMS = () => Date.now();

const composeID = (ts: number, worker: number, seq: number) => {
  const rel = Math.max(0, ts - SNOWFLAKE_EPOCH_MS);
  return String((BigInt(rel) << 22n) | (BigInt(worker) << 12n) | BigInt(seq));
};

const decodeID = (id: bigint) => {
  const sequence = Number(id & BigInt(MAX_SEQUENCE));
  const worker = Number((id >> 12n) & BigInt(MAX_WORKER_ID));
  const rel = Number(id >> 22n);
  return { timestamp_ms: rel + SNOWFLAKE_EPOCH_MS, worker_id: worker, sequence };
};

class Engine {
  private workerID = 0;
  private lastTS = 0;
  private lastSeq = 0;
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
        last_timestamp_ms?: number;
        last_sequence?: number;
        worker_id?: number;
      };
      this.lastTS = data.last_timestamp_ms ?? 0;
      this.lastSeq = data.last_sequence ?? 0;
      this.workerID = data.worker_id ?? 0;
    } catch {
      /* fresh */
    }
  }

  private persist(): void {
    const tmp = this.statePath + ".tmp";
    writeFileSync(
      tmp,
      JSON.stringify({
        last_timestamp_ms: this.lastTS,
        last_sequence: this.lastSeq,
        worker_id: this.workerID,
      }),
    );
    renameSync(tmp, this.statePath);
  }

  private waitNextMS(last: number): Promise<number> {
    return new Promise((resolve) => {
      const tick = () => {
        const n = nowMS();
        if (n > last) resolve(n);
        else setTimeout(tick, 1);
      };
      tick();
    });
  }

  private allocInBucket(bucket: { ts: number }): string {
    if (bucket.ts < this.lastTS) throw new IDError("CLOCK_BACKWARDS", "clock moved backwards");
    if (this.lastTS === bucket.ts) {
      if (this.lastSeq >= MAX_SEQUENCE) {
        throw new IDError("INTERNAL", "sync wait needed");
      }
      this.lastSeq += 1;
    } else {
      this.lastTS = bucket.ts;
      this.lastSeq = 0;
    }
    return composeID(this.lastTS, this.workerID, this.lastSeq);
  }

  private async allocInBucketAsync(bucket: { ts: number }): Promise<string> {
    if (bucket.ts < this.lastTS) throw new IDError("CLOCK_BACKWARDS", "clock moved backwards");
    if (this.lastTS === bucket.ts) {
      if (this.lastSeq >= MAX_SEQUENCE) {
        bucket.ts = await this.waitNextMS(bucket.ts);
        if (bucket.ts < this.lastTS) throw new IDError("CLOCK_BACKWARDS", "clock moved backwards");
        this.lastTS = bucket.ts;
        this.lastSeq = 0;
      } else {
        this.lastSeq += 1;
      }
    } else {
      this.lastTS = bucket.ts;
      this.lastSeq = 0;
    }
    return composeID(this.lastTS, this.workerID, this.lastSeq);
  }

  async configure(workerID: number): Promise<void> {
    if (workerID < 0 || workerID > MAX_WORKER_ID) {
      throw new IDError("INVALID_PARAMS", "worker_id must be 0..1023");
    }
    await this.withLock(() => {
      this.workerID = workerID;
      this.persist();
    });
  }

  async nextID(): Promise<string> {
    return this.withLock(async () => {
      const bucket = { ts: nowMS() };
      const id = await this.allocInBucketAsync(bucket);
      this.persist();
      return id;
    });
  }

  async batch(count: number): Promise<string[]> {
    if (count < 1) throw new IDError("INVALID_PARAMS", "count must be >= 1");
    if (count > MAX_BATCH) throw new IDError("BATCH_TOO_LARGE", `count must be <= ${MAX_BATCH}`);
    return this.withLock(async () => {
      const bucket = { ts: nowMS() };
      const ids: string[] = [];
      for (let i = 0; i < count; i++) {
        ids.push(await this.allocInBucketAsync(bucket));
      }
      this.persist();
      return ids;
    });
  }

  parse(idStr: string): Record<string, number> {
    let val: bigint;
    try {
      val = BigInt(idStr);
    } catch {
      throw new IDError("INVALID_PARAMS", "id must be a positive decimal integer");
    }
    if (val <= 0n) throw new IDError("INVALID_PARAMS", "id must be a positive decimal integer");
    return decodeID(val);
  }
}

async function dispatch(eng: Engine, method: string, params: Record<string, unknown>) {
  if (method === "ping") return { message: "pong" };
  if (method === "configure") {
    if (params.worker_id === undefined) throw new IDError("INVALID_PARAMS", "configure requires worker_id");
    await eng.configure(Number(params.worker_id));
    return {};
  }
  if (method === "next_id") return { id: await eng.nextID() };
  if (method === "batch") {
    if (params.count === undefined) throw new IDError("INVALID_PARAMS", "batch requires count");
    return { ids: await eng.batch(Number(params.count)) };
  }
  if (method === "parse") {
    if (!params.id) throw new IDError("INVALID_PARAMS", "parse requires id");
    return eng.parse(String(params.id));
  }
  throw new IDError("UNKNOWN_METHOD", `unknown method ${method}`);
}

function handleConn(eng: Engine, sock: Socket) {
  let buf = "";
  sock.on("data", async (chunk) => {
    buf += chunk.toString();
    let idx: number;
    while ((idx = buf.indexOf("\n")) >= 0) {
      const line = buf.slice(0, idx);
      buf = buf.slice(idx + 1);
      if (!line.trim()) continue;
      const req = JSON.parse(line) as { id?: string; method?: string; params?: Record<string, unknown> };
      try {
        const result = await dispatch(eng, req.method ?? "", req.params ?? {});
        sock.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as IDError;
        sock.write(
          JSON.stringify({
            id: req.id,
            error: { code: err.code ?? "INTERNAL", message: err.message },
          }) + "\n",
        );
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
const dataDir = process.argv[process.argv.indexOf("--data-dir") + 1];
const eng = new Engine(join(dataDir, "state.json"));

createServer((sock) => handleConn(eng, sock)).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
