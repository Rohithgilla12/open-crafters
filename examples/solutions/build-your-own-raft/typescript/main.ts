// Reference solution for the open-crafters "Build your own Raft" challenge.
//
// A minimal but correct 3-node Raft cluster:
//   - newline-delimited JSON over TCP (see PROTOCOL.md)
//   - leader election, log replication, quorum commit
//   - crash-safe persistence (atomic JSON snapshot per state change)
//   - partition safety (NOT_COMMITTED when quorum unreachable)
//
// Passes all 9 stages.

import { connect, createServer, type Socket } from "node:net";
import {
  closeSync,
  existsSync,
  fsyncSync,
  mkdirSync,
  openSync,
  readFileSync,
  renameSync,
  writeSync,
} from "node:fs";
import { join } from "node:path";

class RaftError extends Error {
  constructor(
    public code: string,
    message: string,
    public extra: Record<string, unknown> = {},
  ) {
    super(message);
  }
}

interface LogEntry {
  index: number;
  term: number;
  key: string;
  value: unknown;
}

class Mutex {
  private locked = false;
  private waiters: Array<() => void> = [];

  async runExclusive<T>(fn: () => T | Promise<T>): Promise<T> {
    await new Promise<void>((resolve) => {
      if (!this.locked) {
        this.locked = true;
        resolve();
      } else {
        this.waiters.push(resolve);
      }
    });

    try {
      return await fn();
    } finally {
      const next = this.waiters.shift();
      if (next) next();
      else this.locked = false;
    }
  }
}

function parseAddr(addrStr: string): [string, number] {
  const i = addrStr.lastIndexOf(":");
  return [addrStr.slice(0, i), Number(addrStr.slice(i + 1))];
}

function parsePeers(peersStr: string): Map<string, string> {
  const peers = new Map<string, string>();
  for (const part of peersStr.split(",")) {
    const trimmed = part.trim();
    if (!trimmed) continue;
    const eq = trimmed.indexOf("=");
    peers.set(trimmed.slice(0, eq), trimmed.slice(eq + 1));
  }
  return peers;
}

class RaftNode {
  private currentTerm = 0;
  private votedFor: string | null = null;
  private log: LogEntry[] = [];
  private commitIndex = 0;
  private lastApplied = 0;
  private kv: Record<string, unknown> = {};

  private role: "leader" | "follower" | "candidate" = "follower";
  private leaderId = "0";
  private nextIndex: Record<string, number> = {};
  private matchIndex: Record<string, number> = {};
  private votesReceived = new Set<string>();
  private electionDeadline = 0;
  private lastQuorumContact = 0;

  private readonly mutex = new Mutex();
  private readonly statePath: string;

  constructor(
    readonly nodeId: string,
    readonly peers: Map<string, string>,
    dataDir: string,
  ) {
    this.statePath = join(dataDir, "state.json");
    mkdirSync(dataDir, { recursive: true });
    this.load();
    this.resetElectionTimer();
  }

  private load(): void {
    if (!existsSync(this.statePath)) return;
    const state = JSON.parse(readFileSync(this.statePath, "utf8")) as {
      term?: number;
      voted_for?: string | null;
      log?: LogEntry[];
      commit_index?: number;
      last_applied?: number;
      kv?: Record<string, unknown>;
    };
    this.currentTerm = state.term ?? 0;
    this.votedFor = state.voted_for ?? null;
    this.log = state.log ?? [];
    this.commitIndex = state.commit_index ?? 0;
    this.lastApplied = state.last_applied ?? 0;
    this.kv = state.kv ?? {};
  }

  private persist(): void {
    const state = {
      term: this.currentTerm,
      voted_for: this.votedFor,
      log: this.log,
      commit_index: this.commitIndex,
      last_applied: this.lastApplied,
      kv: this.kv,
    };
    const tmp = this.statePath + ".tmp";
    const fd = openSync(tmp, "w");
    writeSync(fd, JSON.stringify(state));
    fsyncSync(fd);
    closeSync(fd);
    renameSync(tmp, this.statePath);
  }

  private get peerIds(): string[] {
    return [...this.peers.keys()].sort((a, b) => Number(a) - Number(b));
  }

  private get quorum(): number {
    return Math.floor(this.peerIds.length / 2) + 1;
  }

  private lastLogIndex(): number {
    return this.log.length;
  }

  private lastLogTerm(): number {
    return this.log.length ? this.log[this.log.length - 1].term : 0;
  }

  private resetElectionTimer(): void {
    this.electionDeadline = Date.now() + 300 + Math.random() * 200;
  }

  private stepDown(term: number): void {
    this.currentTerm = term;
    this.role = "follower";
    this.votedFor = null;
    this.leaderId = "0";
    this.persist();
    this.resetElectionTimer();
  }

  private becomeLeader(): void {
    this.role = "leader";
    this.leaderId = this.nodeId;
    this.lastQuorumContact = Date.now();
    const lastIdx = this.lastLogIndex();
    for (const pid of this.peerIds) {
      if (pid !== this.nodeId) {
        this.nextIndex[pid] = lastIdx + 1;
        this.matchIndex[pid] = 0;
      }
    }
  }

  private maybeStepDownLeader(): void {
    if (this.role !== "leader") return;
    if (Date.now() - this.lastQuorumContact > 500) {
      this.role = "follower";
      this.leaderId = "0";
      this.resetElectionTimer();
    }
  }

  private applyCommitted(): void {
    while (this.lastApplied < this.commitIndex) {
      this.lastApplied += 1;
      const entry = this.log[this.lastApplied - 1];
      this.kv[entry.key] = entry.value;
    }
    this.persist();
  }

  private updateCommitIndex(): void {
    for (let n = this.lastLogIndex(); n > this.commitIndex; n--) {
      let count = 1;
      for (const pid of this.peerIds) {
        if (pid !== this.nodeId && (this.matchIndex[pid] ?? 0) >= n) count += 1;
      }
      if (count >= this.quorum && this.log[n - 1].term === this.currentTerm) {
        this.commitIndex = n;
        this.applyCommitted();
        break;
      }
    }
  }

  private async rpc(
    peerId: string,
    method: string,
    params: Record<string, unknown>,
    timeout = 500,
  ): Promise<Record<string, unknown> | null> {
    const addr = this.peers.get(peerId);
    if (!addr) return null;
    const [host, port] = parseAddr(addr);
    return new Promise((resolve) => {
      let settled = false;
      const finish = (value: Record<string, unknown> | null) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        resolve(value);
      };
      const sock = connect({ host, port, timeout }, () => {
        const reqId = crypto.randomUUID().replace(/-/g, "");
        sock.write(JSON.stringify({ id: reqId, method, params }) + "\n");
      });
      let buf = "";
      const timer = setTimeout(() => {
        sock.destroy();
        finish(null);
      }, timeout);
      sock.on("data", (chunk) => {
        buf += chunk.toString();
        const nl = buf.indexOf("\n");
        if (nl === -1) return;
        sock.destroy();
        try {
          const resp = JSON.parse(buf.slice(0, nl)) as {
            error?: unknown;
            result?: Record<string, unknown>;
          };
          if (resp.error) finish(null);
          else finish(resp.result ?? null);
        } catch {
          finish(null);
        }
      });
      sock.on("error", () => {
        sock.destroy();
        finish(null);
      });
    });
  }

  private async startElection(): Promise<void> {
    let term: number;
    let lastIdx: number;
    let lastTerm: number;
    await this.mutex.runExclusive(async () => {
      this.role = "candidate";
      this.currentTerm += 1;
      this.votedFor = this.nodeId;
      this.leaderId = "0";
      this.votesReceived = new Set([this.nodeId]);
      this.persist();
      term = this.currentTerm;
      lastIdx = this.lastLogIndex();
      lastTerm = this.lastLogTerm();
      this.resetElectionTimer();
    });

    for (const pid of this.peerIds) {
      if (pid === this.nodeId) continue;
      void this.sendRequestVote(pid, term!, lastIdx!, lastTerm!);
    }
  }

  private async sendRequestVote(
    peerId: string,
    term: number,
    lastIdx: number,
    lastTerm: number,
  ): Promise<void> {
    const result = await this.rpc(peerId, "request_vote", {
      term,
      candidate_id: this.nodeId,
      last_log_index: lastIdx,
      last_log_term: lastTerm,
    });
    if (!result) return;
    await this.mutex.runExclusive(async () => {
      if ((result.term as number) > this.currentTerm) {
        this.stepDown(result.term as number);
        return;
      }
      if (this.role !== "candidate" || this.currentTerm !== term) return;
      if (result.vote_granted) {
        this.votesReceived.add(peerId);
        if (this.votesReceived.size >= this.quorum) this.becomeLeader();
      }
    });
  }

  private async replicateTo(peerId: string): Promise<void> {
    let nextIdx: number;
    let prevLogIndex: number;
    let prevLogTerm: number;
    let entries: LogEntry[];
    let term: number;
    let leaderCommit: number;

    await this.mutex.runExclusive(async () => {
      if (this.role !== "leader") return;
      nextIdx = this.nextIndex[peerId] ?? 1;
      prevLogIndex = nextIdx - 1;
      prevLogTerm = 0;
      if (prevLogIndex > 0) prevLogTerm = this.log[prevLogIndex - 1].term;
      entries = nextIdx <= this.lastLogIndex() ? this.log.slice(nextIdx - 1) : [];
      term = this.currentTerm;
      leaderCommit = this.commitIndex;
    });

    if (term === undefined) return;

    const result = await this.rpc(peerId, "append_entries", {
      term: term!,
      leader_id: this.nodeId,
      prev_log_index: prevLogIndex!,
      prev_log_term: prevLogTerm!,
      entries: entries!,
      leader_commit: leaderCommit!,
    });
    if (!result) return;

    await this.mutex.runExclusive(async () => {
      if ((result.term as number) > this.currentTerm) {
        this.stepDown(result.term as number);
        return;
      }
      if (this.role !== "leader" || this.currentTerm !== term!) return;
      if (result.success) {
        this.lastQuorumContact = Date.now();
        if (entries!.length) {
          this.matchIndex[peerId] = nextIdx! + entries!.length - 1;
          this.nextIndex[peerId] = this.matchIndex[peerId] + 1;
        }
        this.updateCommitIndex();
      } else if ((this.nextIndex[peerId] ?? 1) > 1) {
        this.nextIndex[peerId] = (this.nextIndex[peerId] ?? 1) - 1;
      }
    });
  }

  private leaderHeartbeat(): void {
    for (const pid of this.peerIds) {
      if (pid !== this.nodeId) void this.replicateTo(pid);
    }
  }

  runRaftLoop(): void {
    let lastHeartbeat = 0;
    let ticking = false;
    setInterval(() => {
      if (ticking) return;
      ticking = true;
      const now = Date.now();
      let needElection = false;
      let needHeartbeat = false;
      void this.mutex
        .runExclusive(async () => {
          if (this.role === "leader") {
            this.maybeStepDownLeader();
            if (this.role === "leader" && now - lastHeartbeat >= 100) {
              lastHeartbeat = now;
              needHeartbeat = true;
            }
          } else if (now >= this.electionDeadline) {
            needElection = true;
          }
        })
        .then(() => {
          if (needHeartbeat) this.leaderHeartbeat();
          if (needElection) void this.startElection();
        })
        .finally(() => {
          ticking = false;
        });
    }, 50);
  }

  async ping(_params: Record<string, unknown>): Promise<Record<string, unknown>> {
    return { message: "pong", node_id: this.nodeId };
  }

  async getStatus(_params: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.mutex.runExclusive(async () => ({
      node_id: this.nodeId,
      role: this.role,
      term: this.currentTerm,
      leader_id: this.leaderId,
      commit_index: this.commitIndex,
      last_applied: this.lastApplied,
    }));
  }

  async set(params: Record<string, unknown>): Promise<Record<string, unknown>> {
    let targetIndex: number;
    await this.mutex.runExclusive(async () => {
      if (this.role !== "leader") {
        throw new RaftError("NOT_LEADER", "not the leader", {
          leader_id: this.leaderId || "0",
        });
      }
      const index = this.lastLogIndex() + 1;
      this.log.push({
        index,
        term: this.currentTerm,
        key: params.key as string,
        value: params.value,
      });
      this.persist();
      targetIndex = index;
    });

    this.leaderHeartbeat();

    const deadline = Date.now() + 1500;
    while (Date.now() < deadline) {
      const committed = await this.mutex.runExclusive(async () => {
        if (this.commitIndex >= targetIndex!) return true;
        if (this.role !== "leader") {
          throw new RaftError("NOT_LEADER", "not the leader", {
            leader_id: this.leaderId || "0",
          });
        }
        return false;
      });
      if (committed) return { index: targetIndex! };
      await new Promise((r) => setTimeout(r, 10));
      this.leaderHeartbeat();
    }

    throw new RaftError("NOT_COMMITTED", "could not replicate to a quorum");
  }

  async get(params: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.mutex.runExclusive(async () => {
      this.applyCommitted();
      const key = params.key as string;
      if (key in this.kv) return { found: true, value: this.kv[key] };
      return { found: false };
    });
  }

  async requestVote(params: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.mutex.runExclusive(async () => {
      const term = params.term as number;
      const candidateId = params.candidate_id as string;
      const lastLogIndex = params.last_log_index as number;
      const lastLogTerm = params.last_log_term as number;

      if (term < this.currentTerm) {
        return { term: this.currentTerm, vote_granted: false };
      }
      if (term > this.currentTerm) this.stepDown(term);

      const upToDate =
        lastLogTerm > this.lastLogTerm() ||
        (lastLogTerm === this.lastLogTerm() && lastLogIndex >= this.lastLogIndex());

      let voteGranted = false;
      if (upToDate && (this.votedFor === null || this.votedFor === candidateId)) {
        this.votedFor = candidateId;
        voteGranted = true;
        this.persist();
      }

      this.resetElectionTimer();
      return { term: this.currentTerm, vote_granted: voteGranted };
    });
  }

  async appendEntries(params: Record<string, unknown>): Promise<Record<string, unknown>> {
    return this.mutex.runExclusive(async () => {
      const term = params.term as number;
      const leaderId = params.leader_id as string;

      if (term < this.currentTerm) {
        return { term: this.currentTerm, success: false };
      }
      if (term > this.currentTerm) this.stepDown(term);

      this.role = "follower";
      this.leaderId = leaderId;
      this.resetElectionTimer();

      const prevLogIndex = params.prev_log_index as number;
      const prevLogTerm = params.prev_log_term as number;

      if (prevLogIndex > 0) {
        if (
          prevLogIndex > this.lastLogIndex() ||
          this.log[prevLogIndex - 1].term !== prevLogTerm
        ) {
          return { term: this.currentTerm, success: false };
        }
      }

      const entries = params.entries as LogEntry[];
      if (entries?.length) {
        this.log = this.log.slice(0, prevLogIndex);
        for (const entry of entries) this.log.push(entry);
        this.persist();
      }

      const leaderCommit = params.leader_commit as number;
      if (leaderCommit > this.commitIndex) {
        this.commitIndex = Math.min(leaderCommit, this.lastLogIndex());
        this.applyCommitted();
      }

      return { term: this.currentTerm, success: true };
    });
  }
}

const METHODS = new Set([
  "ping",
  "get_status",
  "set",
  "get",
  "request_vote",
  "append_entries",
]);

function parseArgs(): { nodeId: string; peers: Map<string, string>; port: number; dataDir: string } {
  const argv = process.argv;
  const get = (flag: string) => {
    const i = argv.indexOf(flag);
    if (i === -1 || i + 1 >= argv.length) throw new Error(`missing ${flag}`);
    return argv[i + 1];
  };
  return {
    nodeId: get("--node-id"),
    peers: parsePeers(get("--peers")),
    port: Number(get("--port")),
    dataDir: get("--data-dir"),
  };
}

function main(): void {
  const { nodeId, peers, port, dataDir } = parseArgs();
  const node = new RaftNode(nodeId, peers, dataDir);
  node.runRaftLoop();

  const dispatch: Record<string, (params: Record<string, unknown>) => Promise<unknown>> = {
    ping: (p) => node.ping(p),
    get_status: (p) => node.getStatus(p),
    set: (p) => node.set(p),
    get: (p) => node.get(p),
    request_vote: (p) => node.requestVote(p),
    append_entries: (p) => node.appendEntries(p),
  };

  const handleConn = (socket: Socket): void => {
    let buf = "";
    socket.setEncoding("utf8");
    socket.on("data", (chunk) => {
      buf += chunk;
      let idx: number;
      while ((idx = buf.indexOf("\n")) !== -1) {
        const line = buf.slice(0, idx);
        buf = buf.slice(idx + 1);
        if (!line.trim()) continue;
        void (async () => {
          let requestId: string | undefined;
          try {
            const request = JSON.parse(line) as {
              id?: string;
              method?: string;
              params?: Record<string, unknown>;
            };
            requestId = request.id;
            const method = request.method ?? "";
            if (!METHODS.has(method)) {
              throw new RaftError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
            }
            const handler = dispatch[method];
            const result = await handler(request.params ?? {});
            socket.write(JSON.stringify({ id: requestId, result }) + "\n");
          } catch (e) {
            if (e instanceof RaftError) {
              socket.write(
                JSON.stringify({
                  id: requestId,
                  error: { code: e.code, message: e.message, ...e.extra },
                }) + "\n",
              );
            } else {
              socket.write(
                JSON.stringify({
                  id: requestId,
                  error: { code: "BAD_REQUEST", message: String(e) },
                }) + "\n",
              );
            }
          }
        })();
      }
    });
  };

  createServer(handleConn).listen(port, "127.0.0.1", () => {
    console.log(`raft node ${nodeId} listening on 127.0.0.1:${port}`);
  });
}

main();
