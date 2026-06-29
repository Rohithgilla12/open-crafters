// Reference solution for "Build your own object store" (TypeScript, Bun). Passes all 9 stages.

import { createHash, randomBytes } from "node:crypto";
import { createServer, type Socket } from "node:net";
import { mkdirSync, readFileSync, renameSync, writeFileSync } from "node:fs";
import { join } from "node:path";

class ObjectStoreError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function bodyETag(body: Buffer): string {
  return createHash("sha256").update(body).digest("hex");
}

type Multipart = { key: string; parts: Map<number, Buffer> };

class Store {
  objects = new Map<string, Buffer>();
  uploads = new Map<string, Multipart>();
  constructor(private snapPath: string) {
    this.load();
  }

  load(): void {
    try {
      const st = JSON.parse(readFileSync(this.snapPath, "utf8")) as {
        objects_b64?: Record<string, string>;
        uploads?: Record<string, { key: string; parts: { part_number: number; body_b64: string }[] }>;
      };
      for (const [k, b64] of Object.entries(st.objects_b64 ?? {})) {
        this.objects.set(k, Buffer.from(b64, "base64"));
      }
      for (const [id, pu] of Object.entries(st.uploads ?? {})) {
        const parts = new Map<number, Buffer>();
        for (const p of pu.parts) parts.set(p.part_number, Buffer.from(p.body_b64, "base64"));
        this.uploads.set(id, { key: pu.key, parts });
      }
    } catch {
      /* fresh store */
    }
  }

  persist(): void {
    const st: {
      objects_b64: Record<string, string>;
      uploads: Record<string, { key: string; parts: { part_number: number; body_b64: string }[] }>;
    } = { objects_b64: {}, uploads: {} };
    for (const [k, body] of this.objects) st.objects_b64[k] = body.toString("base64");
    for (const [id, mp] of this.uploads) {
      const parts = [...mp.parts.entries()]
        .sort((a, b) => a[0] - b[0])
        .map(([num, body]) => ({ part_number: num, body_b64: body.toString("base64") }));
      st.uploads[id] = { key: mp.key, parts };
    }
    const tmp = this.snapPath + ".tmp";
    writeFileSync(tmp, JSON.stringify(st));
    renameSync(tmp, this.snapPath);
  }

  handle(method: string, params: Record<string, unknown>): unknown {
    if (method === "ping") return { message: "pong" };
    if (method === "put") {
      const key = params.key as string;
      const body = Buffer.from(params.body as string);
      this.objects.set(key, body);
      this.persist();
      return { etag: bodyETag(body) };
    }
    if (method === "get") {
      const body = this.objects.get(params.key as string);
      if (!body) throw new ObjectStoreError("NOT_FOUND", `no such key ${JSON.stringify(params.key)}`);
      return { found: true, body: body.toString(), etag: bodyETag(body), size: body.length };
    }
    if (method === "head") {
      const body = this.objects.get(params.key as string);
      if (!body) throw new ObjectStoreError("NOT_FOUND", `no such key ${JSON.stringify(params.key)}`);
      return { found: true, etag: bodyETag(body), size: body.length };
    }
    if (method === "delete") {
      const key = params.key as string;
      if (!this.objects.has(key)) return { deleted: false };
      this.objects.delete(key);
      this.persist();
      return { deleted: true };
    }
    if (method === "list") {
      const prefix = (params.prefix as string) ?? "";
      const keys = [...this.objects.keys()].filter((k) => k.startsWith(prefix)).sort();
      return { keys };
    }
    if (method === "create_multipart") {
      const id = randomBytes(16).toString("hex");
      this.uploads.set(id, { key: params.key as string, parts: new Map() });
      this.persist();
      return { upload_id: id };
    }
    if (method === "upload_part") {
      const mp = this.uploads.get(params.upload_id as string);
      if (!mp) throw new ObjectStoreError("NO_SUCH_UPLOAD", `no upload ${JSON.stringify(params.upload_id)}`);
      const body = Buffer.from(params.body as string);
      mp.parts.set(params.part_number as number, body);
      this.persist();
      return { etag: bodyETag(body) };
    }
    if (method === "complete_multipart") {
      const mp = this.uploads.get(params.upload_id as string);
      if (!mp) throw new ObjectStoreError("NO_SUCH_UPLOAD", `no upload ${JSON.stringify(params.upload_id)}`);
      const parts = params.parts as { part_number: number; etag: string }[];
      if (!parts?.length) throw new ObjectStoreError("INVALID_PART", "no parts provided");
      let prev = 0;
      const assembled: Buffer[] = [];
      for (let i = 0; i < parts.length; i++) {
        const p = parts[i]!;
        if (i > 0 && p.part_number <= prev) throw new ObjectStoreError("INVALID_PART", "parts must be in ascending part_number order");
        const raw = mp.parts.get(p.part_number);
        if (!raw) throw new ObjectStoreError("INVALID_PART", `missing part ${p.part_number}`);
        if (bodyETag(raw) !== p.etag) throw new ObjectStoreError("INVALID_PART", `etag mismatch for part ${p.part_number}`);
        assembled.push(raw);
        prev = p.part_number;
      }
      this.uploads.delete(params.upload_id as string);
      const full = Buffer.concat(assembled);
      this.objects.set(mp.key, full);
      this.persist();
      return { etag: bodyETag(full) };
    }
    throw new ObjectStoreError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
  }
}

function handleConn(store: Store, socket: Socket): void {
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
        const result = store.handle(req.method ?? "", req.params ?? {});
        socket.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as ObjectStoreError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
const dataDir = process.argv[process.argv.indexOf("--data-dir") + 1]!;
mkdirSync(dataDir, { recursive: true });
const store = new Store(join(dataDir, "state.json"));
createServer((s) => handleConn(store, s)).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
