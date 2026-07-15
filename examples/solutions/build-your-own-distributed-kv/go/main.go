// Reference gateway for "Build your own distributed KV" (Go). Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

const (
	ringID       = "kv"
	raftShard    = "raft-shard"
	lsmShard     = "lsm-shard"
	ringReplicas = 64
)

type gwError struct{ code, message string }

func (e *gwError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type engine struct {
	mu       sync.Mutex
	ring     string
	lsm      string
	raft     [3]string
	ready    bool
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

func (e *engine) ensureStack() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ready {
		return nil
	}
	if e.ring == "" || e.lsm == "" || e.raft[0] == "" {
		return &gwError{"INTERNAL", "missing HASHRING_ADDR, LSM_ADDR, or RAFT*_ADDR"}
	}
	if err := rpc(e.ring, "create_ring", map[string]any{"ring_id": ringID, "replicas": ringReplicas}, nil); err != nil {
		if re, ok := err.(*rpcErr); !ok || re.code != "RING_EXISTS" {
			return err
		}
	}
	for _, n := range []string{raftShard, lsmShard} {
		if err := rpc(e.ring, "add_node", map[string]any{"ring_id": ringID, "node_id": n}, nil); err != nil {
			if re, ok := err.(*rpcErr); !ok || re.code != "NODE_EXISTS" {
				return err
			}
		}
	}
	e.ready = true
	return nil
}

func (e *engine) lookupNode(key string) (string, error) {
	var res struct {
		NodeID string `json:"node_id"`
	}
	if err := rpc(e.ring, "lookup", map[string]any{"ring_id": ringID, "key": key}, &res); err != nil {
		return "", err
	}
	return res.NodeID, nil
}

func (e *engine) raftCall(method string, params map[string]any, out any) error {
	// Raft election can take a beat after compose boot; retry NOT_LEADER.
	deadline := time.Now().Add(5 * time.Second)
	var last error
	for {
		for _, addr := range e.raft {
			if addr == "" {
				continue
			}
			err := rpc(addr, method, params, out)
			if err == nil {
				return nil
			}
			if re, ok := err.(*rpcErr); ok && re.code == "NOT_LEADER" {
				last = err
				continue
			}
			return err
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if last != nil {
		return last
	}
	return &gwError{"NOT_LEADER", "no raft leader available"}
}

func (e *engine) put(key, value string) error {
	node, err := e.lookupNode(key)
	if err != nil {
		return err
	}
	switch node {
	case raftShard:
		return e.raftCall("set", map[string]any{"key": key, "value": value}, nil)
	case lsmShard:
		return rpc(e.lsm, "put", map[string]any{"key": key, "value": value}, nil)
	default:
		return &gwError{"INTERNAL", "unknown shard " + node}
	}
}

func (e *engine) get(key string) (bool, string, error) {
	node, err := e.lookupNode(key)
	if err != nil {
		return false, "", err
	}
	switch node {
	case raftShard:
		var res struct {
			Found bool `json:"found"`
			Value any  `json:"value"`
		}
		if err := e.raftCall("get", map[string]any{"key": key}, &res); err != nil {
			return false, "", err
		}
		if !res.Found {
			return false, "", nil
		}
		if s, ok := res.Value.(string); ok {
			return true, s, nil
		}
		return true, fmt.Sprint(res.Value), nil
	case lsmShard:
		var res struct {
			Found bool   `json:"found"`
			Value string `json:"value"`
		}
		if err := rpc(e.lsm, "get", map[string]any{"key": key}, &res); err != nil {
			return false, "", err
		}
		return res.Found, res.Value, nil
	default:
		return false, "", &gwError{"INTERNAL", "unknown shard " + node}
	}
}

func (e *engine) delete(key string) (bool, error) {
	node, err := e.lookupNode(key)
	if err != nil {
		return false, err
	}
	if node != lsmShard {
		return false, &gwError{"UNSUPPORTED", "delete only supported on LSM shard keys"}
	}
	var res struct {
		Deleted bool `json:"deleted"`
	}
	if err := rpc(e.lsm, "del", map[string]any{"key": key}, &res); err != nil {
		return false, err
	}
	return res.Deleted, nil
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	if method == "ping" {
		if err := e.ensureStack(); err != nil {
			return nil, err
		}
		return map[string]string{"message": "pong"}, nil
	}
	if err := e.ensureStack(); err != nil {
		return nil, err
	}
	switch method {
	case "put":
		var p struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &gwError{"INVALID_PARAMS", "put requires key and value"}
		}
		if err := e.put(p.Key, p.Value); err != nil {
			if re, ok := err.(*rpcErr); ok {
				return nil, &gwError{re.code, re.message}
			}
			return nil, err
		}
		return map[string]any{}, nil
	case "get":
		var p struct {
			Key string `json:"key"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &gwError{"INVALID_PARAMS", "get requires key"}
		}
		found, val, err := e.get(p.Key)
		if err != nil {
			if re, ok := err.(*rpcErr); ok {
				return nil, &gwError{re.code, re.message}
			}
			return nil, err
		}
		if !found {
			return map[string]any{"found": false, "value": nil}, nil
		}
		return map[string]any{"found": true, "value": val}, nil
	case "delete":
		var p struct {
			Key string `json:"key"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &gwError{"INVALID_PARAMS", "delete requires key"}
		}
		deleted, err := e.delete(p.Key)
		if err != nil {
			if re, ok := err.(*rpcErr); ok {
				return nil, &gwError{re.code, re.message}
			}
			if ge, ok := err.(*gwError); ok {
				return nil, ge
			}
			return nil, err
		}
		return map[string]bool{"deleted": deleted}, nil
	default:
		return nil, &gwError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "TCP port")
	flag.String("data-dir", "", "")
	flag.Parse()
	if *port == 0 {
		log.Fatal("usage: your_program.sh --port PORT")
	}
	eng := &engine{
		ring: os.Getenv("HASHRING_ADDR"),
		lsm:  os.Getenv("LSM_ADDR"),
	}
	eng.raft[0] = os.Getenv("RAFT1_ADDR")
	eng.raft[1] = os.Getenv("RAFT2_ADDR")
	eng.raft[2] = os.Getenv("RAFT3_ADDR")

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("distributed KV gateway listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			sc := bufio.NewScanner(c)
			sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				result, err := eng.handle(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					if ge, ok := err.(*gwError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": ge.code, "message": ge.message}}
					} else {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": "INTERNAL", "message": err.Error()}}
					}
				} else {
					resp = map[string]any{"id": req.ID, "result": result}
				}
				out, _ := json.Marshal(resp)
				out = append(out, '\n')
				c.Write(out)
			}
		}(conn)
	}
}
