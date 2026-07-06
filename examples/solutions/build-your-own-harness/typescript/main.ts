// Reference solution for build-your-own-harness (TypeScript). Passes all 9 stages.
import { spawn } from "node:child_process";
import { createConnection, createServer, type Socket } from "node:net";
import { dirname } from "node:path";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";

class HarnessError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function waitTCP(addr: string, timeoutMs = 10_000): Promise<void> {
  const [host, portStr] = addr.split(":");
  const port = Number(portStr);
  const deadline = Date.now() + timeoutMs;
  return new Promise((resolve, reject) => {
    const tick = () => {
      const conn = createConnection({ host, port }, () => {
        conn.end();
        resolve();
      });
      conn.on("error", () => {
        if (Date.now() >= deadline) reject(new HarnessError("SPAWN_FAILED", `timeout ${addr}`));
        else setTimeout(tick, 100);
      });
    };
    tick();
  });
}

function proxyCall(addr: string, method: string, params: Record<string, unknown> = {}): Promise<Record<string, unknown>> {
  return new Promise((resolve, reject) => {
    const [host, portStr] = addr.split(":");
    const conn = createConnection({ host, port: Number(portStr) });
    let buf = "";
    conn.setEncoding("utf8");
    conn.on("data", (chunk) => {
      buf += chunk;
      const idx = buf.indexOf("\n");
      if (idx === -1) return;
      conn.end();
      const resp = JSON.parse(buf.slice(0, idx)) as { result?: Record<string, unknown>; error?: { code: string; message: string } };
      if (resp.error) reject(new HarnessError(resp.error.code, resp.error.message));
      else resolve(resp.result ?? {});
    });
    conn.on("error", reject);
    conn.write(JSON.stringify({ id: "1", method, params }) + "\n");
  });
}

function subsetMatch(got: Record<string, unknown>, expect: Record<string, unknown>) {
  return Object.entries(expect).every(([k, v]) => JSON.stringify(got[k]) === JSON.stringify(v));
}

class Engine {
  async spawn(program: string): Promise<string> {
    if (!program) throw new HarnessError("INVALID_PARAMS", "spawn requires program");
    const s = createServer();
    await new Promise<void>((r) => s.listen(0, "127.0.0.1", r));
    const port = (s.address() as { port: number }).port;
    s.close();
    const dataDir = await mkdtemp(`${tmpdir()}/harness-child-`);
    const proc = spawn(program, ["--port", String(port), "--data-dir", dataDir], {
      cwd: dirname(program),
      detached: true,
      stdio: "ignore",
    });
    const addr = `127.0.0.1:${port}`;
    await waitTCP(addr);
    return addr;
  }

  async handle(method: string, params: Record<string, unknown>) {
    if (method === "ping") return { message: "pong" };
    if (method === "spawn") return { addr: await this.spawn(params.program as string) };
    if (method === "call") {
      const addr = params.addr as string;
      const m = params.method as string;
      if (!addr || !m) throw new HarnessError("INVALID_PARAMS", "call requires addr and method");
      return proxyCall(addr, m, (params.params as Record<string, unknown>) ?? {});
    }
    if (method === "run_case") {
      const program = params.program as string;
      const m = params.method as string;
      const expect = params.expect as Record<string, unknown>;
      if (!program || !m || !expect) throw new HarnessError("INVALID_PARAMS", "run_case fields required");
      const addr = await this.spawn(program);
      const got = await proxyCall(addr, m, (params.params as Record<string, unknown>) ?? {});
      if (!subsetMatch(got, expect)) throw new HarnessError("CASE_FAILED", `got ${JSON.stringify(got)}`);
      return {};
    }
    throw new HarnessError("UNKNOWN_METHOD", `unknown method ${method}`);
  }
}

const engine = new Engine();

function handleConn(socket: Socket) {
  let buf = "";
  socket.setEncoding("utf8");
  socket.on("data", async (chunk) => {
    buf += chunk;
    let idx: number;
    while ((idx = buf.indexOf("\n")) !== -1) {
      const line = buf.slice(0, idx);
      buf = buf.slice(idx + 1);
      if (!line.trim()) continue;
      const req = JSON.parse(line) as { id?: string; method?: string; params?: Record<string, unknown> };
      try {
        const result = await engine.handle(req.method ?? "", req.params ?? {});
        socket.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as HarnessError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
