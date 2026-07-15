"""Reference payment ledger gateway (Python). Passes all 9 stages."""

import argparse
import json
import os
import socket
import socketserver
import threading

class GWError(Exception):
    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = message


def rpc(addr, method, params):
    host, port_s = addr.rsplit(":", 1)
    conn = socket.create_connection((host, int(port_s)), timeout=10)
    try:
        conn.sendall((json.dumps({"id": "1", "method": method, "params": params}) + "\n").encode())
        buf = b""
        while b"\n" not in buf:
            chunk = conn.recv(4096)
            if not chunk:
                break
            buf += chunk
        resp = json.loads(buf.decode().split("\n", 1)[0])
        if resp.get("error"):
            e = resp["error"]
            raise GWError(e["code"], e["message"])
        return resp.get("result", {})
    finally:
        conn.close()


class Engine:
    def __init__(self):
        self.wal = os.environ.get("WAL_ADDR", "")
        self.idgen = os.environ.get("IDGEN_ADDR", "")
        self.mvcc = os.environ.get("MVCC_ADDR", "")
        self.lock = threading.Lock()
        self.idgen_ready = False

    def ensure_idgen(self):
        if not self.idgen_ready:
            rpc(self.idgen, "configure", {"worker_id": 1})
            self.idgen_ready = True

    def next_id(self):
        self.ensure_idgen()
        return rpc(self.idgen, "next_id", {})["id"]

    def open_account(self, account_id, balance):
        txn = rpc(self.mvcc, "begin", {})["txn"]
        g = rpc(self.mvcc, "get", {"txn": txn, "key": f"bal:{account_id}"})
        if g.get("found"):
            rpc(self.mvcc, "rollback", {"txn": txn})
            raise GWError("ACCOUNT_EXISTS", "account already exists")
        rpc(self.mvcc, "set", {"txn": txn, "key": f"bal:{account_id}", "value": str(balance)})
        rpc(self.mvcc, "commit", {"txn": txn})

    def get_balance(self, account_id):
        txn = rpc(self.mvcc, "begin", {})["txn"]
        try:
            g = rpc(self.mvcc, "get", {"txn": txn, "key": f"bal:{account_id}"})
            if not g.get("found"):
                return 0, False
            return int(g["value"]), True
        finally:
            try:
                rpc(self.mvcc, "rollback", {"txn": txn})
            except GWError:
                pass

    def transfer(self, frm, to, amount, key):
        with self.lock:
            idem = rpc(self.wal, "get", {"key": f"idem:{key}"})
            if idem.get("found"):
                return idem["value"], True
            tid = self.next_id()
            txn = rpc(self.mvcc, "begin", {})["txn"]

            def read(acct):
                g = rpc(self.mvcc, "get", {"txn": txn, "key": f"bal:{acct}"})
                if not g.get("found"):
                    raise GWError("ACCOUNT_NOT_FOUND", f"account {acct}")
                return int(g["value"])

            try:
                from_bal = read(frm)
                to_bal = read(to)
                if from_bal < amount:
                    raise GWError("INSUFFICIENT_FUNDS", "not enough balance")
                rpc(self.mvcc, "set", {"txn": txn, "key": f"bal:{frm}", "value": str(from_bal - amount)})
                rpc(self.mvcc, "set", {"txn": txn, "key": f"bal:{to}", "value": str(to_bal + amount)})
                rpc(self.mvcc, "commit", {"txn": txn})
            except GWError as e:
                try:
                    rpc(self.mvcc, "rollback", {"txn": txn})
                except GWError:
                    pass
                if e.code == "CONFLICT":
                    raise GWError("CONFLICT", e.message) from e
                raise
            env = {
                "transfer_id": tid,
                "from_account": frm,
                "to_account": to,
                "amount": amount,
                "idempotency_key": key,
            }
            rpc(self.wal, "set", {"key": f"xfer:{tid}", "value": json.dumps(env)})
            rpc(self.wal, "set", {"key": f"idem:{key}", "value": tid})
            return tid, False

    def handle(self, method, params):
        if method == "ping":
            return {"message": "pong"}
        if method == "open_account":
            account_id = params.get("account_id")
            balance = params.get("balance")
            if not account_id or balance is None or balance < 0:
                raise GWError("INVALID_PARAMS", "open_account requires account_id and balance")
            with self.lock:
                self.open_account(account_id, balance)
            return {}
        if method == "get_balance":
            account_id = params.get("account_id")
            if not account_id:
                raise GWError("INVALID_PARAMS", "get_balance requires account_id")
            bal, found = self.get_balance(account_id)
            return {"balance": bal, "found": found}
        if method == "transfer":
            frm = params.get("from_account")
            to = params.get("to_account")
            amount = params.get("amount")
            key = params.get("idempotency_key")
            if not frm or not to or not amount or amount <= 0 or not key or frm == to:
                raise GWError("INVALID_PARAMS", "invalid transfer params")
            tid, replayed = self.transfer(frm, to, amount, key)
            return {"transfer_id": tid, "replayed": replayed}
        if method == "get_transfer":
            tid = params.get("transfer_id")
            if not tid:
                raise GWError("INVALID_PARAMS", "get_transfer requires transfer_id")
            g = rpc(self.wal, "get", {"key": f"xfer:{tid}"})
            if not g.get("found"):
                return {"found": False, "transfer": None}
            return {"found": True, "transfer": json.loads(g["value"])}
        raise GWError("UNKNOWN_METHOD", f"unknown method {method!r}")


ENGINE = Engine()


class Handler(socketserver.StreamRequestHandler):
    def handle(self):
        for line in self.rfile:
            if not line.strip():
                continue
            req = json.loads(line)
            try:
                result = ENGINE.handle(req.get("method"), req.get("params") or {})
                resp = {"id": req.get("id"), "result": result}
            except GWError as e:
                resp = {"id": req.get("id"), "error": {"code": e.code, "message": e.message}}
            self.wfile.write(json.dumps(resp).encode() + b"\n")
            self.wfile.flush()


class Server(socketserver.ThreadingTCPServer):
    allow_reuse_address = True
    daemon_threads = True


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--port", type=int, required=True)
    p.add_argument("--data-dir", default="")
    args = p.parse_args()
    with Server(("127.0.0.1", args.port), Handler) as srv:
        print(f"listening on 127.0.0.1:{args.port}", flush=True)
        srv.serve_forever()


if __name__ == "__main__":
    main()
