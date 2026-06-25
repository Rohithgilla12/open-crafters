// Reference solution for the open-crafters "Build your own LSM-tree" challenge
// (TypeScript, run with Bun). Passes all 9 stages.

import { createServer, type Socket } from "node:net";
import {
  closeSync,
  fsyncSync,
  mkdirSync,
  openSync,
  readdirSync,
  readFileSync,
  unlinkSync,
  writeFileSync,
  writeSync,
} from "node:fs";
import { join } from "node:path";

const SST_MAGIC = Buffer.from("SST1");

type MemEntry = { value: string; deleted: boolean };

class RpcError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function encodeSST(entries: { key: string; value: string; deleted: boolean }[]): Buffer {
  const parts: Buffer[] = [SST_MAGIC, Buffer.alloc(4)];
  parts[1]!.writeUInt32LE(entries.length, 0);
  for (const e of entries) {
    const keyB = Buffer.from(e.key, "utf8");
    const valB = e.deleted ? Buffer.alloc(0) : Buffer.from(e.value, "utf8");
    const keyLen = Buffer.alloc(4);
    keyLen.writeUInt32LE(keyB.length, 0);
    const valLen = Buffer.alloc(4);
    valLen.writeUInt32LE(valB.length, 0);
    parts.push(keyLen, keyB, valLen, valB);
  }
  return Buffer.concat(parts);
}

function parseSST(data: Buffer): { key: string; value: string; deleted: boolean }[] {
  if (data.length < 8 || !data.subarray(0, 4).equals(SST_MAGIC)) throw new Error("invalid SST");
  const count = data.readUInt32LE(4);
  let offset = 8;
  const out: { key: string; value: string; deleted: boolean }[] = [];
  for (let i = 0; i < count; i++) {
    const keyLen = data.readUInt32LE(offset);
    offset += 4;
    const key = data.subarray(offset, offset + keyLen).toString("utf8");
    offset += keyLen;
    const valLen = data.readUInt32LE(offset);
    offset += 4;
    const value = data.subarray(offset, offset + valLen).toString("utf8");
    offset += valLen;
    out.push({ key, value, deleted: valLen === 0 });
  }
  return out;
}

class Store {
  private mem = new Map<string, MemEntry>();
  private sstFiles: string[] = [];
  private nextSeq = 1;

  constructor(private sstDir: string) {
    mkdirSync(sstDir, { recursive: true });
    this.loadIndex();
  }

  private loadIndex(): void {
    const names = readdirSync(this.sstDir)
      .filter((n) => n.endsWith(".sst"))
      .sort();
    this.sstFiles = names.map((n) => join(this.sstDir, n));
    if (names.length > 0) {
      this.nextSeq = parseInt(names[names.length - 1]!.slice(0, 6), 10) + 1;
    }
  }

  private readSST(path: string) {
    return parseSST(readFileSync(path));
  }

  private writeSST(entries: { key: string; value: string; deleted: boolean }[]): string {
    const path = join(this.sstDir, `${String(this.nextSeq).padStart(6, "0")}.sst`);
    const data = encodeSST(entries);
    writeFileSync(path, data);
    const fd = openSync(path, "r+");
    fsyncSync(fd);
    closeSync(fd);
    this.sstFiles.push(path);
    this.nextSeq++;
    return path;
  }

  private lookup(key: string): { value: string; found: boolean } {
    const mem = this.mem.get(key);
    if (mem) return mem.deleted ? { value: "", found: false } : { value: mem.value, found: true };
    for (let i = this.sstFiles.length - 1; i >= 0; i--) {
      for (const e of this.readSST(this.sstFiles[i]!)) {
        if (e.key === key) return e.deleted ? { value: "", found: false } : { value: e.value, found: true };
      }
    }
    return { value: "", found: false };
  }

  private mergedLive(): Map<string, string> {
    const resolved = new Map<string, { value: string; deleted: boolean }>();
    for (const path of this.sstFiles) {
      for (const e of this.readSST(path)) {
        resolved.set(e.key, { value: e.value, deleted: e.deleted });
      }
    }
    for (const [key, e] of this.mem) {
      resolved.set(key, { value: e.value, deleted: e.deleted });
    }
    const live = new Map<string, string>();
    for (const [key, e] of resolved) {
      if (!e.deleted) live.set(key, e.value);
    }
    return live;
  }

  handle(method: string, params: Record<string, string>): unknown {
    switch (method) {
      case "ping":
        return { message: "pong" };
      case "put":
        this.mem.set(params.key!, { value: params.value!, deleted: false });
        return {};
      case "get": {
        const { value, found } = this.lookup(params.key!);
        return found ? { value, found: true } : { value: null, found: false };
      }
      case "del": {
        const existed = this.lookup(params.key!).found;
        this.mem.set(params.key!, { value: "", deleted: true });
        return { deleted: existed };
      }
      case "flush": {
        if (this.mem.size === 0) return {};
        const entries = [...this.mem.entries()]
          .map(([key, e]) => ({ key, value: e.value, deleted: e.deleted }))
          .sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0));
        this.writeSST(entries);
        this.mem.clear();
        return {};
      }
      case "scan": {
        const live = this.mergedLive();
        const keys = [...live.keys()].filter((k) => k >= params.start! && k < params.end!).sort();
        return { entries: keys.map((key) => ({ key, value: live.get(key)! })) };
      }
      case "compact": {
        if (this.sstFiles.length < 2) return {};
        const resolved = new Map<string, { value: string; deleted: boolean }>();
        for (const path of this.sstFiles) {
          for (const e of this.readSST(path)) resolved.set(e.key, e);
        }
        const entries = [...resolved.entries()]
          .map(([key, e]) => ({ key, value: e.value, deleted: e.deleted }))
          .sort((a, b) => (a.key < b.key ? -1 : a.key > b.key ? 1 : 0));
        const old = [...this.sstFiles];
        this.writeSST(entries);
        this.sstFiles = this.sstFiles.filter((p) => {
          if (old.includes(p)) {
            unlinkSync(p);
            return false;
          }
          return true;
        });
        return {};
      }
      default:
        throw new RpcError("UNKNOWN_METHOD", `unknown method ${method}`);
    }
  }
}

const METHODS = new Set(["ping", "put", "get", "del", "flush", "scan", "compact"]);

function serveSocket(socket: Socket, store: Store): void {
  let buffer = "";
  socket.setEncoding("utf8");
  socket.on("data", (chunk) => {
    buffer += chunk;
    let idx: number;
    while ((idx = buffer.indexOf("\n")) >= 0) {
      const line = buffer.slice(0, idx).trim();
      buffer = buffer.slice(idx + 1);
      if (!line) continue;
      let requestId: string | undefined;
      try {
        const request = JSON.parse(line) as {
          id?: string;
          method?: string;
          params?: Record<string, string>;
        };
        requestId = request.id;
        const method = request.method ?? "";
        if (!METHODS.has(method)) throw new RpcError("UNKNOWN_METHOD", `unknown method ${method}`);
        const result = store.handle(method, request.params ?? {});
        socket.write(JSON.stringify({ id: requestId, result }) + "\n");
      } catch (e) {
        const err = e as RpcError;
        socket.write(
          JSON.stringify({
            id: requestId,
            error: { code: err.code ?? "BAD_REQUEST", message: String(err.message ?? e) },
          }) + "\n",
        );
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
const dataDir = process.argv[process.argv.indexOf("--data-dir") + 1]!;
const store = new Store(join(dataDir, "sst"));

createServer((socket) => serveSocket(socket, store)).listen(port, "127.0.0.1", () => {
  console.log(`lsm kv listening on 127.0.0.1:${port}`);
});
