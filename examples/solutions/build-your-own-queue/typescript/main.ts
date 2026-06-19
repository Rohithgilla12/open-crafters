// Reference solution for "Build your own message queue" (TypeScript, run with Bun).
//
// Durable broker with at-least-once delivery: visibility timeouts, nack,
// receipt fencing, and dead-letter queues after max_receives. Un-acked
// messages survive SIGKILL; acked ones stay gone (atomic state snapshot).
// Uses only node: APIs. Passes all 9 stages.

import { createServer, type Socket } from "node:net";
import { closeSync, fsyncSync, openSync, readFileSync, renameSync, writeSync } from "node:fs";
import { randomUUID } from "node:crypto";
import { join } from "node:path";

class RpcError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

interface Msg {
  id: string;
  body: string;
  seq: number;
  receives: number;
  inflight: boolean;
  invisibleUntil: number; // epoch ms
  receipt: string | null;
}

interface Queue {
  messages: Map<string, Msg>;
  maxReceives: number | null;
  dlq: string | null;
}

const newQueue = (): Queue => ({ messages: new Map(), maxReceives: null, dlq: null });

interface PersistedMsg {
  id: string;
  body: string;
  seq: number;
  receives?: number;
}
interface PersistedQueue {
  max_receives?: number | null;
  dead_letter_queue?: string | null;
  messages?: PersistedMsg[];
}
interface PersistedState {
  seq?: number;
  queues?: Record<string, PersistedQueue>;
}

class Broker {
  private queues = new Map<string, Queue>();
  private seq = 0;
  private snapPath: string;

  constructor(dataDir: string) {
    this.snapPath = join(dataDir, "state.json");
    this.recover();
  }

  private recover(): void {
    let data: string;
    try {
      data = readFileSync(this.snapPath, "utf8");
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code === "ENOENT") return;
      throw e;
    }
    const st = JSON.parse(data) as PersistedState;
    this.seq = st.seq ?? 0;
    for (const [name, q] of Object.entries(st.queues ?? {})) {
      const queue = newQueue();
      queue.maxReceives = q.max_receives ?? null;
      queue.dlq = q.dead_letter_queue ?? null;
      for (const m of q.messages ?? []) {
        // Un-acked messages come back visible; in-flight state is not durable.
        queue.messages.set(m.id, {
          id: m.id, body: m.body, seq: m.seq, receives: m.receives ?? 0,
          inflight: false, invisibleUntil: 0, receipt: null,
        });
      }
      this.queues.set(name, queue);
    }
  }

  private persist(): void {
    const queues: Record<string, unknown> = {};
    for (const [name, q] of this.queues) {
      queues[name] = {
        max_receives: q.maxReceives,
        dead_letter_queue: q.dlq,
        messages: [...q.messages.values()].map((m) => ({ id: m.id, body: m.body, seq: m.seq, receives: m.receives })),
      };
    }
    const tmp = this.snapPath + ".tmp";
    const fd = openSync(tmp, "w");
    writeSync(fd, JSON.stringify({ seq: this.seq, queues }));
    fsyncSync(fd);
    closeSync(fd);
    renameSync(tmp, this.snapPath);
  }

  private queue(name: string): Queue {
    let q = this.queues.get(name);
    if (!q) {
      q = newQueue();
      this.queues.set(name, q);
    }
    return q;
  }

  private nextSeq(): number {
    return ++this.seq;
  }

  private maybeDeadLetter(q: Queue, m: Msg): boolean {
    if (q.maxReceives === null || m.receives < q.maxReceives) return false;
    q.messages.delete(m.id);
    const dlq = this.queue(q.dlq!);
    dlq.messages.set(m.id, { id: m.id, body: m.body, seq: this.nextSeq(), receives: 0, inflight: false, invisibleUntil: 0, receipt: null });
    return true;
  }

  private expire(q: Queue, now: number): boolean {
    let changed = false;
    for (const m of [...q.messages.values()]) {
      if (m.inflight && m.invisibleUntil <= now) {
        if (this.maybeDeadLetter(q, m)) changed = true;
        else {
          m.inflight = false;
          m.receipt = null;
        }
      }
    }
    return changed;
  }

  private findInflight(q: Queue, receipt: string): Msg | undefined {
    for (const m of q.messages.values()) {
      if (m.inflight && m.receipt === receipt) return m;
    }
    return undefined;
  }

  handle(method: string, p: Record<string, unknown>): unknown {
    const str = (k: string): string => p[k] as string;
    const num = (k: string): number | undefined => p[k] as number | undefined;
    switch (method) {
      case "ping":
        return { message: "pong" };

      case "send": {
        const q = this.queue(str("queue"));
        const id = randomUUID();
        q.messages.set(id, { id, body: str("body"), seq: this.nextSeq(), receives: 0, inflight: false, invisibleUntil: 0, receipt: null });
        this.persist();
        return { id };
      }

      case "receive": {
        const timeout = num("visibility_timeout_ms") ?? 30000;
        const q = this.queue(str("queue"));
        const now = Date.now();
        if (this.expire(q, now)) this.persist();
        let pick: Msg | undefined;
        for (const m of q.messages.values()) {
          if (!m.inflight && (!pick || m.seq < pick.seq)) pick = m;
        }
        if (!pick) return { message: null };
        pick.receives++;
        pick.inflight = true;
        pick.invisibleUntil = now + timeout;
        pick.receipt = randomUUID();
        return { message: { id: pick.id, body: pick.body, receipt: pick.receipt, receives: pick.receives } };
      }

      case "ack": {
        const q = this.queue(str("queue"));
        const m = this.findInflight(q, str("receipt"));
        if (!m) return { acked: false };
        q.messages.delete(m.id);
        this.persist();
        return { acked: true };
      }

      case "nack": {
        const q = this.queue(str("queue"));
        const m = this.findInflight(q, str("receipt"));
        if (!m) return { nacked: false };
        if (this.maybeDeadLetter(q, m)) this.persist();
        else {
          m.inflight = false;
          m.invisibleUntil = 0;
          m.receipt = null;
        }
        return { nacked: true };
      }

      case "stats": {
        const q = this.queues.get(str("queue"));
        if (!q) return { visible: 0, inflight: 0 };
        if (this.expire(q, Date.now())) this.persist();
        let visible = 0,
          inflight = 0;
        for (const m of q.messages.values()) m.inflight ? inflight++ : visible++;
        return { visible, inflight };
      }

      case "configure": {
        const q = this.queue(str("queue"));
        q.maxReceives = num("max_receives") ?? null;
        q.dlq = str("dead_letter_queue");
        this.persist();
        return {};
      }

      default:
        throw new RpcError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
    }
  }
}

function handleConnection(socket: Socket, broker: Broker): void {
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
        response = { id: req.id, result: broker.handle(req.method ?? "", req.params ?? {}) };
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
const broker = new Broker(dataDir);
createServer((socket) => handleConnection(socket, broker)).listen(port, "127.0.0.1", () => {
  console.log(`message broker listening on 127.0.0.1:${port}`);
});
