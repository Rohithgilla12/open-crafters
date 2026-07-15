// Reference payment ledger gateway (TypeScript). Passes all 9 stages.
import { createConnection, createServer, type Socket } from "node:net";

class GWError extends Error {
  constructor(public code: string, message: string) {
    super(message);
  }
}

function rpc(addr: string, method: string, params: Record<string, unknown>): Promise<Record<string, unknown>> {
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
      const resp = JSON.parse(buf.slice(0, idx)) as {
        result?: Record<string, unknown>;
        error?: { code: string; message: string };
      };
      if (resp.error) reject(new GWError(resp.error.code, resp.error.message));
      else resolve(resp.result ?? {});
    });
    conn.on("error", reject);
    conn.write(JSON.stringify({ id: "1", method, params }) + "\n");
  });
}

class Engine {
  wal = process.env.WAL_ADDR ?? "";
  idgen = process.env.IDGEN_ADDR ?? "";
  mvcc = process.env.MVCC_ADDR ?? "";
  idgenReady = false;
  chain: Promise<unknown> = Promise.resolve();

  serialize<T>(fn: () => Promise<T>): Promise<T> {
    const run = this.chain.then(fn, fn);
    this.chain = run.then(
      () => undefined,
      () => undefined,
    );
    return run;
  }

  async ensureIdgen() {
    if (!this.idgenReady) {
      await rpc(this.idgen, "configure", { worker_id: 1 });
      this.idgenReady = true;
    }
  }

  async nextId() {
    await this.ensureIdgen();
    const res = await rpc(this.idgen, "next_id", {});
    return String(res.id);
  }

  async openAccount(accountId: string, balance: number) {
    const begin = await rpc(this.mvcc, "begin", {});
    const txn = String(begin.txn);
    const g = await rpc(this.mvcc, "get", { txn, key: `bal:${accountId}` });
    if (g.found) {
      await rpc(this.mvcc, "rollback", { txn });
      throw new GWError("ACCOUNT_EXISTS", "account already exists");
    }
    await rpc(this.mvcc, "set", { txn, key: `bal:${accountId}`, value: String(balance) });
    await rpc(this.mvcc, "commit", { txn });
  }

  async getBalance(accountId: string): Promise<{ balance: number; found: boolean }> {
    const begin = await rpc(this.mvcc, "begin", {});
    const txn = String(begin.txn);
    try {
      const g = await rpc(this.mvcc, "get", { txn, key: `bal:${accountId}` });
      if (!g.found) return { balance: 0, found: false };
      return { balance: Number(g.value), found: true };
    } finally {
      await rpc(this.mvcc, "rollback", { txn }).catch(() => {});
    }
  }

  async transfer(from: string, to: string, amount: number, key: string) {
    return this.serialize(async () => {
      const idem = await rpc(this.wal, "get", { key: `idem:${key}` });
      if (idem.found) return { transfer_id: String(idem.value), replayed: true };
      const tid = await this.nextId();
      const begin = await rpc(this.mvcc, "begin", {});
      const txn = String(begin.txn);
      const read = async (acct: string) => {
        const g = await rpc(this.mvcc, "get", { txn, key: `bal:${acct}` });
        if (!g.found) throw new GWError("ACCOUNT_NOT_FOUND", `account ${acct}`);
        return Number(g.value);
      };
      try {
        const fromBal = await read(from);
        const toBal = await read(to);
        if (fromBal < amount) throw new GWError("INSUFFICIENT_FUNDS", "not enough balance");
        await rpc(this.mvcc, "set", { txn, key: `bal:${from}`, value: String(fromBal - amount) });
        await rpc(this.mvcc, "set", { txn, key: `bal:${to}`, value: String(toBal + amount) });
        await rpc(this.mvcc, "commit", { txn });
      } catch (e) {
        await rpc(this.mvcc, "rollback", { txn }).catch(() => {});
        throw e;
      }
      const env = {
        transfer_id: tid,
        from_account: from,
        to_account: to,
        amount,
        idempotency_key: key,
      };
      await rpc(this.wal, "set", { key: `xfer:${tid}`, value: JSON.stringify(env) });
      await rpc(this.wal, "set", { key: `idem:${key}`, value: tid });
      return { transfer_id: tid, replayed: false };
    });
  }

  async handle(method: string, params: Record<string, unknown>) {
    if (method === "ping") return { message: "pong" };
    if (method === "open_account") {
      const accountId = String(params.account_id ?? "");
      const balance = Number(params.balance);
      if (!accountId || Number.isNaN(balance) || balance < 0) {
        throw new GWError("INVALID_PARAMS", "open_account requires account_id and balance");
      }
      await this.serialize(() => this.openAccount(accountId, balance));
      return {};
    }
    if (method === "get_balance") {
      const accountId = String(params.account_id ?? "");
      if (!accountId) throw new GWError("INVALID_PARAMS", "get_balance requires account_id");
      return this.getBalance(accountId);
    }
    if (method === "transfer") {
      const from = String(params.from_account ?? "");
      const to = String(params.to_account ?? "");
      const amount = Number(params.amount);
      const key = String(params.idempotency_key ?? "");
      if (!from || !to || !amount || amount <= 0 || !key || from === to) {
        throw new GWError("INVALID_PARAMS", "invalid transfer params");
      }
      return this.transfer(from, to, amount, key);
    }
    if (method === "get_transfer") {
      const tid = String(params.transfer_id ?? "");
      if (!tid) throw new GWError("INVALID_PARAMS", "get_transfer requires transfer_id");
      const g = await rpc(this.wal, "get", { key: `xfer:${tid}` });
      if (!g.found) return { found: false, transfer: null };
      return { found: true, transfer: JSON.parse(String(g.value)) };
    }
    throw new GWError("UNKNOWN_METHOD", `unknown method ${method}`);
  }
}

const engine = new Engine();

function handleConn(socket: Socket) {
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
      engine
        .handle(req.method ?? "", req.params ?? {})
        .then((result) => socket.write(JSON.stringify({ id: req.id, result }) + "\n"))
        .catch((e: GWError) =>
          socket.write(
            JSON.stringify({ id: req.id, error: { code: e.code ?? "INTERNAL", message: e.message } }) + "\n",
          ),
        );
    }
  });
}

const port = Number(process.argv[process.argv.indexOf("--port") + 1]);
createServer(handleConn).listen(port, "127.0.0.1", () => {
  console.log(`listening on 127.0.0.1:${port}`);
});
