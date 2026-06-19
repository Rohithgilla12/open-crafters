// Reference solution for "Build your own MVCC" (TypeScript, run with Bun).
//
// Transactional KV with snapshot isolation: begin captures a snapshot, reads
// are multi-version and frozen, commit is durable and assigns a monotonic
// sequence, and a write-write conflict aborts with CONFLICT. Recovery replays
// the commit log. Uses only node: APIs. Passes all 9 stages.

import { createServer, type Socket } from "node:net";
import { fsyncSync, openSync, readFileSync, writeSync } from "node:fs";
import { join } from "node:path";

class RpcError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

interface Version {
  seq: number;
  val: string | null; // null = tombstone
}
interface Txn {
  snapshot: number;
  writes: Map<string, string | null>;
}
interface CommitRecord {
  seq: number;
  writes: Record<string, string | null>;
}

class Store {
  private versions = new Map<string, Version[]>();
  private commitSeq = 0;
  private txns = new Map<string, Txn>();
  private txnCounter = 0;
  private logPath: string;
  private logFd: number;

  constructor(dataDir: string) {
    this.logPath = join(dataDir, "commits.log");
    this.recover();
    this.logFd = openSync(this.logPath, "a");
  }

  private apply(rec: CommitRecord): void {
    for (const [key, val] of Object.entries(rec.writes)) {
      const list = this.versions.get(key) ?? [];
      list.push({ seq: rec.seq, val });
      this.versions.set(key, list);
    }
    if (rec.seq > this.commitSeq) this.commitSeq = rec.seq;
  }

  private recover(): void {
    let data: string;
    try {
      data = readFileSync(this.logPath, "utf8");
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code === "ENOENT") return;
      throw e;
    }
    for (const line of data.split("\n")) {
      if (line.trim()) this.apply(JSON.parse(line) as CommitRecord);
    }
  }

  private readCommitted(key: string, snapshot: number): { value: string | null; found: boolean } {
    let found: string | null = null;
    for (const v of this.versions.get(key) ?? []) {
      if (v.seq <= snapshot) found = v.val;
    }
    if (found === null) return { value: null, found: false };
    return { value: found, found: true };
  }

  private txnOf(params: Record<string, unknown>): Txn {
    const t = this.txns.get(params.txn as string);
    if (!t) throw new RpcError("UNKNOWN_TXN", `no open transaction ${JSON.stringify(params.txn)}`);
    return t;
  }

  handle(method: string, params: Record<string, unknown>): unknown {
    switch (method) {
      case "ping":
        return { message: "pong" };

      case "begin": {
        const id = "t" + ++this.txnCounter;
        this.txns.set(id, { snapshot: this.commitSeq, writes: new Map() });
        return { txn: id };
      }

      case "get": {
        const t = this.txnOf(params);
        const key = params.key as string;
        if (t.writes.has(key)) {
          const val = t.writes.get(key)!;
          return val === null ? { value: null, found: false } : { value: val, found: true };
        }
        return this.readCommitted(key, t.snapshot);
      }

      case "set": {
        const t = this.txnOf(params);
        t.writes.set(params.key as string, params.value as string);
        return {};
      }

      case "delete": {
        const t = this.txnOf(params);
        t.writes.set(params.key as string, null);
        return {};
      }

      case "commit": {
        const id = params.txn as string;
        const t = this.txnOf(params);
        for (const key of t.writes.keys()) {
          const hist = this.versions.get(key);
          if (hist && hist[hist.length - 1]!.seq > t.snapshot) {
            this.txns.delete(id);
            throw new RpcError("CONFLICT", `key ${JSON.stringify(key)} was modified by a concurrent transaction`);
          }
        }
        if (t.writes.size > 0) {
          this.commitSeq++;
          const rec: CommitRecord = { seq: this.commitSeq, writes: Object.fromEntries(t.writes) };
          this.persist(rec);
          this.apply(rec);
        }
        this.txns.delete(id);
        return { committed: true };
      }

      case "rollback": {
        const id = params.txn as string;
        this.txnOf(params);
        this.txns.delete(id);
        return {};
      }

      default:
        throw new RpcError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
    }
  }

  private persist(rec: CommitRecord): void {
    writeSync(this.logFd, JSON.stringify(rec) + "\n");
    fsyncSync(this.logFd);
  }
}

function handleConnection(socket: Socket, store: Store): void {
  let buffer = "";
  socket.on("data", (chunk) => {
    buffer += chunk.toString("utf8");
    let nl: number;
    while ((nl = buffer.indexOf("\n")) >= 0) {
      const line = buffer.slice(0, nl);
      buffer = buffer.slice(nl + 1);
      if (!line.trim()) continue;
      const req = JSON.parse(line) as { id?: string; method?: string; params?: Record<string, unknown> };
      let response: unknown;
      try {
        response = { id: req.id, result: store.handle(req.method ?? "", req.params ?? {}) };
      } catch (e) {
        const err = e instanceof RpcError ? e : new RpcError("BAD_REQUEST", String(e));
        response = { id: req.id, error: { code: err.code, message: err.message } };
      }
      socket.write(JSON.stringify(response) + "\n");
    }
  });
}

function parseArgs(): { port: number; dataDir: string } {
  const args = process.argv.slice(2);
  let port = 0;
  let dataDir = "";
  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--port") port = Number(args[++i]);
    else if (args[i] === "--data-dir") dataDir = args[++i]!;
  }
  return { port, dataDir };
}

const { port, dataDir } = parseArgs();
const store = new Store(dataDir);
createServer((socket) => handleConnection(socket, store)).listen(port, "127.0.0.1", () => {
  console.log(`mvcc store listening on 127.0.0.1:${port}`);
});
