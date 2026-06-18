// Reference solution for the open-crafters "Build your own WAL" challenge
// (TypeScript, run with Bun).
//
// A key-value store made durable by a write-ahead log:
//   - record format: crc32(4, LE) | length(4, LE) | JSON payload, with the CRC
//     covering the length bytes followed by the payload (see PROTOCOL.md)
//   - fsync before acknowledging any write
//   - recovery replays wal.log on top of snapshot.json, stops at the first
//     invalid record, and truncates the torn/corrupt tail before accepting new
//     appends
//   - checkpoint: atomically snapshot full state, then reset the log
//
// Uses only node: APIs (so it runs under Bun or Node) and a hand-rolled CRC-32
// to stay independent of any runtime's zlib. Passes all 9 stages.

import { createServer, type Socket } from "node:net";
import {
  closeSync,
  fsyncSync,
  ftruncateSync,
  openSync,
  readFileSync,
  renameSync,
  writeSync,
} from "node:fs";
import { join } from "node:path";

const CRC_TABLE = (() => {
  const table = new Uint32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    table[n] = c >>> 0;
  }
  return table;
})();

function crc32(buf: Buffer): number {
  let crc = 0xffffffff;
  for (let i = 0; i < buf.length; i++) crc = (CRC_TABLE[(crc ^ buf[i]!) & 0xff]! ^ (crc >>> 8)) >>> 0;
  return (crc ^ 0xffffffff) >>> 0;
}

interface Record {
  op: "set" | "del";
  key: string;
  value?: string;
}

function encodeRecord(rec: Record): Buffer {
  const body = Buffer.from(JSON.stringify(rec), "utf8");
  const buf = Buffer.alloc(8 + body.length);
  buf.writeUInt32LE(body.length, 4);
  body.copy(buf, 8);
  buf.writeUInt32LE(crc32(buf.subarray(4)), 0);
  return buf;
}

class RpcError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

class Store {
  private data = new Map<string, string>();
  private walPath: string;
  private snapPath: string;
  private walFd: number;

  constructor(dataDir: string) {
    this.walPath = join(dataDir, "wal.log");
    this.snapPath = join(dataDir, "snapshot.json");
    this.recover();
    this.walFd = openSync(this.walPath, "a");
  }

  private readFileOrNull(path: string): Buffer | null {
    try {
      return readFileSync(path);
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code === "ENOENT") return null;
      throw e;
    }
  }

  private recover(): void {
    const snap = this.readFileOrNull(this.snapPath);
    if (snap) {
      const parsed = JSON.parse(snap.toString("utf8")) as { data: Record<string, string> };
      for (const [k, v] of Object.entries(parsed.data)) this.data.set(k, v);
    }

    const raw = this.readFileOrNull(this.walPath);
    if (!raw) return;

    let offset = 0;
    let validEnd = 0;
    while (offset + 8 <= raw.length) {
      const storedCRC = raw.readUInt32LE(offset);
      const length = raw.readUInt32LE(offset + 4);
      if (offset + 8 + length > raw.length) break; // torn payload
      const framed = raw.subarray(offset + 4, offset + 8 + length);
      if (crc32(framed) !== storedCRC) break; // corrupt record: stop replay here
      const rec = JSON.parse(raw.subarray(offset + 8, offset + 8 + length).toString("utf8")) as Record;
      if (rec.op === "set") this.data.set(rec.key, rec.value ?? "");
      else if (rec.op === "del") this.data.delete(rec.key);
      offset += 8 + length;
      validEnd = offset;
    }

    if (validEnd < raw.length) {
      // Drop the torn/corrupt tail so the log parses cleanly from byte 0 and
      // new appends don't land after garbage.
      const fd = openSync(this.walPath, "r+");
      ftruncateSync(fd, validEnd);
      fsyncSync(fd);
      closeSync(fd);
    }
  }

  private append(rec: Record): void {
    writeSync(this.walFd, encodeRecord(rec));
    fsyncSync(this.walFd);
  }

  ping(): unknown {
    return { message: "pong" };
  }

  set(params: { key: string; value: string }): unknown {
    this.append({ op: "set", key: params.key, value: params.value });
    this.data.set(params.key, params.value);
    return {};
  }

  get(params: { key: string }): unknown {
    if (this.data.has(params.key)) return { value: this.data.get(params.key), found: true };
    return { value: null, found: false };
  }

  del(params: { key: string }): unknown {
    const existed = this.data.has(params.key);
    this.append({ op: "del", key: params.key });
    this.data.delete(params.key);
    return { deleted: existed };
  }

  checkpoint(): unknown {
    // Snapshot must be durable BEFORE the log is reset: a crash in between just
    // replays the old log onto the new snapshot, which is harmless because
    // set/del are absolute.
    const body = JSON.stringify({ data: Object.fromEntries(this.data) });
    const tmp = this.snapPath + ".tmp";
    const fd = openSync(tmp, "w");
    writeSync(fd, body);
    fsyncSync(fd);
    closeSync(fd);
    renameSync(tmp, this.snapPath);

    closeSync(this.walFd);
    const reset = openSync(this.walPath, "w"); // truncate to empty
    fsyncSync(reset);
    closeSync(reset);
    this.walFd = openSync(this.walPath, "a");
    return {};
  }

  handle(method: string, params: Record<string, unknown>): unknown {
    switch (method) {
      case "ping":
        return this.ping();
      case "set":
        return this.set(params as { key: string; value: string });
      case "get":
        return this.get(params as { key: string });
      case "del":
        return this.del(params as { key: string });
      case "checkpoint":
        return this.checkpoint();
      default:
        throw new RpcError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
    }
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
  console.log(`kv store listening on 127.0.0.1:${port}`);
});
