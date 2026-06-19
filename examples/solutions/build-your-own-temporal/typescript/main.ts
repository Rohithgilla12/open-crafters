// Reference solution for "Build your own Temporal" (TypeScript, run with Bun).
//
// A workflow engine: append-only event histories, workflow/activity task
// dispatch over a non-blocking poll protocol, activity retries with
// exponential backoff, durable timers, signals, and crash-safe persistence
// (atomic JSON snapshot per state change). Uses only node: APIs. Passes all
// 10 stages.

import { createServer, type Socket } from "node:net";
import { closeSync, fsyncSync, openSync, readFileSync, renameSync, writeSync } from "node:fs";
import { randomUUID } from "node:crypto";
import { join } from "node:path";

class EngineError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

interface Event {
  event_id: number;
  type: string;
  attributes: Record<string, unknown>;
}

interface PendingActivity {
  activity_type: string;
  input: unknown;
  attempt: number;
  maximum_attempts: number;
  initial_interval_ms: number;
  backoff_coefficient: number;
  available_at: number; // epoch ms
  claimed?: boolean; // runtime only
}

interface Workflow {
  workflow_id: string;
  run_id: string;
  workflow_type: string;
  task_queue: string;
  status: string;
  result: unknown;
  error: unknown;
  history: Event[];
  needs_wft: boolean;
  pending_activities: Record<string, PendingActivity>;
  timers: Record<string, number>; // timer_id -> fire_at epoch ms
  wft_token?: string | null; // runtime only
}

const str = (p: Record<string, unknown>, k: string): string => (p[k] as string) ?? "";
const num = (p: Record<string, unknown>, k: string, def: number): number =>
  typeof p[k] === "number" ? (p[k] as number) : def;

class Engine {
  private order: string[] = [];
  private workflows = new Map<string, Workflow>();
  private wftClaims = new Map<string, string>();
  private actClaims = new Map<string, [string, string]>();
  private statePath: string;

  constructor(dataDir: string) {
    this.statePath = join(dataDir, "state.json");
    this.load();
  }

  private load(): void {
    let data: string;
    try {
      data = readFileSync(this.statePath, "utf8");
    } catch (e) {
      if ((e as NodeJS.ErrnoException).code === "ENOENT") return;
      throw e;
    }
    const snap = JSON.parse(data) as { workflows: Workflow[] };
    for (const wf of snap.workflows) {
      wf.wft_token = null; // claims forgotten on restart so tasks re-deliver
      wf.pending_activities ??= {};
      wf.timers ??= {};
      for (const a of Object.values(wf.pending_activities)) a.claimed = false;
      this.workflows.set(wf.workflow_id, wf);
      this.order.push(wf.workflow_id);
    }
  }

  private persist(): void {
    const workflows = this.order.map((id) => {
      const wf = this.workflows.get(id)!;
      const pending: Record<string, PendingActivity> = {};
      for (const [aid, a] of Object.entries(wf.pending_activities)) {
        pending[aid] = {
          activity_type: a.activity_type, input: a.input, attempt: a.attempt,
          maximum_attempts: a.maximum_attempts, initial_interval_ms: a.initial_interval_ms,
          backoff_coefficient: a.backoff_coefficient, available_at: a.available_at,
        };
      }
      return {
        workflow_id: wf.workflow_id, run_id: wf.run_id, workflow_type: wf.workflow_type,
        task_queue: wf.task_queue, status: wf.status, result: wf.result, error: wf.error,
        history: wf.history,
        // A claimed-but-incomplete workflow task must re-deliver after a crash.
        needs_wft: wf.needs_wft || wf.wft_token != null,
        pending_activities: pending, timers: wf.timers,
      };
    });
    const tmp = this.statePath + ".tmp";
    const fd = openSync(tmp, "w");
    writeSync(fd, JSON.stringify({ workflows }));
    fsyncSync(fd);
    closeSync(fd);
    renameSync(tmp, this.statePath);
  }

  private get(id: string): Workflow {
    const wf = this.workflows.get(id);
    if (!wf) throw new EngineError("WORKFLOW_NOT_FOUND", `no workflow with id ${JSON.stringify(id)}`);
    return wf;
  }

  private appendEvent(wf: Workflow, type: string, attributes: Record<string, unknown>): void {
    wf.history.push({ event_id: wf.history.length + 1, type, attributes });
  }

  handle(method: string, p: Record<string, unknown>): unknown {
    switch (method) {
      case "ping":
        return { message: "pong" };

      case "start_workflow": {
        const id = str(p, "workflow_id");
        if (this.workflows.has(id)) throw new EngineError("WORKFLOW_ALREADY_EXISTS", `workflow ${JSON.stringify(id)} already exists`);
        const wf: Workflow = {
          workflow_id: id, run_id: randomUUID(), workflow_type: str(p, "workflow_type"),
          task_queue: str(p, "task_queue"), status: "RUNNING", result: null, error: null,
          history: [], needs_wft: true, pending_activities: {}, timers: {}, wft_token: null,
        };
        this.appendEvent(wf, "WORKFLOW_EXECUTION_STARTED", { workflow_type: str(p, "workflow_type"), input: p.input ?? null });
        this.workflows.set(id, wf);
        this.order.push(id);
        this.persist();
        return { run_id: wf.run_id };
      }

      case "describe_workflow": {
        const wf = this.get(str(p, "workflow_id"));
        return { workflow_id: wf.workflow_id, run_id: wf.run_id, workflow_type: wf.workflow_type, status: wf.status, result: wf.result, error: wf.error };
      }

      case "get_history":
        return { events: this.get(str(p, "workflow_id")).history };

      case "poll_workflow_task": {
        const queue = str(p, "task_queue");
        for (const id of this.order) {
          const wf = this.workflows.get(id)!;
          if (wf.task_queue === queue && wf.status === "RUNNING" && wf.needs_wft && wf.wft_token == null) {
            const tok = randomUUID();
            wf.needs_wft = false;
            wf.wft_token = tok;
            this.wftClaims.set(tok, id);
            return { task: { task_token: tok, workflow_id: id, run_id: wf.run_id, workflow_type: wf.workflow_type, history: wf.history } };
          }
        }
        return { task: null };
      }

      case "complete_workflow_task": {
        const tok = str(p, "task_token");
        const id = this.wftClaims.get(tok);
        if (id === undefined) throw new EngineError("TASK_NOT_FOUND", `no claimed workflow task with token ${JSON.stringify(tok)}`);
        this.wftClaims.delete(tok);
        const wf = this.workflows.get(id)!;
        wf.wft_token = null;
        const cmds = Array.isArray(p.commands) ? (p.commands as Record<string, unknown>[]) : [];
        for (const cmd of cmds) this.applyCommand(wf, cmd);
        if (wf.status !== "RUNNING") wf.needs_wft = false;
        this.persist();
        return {};
      }

      case "poll_activity_task": {
        const queue = str(p, "task_queue");
        const now = Date.now();
        for (const id of this.order) {
          const wf = this.workflows.get(id)!;
          if (wf.task_queue !== queue || wf.status !== "RUNNING") continue;
          for (const [aid, act] of Object.entries(wf.pending_activities)) {
            if (act.claimed || act.available_at > now) continue;
            const tok = randomUUID();
            act.claimed = true;
            this.actClaims.set(tok, [id, aid]);
            return { task: { task_token: tok, workflow_id: id, run_id: wf.run_id, activity_id: aid, activity_type: act.activity_type, input: act.input, attempt: act.attempt } };
          }
        }
        return { task: null };
      }

      case "complete_activity_task": {
        const [wf, aid] = this.takeActivityClaim(str(p, "task_token"));
        delete wf.pending_activities[aid];
        this.appendEvent(wf, "ACTIVITY_TASK_COMPLETED", { activity_id: aid, result: p.result ?? null });
        wf.needs_wft = true;
        this.persist();
        return {};
      }

      case "fail_activity_task": {
        const [wf, aid] = this.takeActivityClaim(str(p, "task_token"));
        const act = wf.pending_activities[aid]!;
        if (act.attempt < act.maximum_attempts) {
          const delay = act.initial_interval_ms * Math.pow(act.backoff_coefficient, act.attempt - 1);
          act.attempt++;
          act.available_at = Date.now() + delay;
          act.claimed = false;
        } else {
          delete wf.pending_activities[aid];
          this.appendEvent(wf, "ACTIVITY_TASK_FAILED", { activity_id: aid, error: p.error ?? null });
          wf.needs_wft = true;
        }
        this.persist();
        return {};
      }

      case "signal_workflow": {
        const wf = this.get(str(p, "workflow_id"));
        if (wf.status !== "RUNNING") throw new EngineError("WORKFLOW_CLOSED", `workflow ${JSON.stringify(wf.workflow_id)} is ${wf.status}`);
        this.appendEvent(wf, "WORKFLOW_EXECUTION_SIGNALED", { signal_name: str(p, "signal_name"), input: p.input ?? null });
        wf.needs_wft = true;
        this.persist();
        return {};
      }

      default:
        throw new EngineError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
    }
  }

  private applyCommand(wf: Workflow, command: Record<string, unknown>): void {
    const ctype = command.type as string;
    const attrs = (command.attributes as Record<string, unknown>) ?? {};
    switch (ctype) {
      case "SCHEDULE_ACTIVITY": {
        const aid = str(attrs, "activity_id");
        this.appendEvent(wf, "ACTIVITY_TASK_SCHEDULED", { activity_id: aid, activity_type: str(attrs, "activity_type"), input: attrs.input ?? null });
        const policy = (attrs.retry_policy as Record<string, unknown>) ?? {};
        wf.pending_activities[aid] = {
          activity_type: str(attrs, "activity_type"), input: attrs.input ?? null, attempt: 1,
          maximum_attempts: num(policy, "maximum_attempts", 1),
          initial_interval_ms: num(policy, "initial_interval_ms", 1000),
          backoff_coefficient: num(policy, "backoff_coefficient", 2.0),
          available_at: Date.now(), claimed: false,
        };
        break;
      }
      case "START_TIMER":
        this.appendEvent(wf, "TIMER_STARTED", { timer_id: str(attrs, "timer_id"), duration_ms: attrs.duration_ms });
        wf.timers[str(attrs, "timer_id")] = Date.now() + num(attrs, "duration_ms", 0);
        break;
      case "COMPLETE_WORKFLOW":
        this.appendEvent(wf, "WORKFLOW_EXECUTION_COMPLETED", { result: attrs.result ?? null });
        wf.status = "COMPLETED";
        wf.result = attrs.result ?? null;
        break;
      case "FAIL_WORKFLOW":
        this.appendEvent(wf, "WORKFLOW_EXECUTION_FAILED", { error: attrs.error ?? null });
        wf.status = "FAILED";
        wf.error = attrs.error ?? null;
        break;
      default:
        throw new EngineError("UNKNOWN_COMMAND", `unknown command type ${JSON.stringify(ctype)}`);
    }
  }

  private takeActivityClaim(tok: string): [Workflow, string] {
    const claim = this.actClaims.get(tok);
    if (!claim) throw new EngineError("TASK_NOT_FOUND", `no claimed activity task with token ${JSON.stringify(tok)}`);
    this.actClaims.delete(tok);
    return [this.workflows.get(claim[0])!, claim[1]];
  }

  timerLoop(): void {
    setInterval(() => {
      const now = Date.now();
      let fired = false;
      for (const id of this.order) {
        const wf = this.workflows.get(id)!;
        if (wf.status !== "RUNNING") continue;
        for (const [tid, fireAt] of Object.entries(wf.timers)) {
          if (fireAt <= now) {
            delete wf.timers[tid];
            this.appendEvent(wf, "TIMER_FIRED", { timer_id: tid });
            wf.needs_wft = true;
            fired = true;
          }
        }
      }
      if (fired) this.persist();
    }, 50);
  }
}

function handleConnection(socket: Socket, engine: Engine): void {
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
        response = { id: req.id, result: engine.handle(req.method ?? "", req.params ?? {}) };
      } catch (e) {
        const err = e instanceof EngineError ? e : new EngineError("BAD_REQUEST", String(e));
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
const engine = new Engine(dataDir);
engine.timerLoop();
createServer((socket) => handleConnection(socket, engine)).listen(port, "127.0.0.1", () => {
  console.log(`workflow engine listening on 127.0.0.1:${port}`);
});
