// Reference solution for "Build your own scheduler" (TypeScript, Bun). Passes all 9 stages.

import { createServer, type Socket } from "node:net";
import { closeSync, fsyncSync, openSync, readFileSync, renameSync, writeSync } from "node:fs";
import { randomUUID } from "node:crypto";
import { join } from "node:path";

class SchedulerError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

interface Job {
  job_id: string;
  payload: unknown;
  run_at_ms: number;
  status: string;
  attempt: number;
  lease_ms: number;
  max_attempts: number;
  retry_delay_ms: number;
  interval_ms?: number;
  result?: unknown;
  error?: unknown;
  lease_token?: string;
  lease_expires_at_ms?: number;
}

const nowMS = () => Date.now();

class Engine {
  private jobs = new Map<string, Job>();
  private leases = new Map<string, string>();
  private statePath: string;

  constructor(dataDir: string) {
    this.statePath = join(dataDir, "state.json");
    this.load();
  }

  private load(): void {
    try {
      const data = JSON.parse(readFileSync(this.statePath, "utf8")) as { jobs: Job[] };
      for (const j of data.jobs ?? []) this.jobs.set(j.job_id, j);
    } catch {
      /* fresh */
    }
  }

  private persist(): void {
    const jobs = [...this.jobs.values()].map((j) => {
      const { lease_token, lease_expires_at_ms, ...rest } = j;
      return rest;
    });
    const tmp = this.statePath + ".tmp";
    const fd = openSync(tmp, "w");
    writeSync(fd, JSON.stringify({ jobs }));
    fsyncSync(fd);
    closeSync(fd);
    renameSync(tmp, this.statePath);
  }

  private releaseExpired(): void {
    const now = nowMS();
    for (const j of this.jobs.values()) {
      if (j.status === "leased" && (j.lease_expires_at_ms ?? 0) <= now) {
        j.status = "pending";
        delete j.lease_token;
        delete j.lease_expires_at_ms;
      }
    }
  }

  private pollable(j: Job): boolean {
    if (["cancelled", "completed", "failed"].includes(j.status)) return false;
    if (j.run_at_ms > nowMS()) return false;
    if (j.status === "leased") return (j.lease_expires_at_ms ?? 0) <= nowMS();
    return j.status === "pending";
  }

  ping(_params: Record<string, unknown>) {
    return { message: "pong" };
  }

  schedule(params: Record<string, unknown>) {
    const payload = params.payload;
    let runAt: number;
    if (typeof params.delay_ms === "number") runAt = nowMS() + params.delay_ms;
    else if (typeof params.run_at_ms === "number") runAt = params.run_at_ms;
    else throw new SchedulerError("INVALID_PARAMS", "schedule requires delay_ms or run_at_ms");

    const rp = (params.retry_policy ?? {}) as Record<string, number>;
    const jobId = "j-" + randomUUID().replace(/-/g, "").slice(0, 12);
    const job: Job = {
      job_id: jobId,
      payload,
      run_at_ms: runAt,
      status: "pending",
      attempt: 1,
      lease_ms: typeof params.lease_ms === "number" ? params.lease_ms : 3000,
      max_attempts: rp.maximum_attempts ?? 1,
      retry_delay_ms: rp.retry_delay_ms ?? 0,
      interval_ms: typeof params.interval_ms === "number" ? params.interval_ms : undefined,
    };
    this.jobs.set(jobId, job);
    this.persist();
    return { job_id: jobId };
  }

  poll(_params: Record<string, unknown>) {
    this.releaseExpired();
    let best: Job | undefined;
    for (const j of this.jobs.values()) {
      if (this.pollable(j) && (!best || j.run_at_ms < best.run_at_ms)) best = j;
    }
    if (!best) return { job: null };
    const token = randomUUID().replace(/-/g, "");
    best.status = "leased";
    best.lease_token = token;
    best.lease_expires_at_ms = nowMS() + best.lease_ms;
    this.leases.set(token, best.job_id);
    return {
      job: {
        job_id: best.job_id,
        payload: best.payload,
        attempt: best.attempt,
        lease_token: token,
      },
    };
  }

  private jobByToken(token: string): Job {
    const id = this.leases.get(token);
    if (!id) throw new SchedulerError("LEASE_NOT_FOUND", "unknown lease token");
    const j = this.jobs.get(id);
    if (!j || j.lease_token !== token || (j.lease_expires_at_ms ?? 0) <= nowMS()) {
      throw new SchedulerError("LEASE_NOT_FOUND", "lease expired or invalid");
    }
    return j;
  }

  complete(params: Record<string, unknown>) {
    const token = String(params.lease_token);
    const j = this.jobByToken(token);
    j.status = "completed";
    j.result = params.result;
    delete j.lease_token;
    delete j.lease_expires_at_ms;
    this.leases.delete(token);
    if (j.interval_ms) this.spawnNext(j, j.interval_ms);
    this.persist();
    return {};
  }

  private spawnNext(parent: Job, interval: number): void {
    const jobId = "j-" + randomUUID().replace(/-/g, "").slice(0, 12);
    this.jobs.set(jobId, {
      job_id: jobId,
      payload: parent.payload,
      run_at_ms: nowMS() + interval,
      status: "pending",
      attempt: 1,
      lease_ms: parent.lease_ms,
      max_attempts: parent.max_attempts,
      retry_delay_ms: parent.retry_delay_ms,
      interval_ms: parent.interval_ms,
    });
  }

  fail(params: Record<string, unknown>) {
    const token = String(params.lease_token);
    const j = this.jobByToken(token);
    delete j.lease_token;
    delete j.lease_expires_at_ms;
    this.leases.delete(token);
    if (j.attempt < j.max_attempts) {
      j.attempt++;
      j.status = "pending";
      j.run_at_ms = nowMS() + j.retry_delay_ms;
    } else {
      j.status = "failed";
      j.error = params.error;
    }
    this.persist();
    return {};
  }

  cancel(params: Record<string, unknown>) {
    const id = String(params.job_id);
    const j = this.jobs.get(id);
    if (!j) throw new SchedulerError("JOB_NOT_FOUND", `no job ${id}`);
    if (j.status !== "pending") return { cancelled: false };
    j.status = "cancelled";
    this.persist();
    return { cancelled: true };
  }

  get_job(params: Record<string, unknown>) {
    this.releaseExpired();
    const id = String(params.job_id);
    const j = this.jobs.get(id);
    if (!j) throw new SchedulerError("JOB_NOT_FOUND", `no job ${id}`);
    return {
      job_id: j.job_id,
      status: j.status,
      payload: j.payload,
      run_at_ms: j.run_at_ms,
      attempt: j.attempt,
      result: j.result ?? null,
      error: j.error ?? null,
    };
  }
}

const engine = new Engine(process.argv[process.argv.indexOf("--data-dir") + 1]);

function handle(method: string, params: Record<string, unknown>): unknown {
  const fn = (engine as Record<string, (p: Record<string, unknown>) => unknown>)[method];
  if (!fn) throw new SchedulerError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
  return fn.call(engine, params);
}

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
        const result = handle(req.method ?? "", req.params ?? {});
        socket.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as SchedulerError;
        socket.write(JSON.stringify({ id: req.id, error: { code: err.code, message: err.message } }) + "\n");
      }
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
