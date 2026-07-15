// Reference gateway for "Build your own payment ledger" (Go). Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
)

type gwError struct{ code, message string }

func (e *gwError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type peer struct{ addr string }

type engine struct {
	mu          sync.Mutex
	wal         peer
	idgen       peer
	mvcc        peer
	idgenReady  bool
}

type rpcErr struct{ code, message string }

func (e *rpcErr) Error() string { return e.code + ": " + e.message }

func rpc(addr, method string, params map[string]any, out any) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	req, _ := json.Marshal(map[string]any{"id": "1", "method": method, "params": params})
	req = append(req, '\n')
	if _, err := conn.Write(req); err != nil {
		return err
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	if !sc.Scan() {
		return fmt.Errorf("no response from %s", addr)
	}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return &rpcErr{resp.Error.Code, resp.Error.Message}
	}
	if out != nil && len(resp.Result) > 0 {
		return json.Unmarshal(resp.Result, out)
	}
	return nil
}

func (e *engine) ensureIDGen() error {
	if e.idgenReady {
		return nil
	}
	if err := rpc(e.idgen.addr, "configure", map[string]any{"worker_id": 1}, nil); err != nil {
		return err
	}
	e.idgenReady = true
	return nil
}

func (e *engine) nextTransferID() (string, error) {
	if err := e.ensureIDGen(); err != nil {
		return "", err
	}
	var res struct {
		ID string `json:"id"`
	}
	if err := rpc(e.idgen.addr, "next_id", map[string]any{}, &res); err != nil {
		return "", err
	}
	return res.ID, nil
}

func balKey(id string) string { return "bal:" + id }
func xferKey(id string) string { return "xfer:" + id }
func idemKey(k string) string  { return "idem:" + k }

func (e *engine) openAccount(id string, balance int) error {
	var begin struct {
		Txn string `json:"txn"`
	}
	if err := rpc(e.mvcc.addr, "begin", map[string]any{}, &begin); err != nil {
		return err
	}
	var getRes struct {
		Found bool `json:"found"`
	}
	if err := rpc(e.mvcc.addr, "get", map[string]any{"txn": begin.Txn, "key": balKey(id)}, &getRes); err != nil {
		return err
	}
	if getRes.Found {
		_ = rpc(e.mvcc.addr, "rollback", map[string]any{"txn": begin.Txn}, nil)
		return &gwError{"ACCOUNT_EXISTS", "account already exists"}
	}
	if err := rpc(e.mvcc.addr, "set", map[string]any{
		"txn": begin.Txn, "key": balKey(id), "value": strconv.Itoa(balance),
	}, nil); err != nil {
		return err
	}
	var commit struct {
		Committed bool `json:"committed"`
	}
	if err := rpc(e.mvcc.addr, "commit", map[string]any{"txn": begin.Txn}, &commit); err != nil {
		return err
	}
	return nil
}

func (e *engine) getBalance(id string) (int, bool, error) {
	var begin struct {
		Txn string `json:"txn"`
	}
	if err := rpc(e.mvcc.addr, "begin", map[string]any{}, &begin); err != nil {
		return 0, false, err
	}
	defer rpc(e.mvcc.addr, "rollback", map[string]any{"txn": begin.Txn}, nil)
	var getRes struct {
		Value string `json:"value"`
		Found bool   `json:"found"`
	}
	if err := rpc(e.mvcc.addr, "get", map[string]any{"txn": begin.Txn, "key": balKey(id)}, &getRes); err != nil {
		return 0, false, err
	}
	if !getRes.Found {
		return 0, false, nil
	}
	n, err := strconv.Atoi(getRes.Value)
	if err != nil {
		return 0, false, err
	}
	return n, true, nil
}

func (e *engine) transfer(from, to string, amount int, key string) (string, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	var idemGet struct {
		Value string `json:"value"`
		Found bool   `json:"found"`
	}
	if err := rpc(e.wal.addr, "get", map[string]any{"key": idemKey(key)}, &idemGet); err != nil {
		return "", false, err
	}
	if idemGet.Found {
		return idemGet.Value, true, nil
	}

	tid, err := e.nextTransferID()
	if err != nil {
		return "", false, err
	}

	var begin struct {
		Txn string `json:"txn"`
	}
	if err := rpc(e.mvcc.addr, "begin", map[string]any{}, &begin); err != nil {
		return "", false, err
	}
	readBal := func(acct string) (int, error) {
		var g struct {
			Value string `json:"value"`
			Found bool   `json:"found"`
		}
		if err := rpc(e.mvcc.addr, "get", map[string]any{"txn": begin.Txn, "key": balKey(acct)}, &g); err != nil {
			return 0, err
		}
		if !g.Found {
			return 0, &gwError{"ACCOUNT_NOT_FOUND", "account " + acct}
		}
		return strconv.Atoi(g.Value)
	}
	fromBal, err := readBal(from)
	if err != nil {
		_ = rpc(e.mvcc.addr, "rollback", map[string]any{"txn": begin.Txn}, nil)
		return "", false, err
	}
	toBal, err := readBal(to)
	if err != nil {
		_ = rpc(e.mvcc.addr, "rollback", map[string]any{"txn": begin.Txn}, nil)
		return "", false, err
	}
	if fromBal < amount {
		_ = rpc(e.mvcc.addr, "rollback", map[string]any{"txn": begin.Txn}, nil)
		return "", false, &gwError{"INSUFFICIENT_FUNDS", "not enough balance"}
	}
	if err := rpc(e.mvcc.addr, "set", map[string]any{
		"txn": begin.Txn, "key": balKey(from), "value": strconv.Itoa(fromBal - amount),
	}, nil); err != nil {
		return "", false, err
	}
	if err := rpc(e.mvcc.addr, "set", map[string]any{
		"txn": begin.Txn, "key": balKey(to), "value": strconv.Itoa(toBal + amount),
	}, nil); err != nil {
		return "", false, err
	}
	if err := rpc(e.mvcc.addr, "commit", map[string]any{"txn": begin.Txn}, nil); err != nil {
		if re, ok := err.(*rpcErr); ok && re.code == "CONFLICT" {
			return "", false, &gwError{"CONFLICT", re.message}
		}
		return "", false, err
	}

	env := map[string]any{
		"transfer_id": tid, "from_account": from, "to_account": to,
		"amount": amount, "idempotency_key": key,
	}
	body, _ := json.Marshal(env)
	if err := rpc(e.wal.addr, "set", map[string]any{"key": xferKey(tid), "value": string(body)}, nil); err != nil {
		return "", false, err
	}
	if err := rpc(e.wal.addr, "set", map[string]any{"key": idemKey(key), "value": tid}, nil); err != nil {
		return "", false, err
	}
	return tid, false, nil
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "open_account":
		var p struct {
			AccountID string `json:"account_id"`
			Balance   int    `json:"balance"`
		}
		if json.Unmarshal(raw, &p) != nil || p.AccountID == "" || p.Balance < 0 {
			return nil, &gwError{"INVALID_PARAMS", "open_account requires account_id and balance"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		if err := e.openAccount(p.AccountID, p.Balance); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	case "get_balance":
		var p struct {
			AccountID string `json:"account_id"`
		}
		if json.Unmarshal(raw, &p) != nil || p.AccountID == "" {
			return nil, &gwError{"INVALID_PARAMS", "get_balance requires account_id"}
		}
		bal, found, err := e.getBalance(p.AccountID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"balance": bal, "found": found}, nil
	case "transfer":
		var p struct {
			From           string `json:"from_account"`
			To             string `json:"to_account"`
			Amount         int    `json:"amount"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if json.Unmarshal(raw, &p) != nil || p.From == "" || p.To == "" || p.Amount <= 0 || p.IdempotencyKey == "" || p.From == p.To {
			return nil, &gwError{"INVALID_PARAMS", "invalid transfer params"}
		}
		id, replayed, err := e.transfer(p.From, p.To, p.Amount, p.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		return map[string]any{"transfer_id": id, "replayed": replayed}, nil
	case "get_transfer":
		var p struct {
			TransferID string `json:"transfer_id"`
		}
		if json.Unmarshal(raw, &p) != nil || p.TransferID == "" {
			return nil, &gwError{"INVALID_PARAMS", "get_transfer requires transfer_id"}
		}
		var g struct {
			Value string `json:"value"`
			Found bool   `json:"found"`
		}
		if err := rpc(e.wal.addr, "get", map[string]any{"key": xferKey(p.TransferID)}, &g); err != nil {
			return nil, err
		}
		if !g.Found {
			return map[string]any{"found": false, "transfer": nil}, nil
		}
		var env map[string]any
		if json.Unmarshal([]byte(g.Value), &env) != nil {
			return nil, &gwError{"INTERNAL", "corrupt transfer record"}
		}
		return map[string]any{"found": true, "transfer": env}, nil
	default:
		return nil, &gwError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	eng := &engine{
		wal:   peer{addr: os.Getenv("WAL_ADDR")},
		idgen: peer{addr: os.Getenv("IDGEN_ADDR")},
		mvcc:  peer{addr: os.Getenv("MVCC_ADDR")},
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("listening on 127.0.0.1:%d\n", *port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			sc := bufio.NewScanner(c)
			sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			w := bufio.NewWriter(c)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				res, err := eng.handle(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					if ge, ok := err.(*gwError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": ge.code, "message": ge.message}}
					} else {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": "INTERNAL", "message": err.Error()}}
					}
				} else {
					resp = map[string]any{"id": req.ID, "result": res}
				}
				b, _ := json.Marshal(resp)
				w.Write(b)
				w.WriteByte('\n')
				w.Flush()
			}
		}(conn)
	}
}
