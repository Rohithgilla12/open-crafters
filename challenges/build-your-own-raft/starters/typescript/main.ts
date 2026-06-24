// Starter template for "Build your own Raft" (TypeScript, run with Bun).
//
// Boots a TCP server speaking newline-delimited JSON and answers `ping` with the
// node id — enough to pass the first stage. Extend handleRequest stage by stage.

import { createServer, type Socket } from "node:net";

class RaftError extends Error {
  constructor(
    public code: string,
    message: string,
    public extra: Record<string, unknown> = {},
  ) {
    super(message);
  }
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

function handleRequest(method: string, _params: Record<string, unknown>, nodeId: string): unknown {
  if (method === "ping") return { message: "pong", node_id: nodeId };

  // TODO (stage 2): leader election — get_status, request_vote, append_entries heartbeats
  // TODO (stage 3): set on leader, replicate log entries to a quorum
  // TODO (stage 4): get — serve reads from committed/applied state
  // TODO (stage 5): tolerate a follower crash (majority still commits)
  // TODO (stage 6): re-elect after leader crash, preserve committed log
  // TODO (stage 7): persist term, voted_for, log, commit_index, KV to --data-dir
  // TODO (stage 8): partition safety — NOT_COMMITTED when quorum unreachable
  // TODO (stage 9): gauntlet — writes, crash, restart
  throw new RaftError("UNKNOWN_METHOD", `unknown method ${JSON.stringify(method)}`);
}

function handleConn(socket: Socket, nodeId: string): void {
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
        const result = handleRequest(req.method ?? "", req.params ?? {}, nodeId);
        socket.write(JSON.stringify({ id: req.id, result }) + "\n");
      } catch (e) {
        const err = e as RaftError;
        socket.write(
          JSON.stringify({
            id: req.id,
            error: { code: err.code, message: err.message, ...err.extra },
          }) + "\n",
        );
      }
    }
  });
}

const { nodeId, peers, port, dataDir } = parseArgs();
void peers;
void dataDir;

createServer((socket) => handleConn(socket, nodeId)).listen(port, "127.0.0.1", () => {
  console.log(`raft node ${nodeId} listening on 127.0.0.1:${port}`);
});
