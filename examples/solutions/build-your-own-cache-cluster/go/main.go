// Reference gateway for "Build your own cache cluster" (Go). Passes all 9 stages.
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
)

const (
	ringID      = "cache"
	node1       = "node1"
	node2       = "node2"
	filterID    = "keys"
	ringReplicas = 64
)

type gwError struct{ code, message string }

func (e *gwError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type peers struct {
	ring    string
	bloom   string
	limiter string
	cache1  string
	cache2  string
}

type engine struct {
	mu    sync.Mutex
	peers peers
	ready bool
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
	p := e.peers
	if p.ring == "" || p.bloom == "" || p.limiter == "" || p.cache1 == "" || p.cache2 == "" {
		return &gwError{"INTERNAL", "missing HASHRING_ADDR, BLOOM_ADDR, LIMITER_ADDR, or CACHE_NODE*_ADDR"}
	}
	if err := rpc(p.ring, "create_ring", map[string]any{"ring_id": ringID, "replicas": ringReplicas}, nil); err != nil {
		if re, ok := err.(*rpcErr); !ok || re.code != "RING_EXISTS" {
			return err
		}
	}
	for _, n := range []string{node1, node2} {
		if err := rpc(p.ring, "add_node", map[string]any{"ring_id": ringID, "node_id": n}, nil); err != nil {
			if re, ok := err.(*rpcErr); !ok || re.code != "NODE_EXISTS" {
				return err
			}
		}
	}
	if err := rpc(p.bloom, "create", map[string]any{"filter_id": filterID, "m": 8192, "k": 4}, nil); err != nil {
		if re, ok := err.(*rpcErr); !ok || re.code != "FILTER_EXISTS" {
			return err
		}
	}
	for _, addr := range []string{p.cache1, p.cache2} {
		if err := rpc(addr, "configure", map[string]any{"max_keys": 4096}, nil); err != nil {
			return err
		}
	}
	e.ready = true
	return nil
}

func (e *engine) cacheAddr(nodeID string) (string, error) {
	switch nodeID {
	case node1:
		return e.peers.cache1, nil
	case node2:
		return e.peers.cache2, nil
	default:
		return "", &gwError{"INTERNAL", "unknown node " + nodeID}
	}
}

func (e *engine) lookupNode(key string) (string, error) {
	var res struct {
		NodeID string `json:"node_id"`
	}
	if err := rpc(e.peers.ring, "lookup", map[string]any{"ring_id": ringID, "key": key}, &res); err != nil {
		return "", err
	}
	return res.NodeID, nil
}

func (e *engine) admit(key string) error {
	rlKey := "rl:" + key
	var takeRes struct {
		Allowed bool `json:"allowed"`
	}
	err := rpc(e.peers.limiter, "take", map[string]any{"key": rlKey, "cost": 1}, &takeRes)
	if err != nil {
		if re, ok := err.(*rpcErr); ok && re.code == "KEY_NOT_FOUND" {
			if err := rpc(e.peers.limiter, "configure", map[string]any{
				"key": rlKey, "algorithm": "token_bucket",
				"capacity": 100, "refill_tokens": 100, "refill_interval_ms": 1000,
			}, nil); err != nil {
				return err
			}
			err = rpc(e.peers.limiter, "take", map[string]any{"key": rlKey, "cost": 1}, &takeRes)
		}
		if err != nil {
			return err
		}
	}
	if !takeRes.Allowed {
		return &gwError{"RATE_LIMITED", "rate limit exceeded for key"}
	}
	return nil
}

func (e *engine) bloomMaybe(key string) (bool, error) {
	var res struct {
		MaybePresent bool `json:"maybe_present"`
	}
	if err := rpc(e.peers.bloom, "contains", map[string]any{"filter_id": filterID, "item": key}, &res); err != nil {
		return false, err
	}
	return res.MaybePresent, nil
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	if err := e.ensureStack(); err != nil {
		return nil, err
	}
	switch method {
	case "set":
		var p struct {
			Key    string `json:"key"`
			Value  string `json:"value"`
			TTLMS  int    `json:"ttl_ms"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" || p.Value == "" {
			return nil, &gwError{"INVALID_PARAMS", "set requires key and value"}
		}
		if err := e.admit(p.Key); err != nil {
			return nil, err
		}
		node, err := e.lookupNode(p.Key)
		if err != nil {
			return nil, err
		}
		addr, err := e.cacheAddr(node)
		if err != nil {
			return nil, err
		}
		params := map[string]any{"key": p.Key, "value": p.Value}
		if p.TTLMS > 0 {
			params["ttl_ms"] = p.TTLMS
		}
		var setRes struct {
			Version int `json:"version"`
		}
		if err := rpc(addr, "set", params, &setRes); err != nil {
			return nil, err
		}
		if err := rpc(e.peers.bloom, "add", map[string]any{"filter_id": filterID, "item": p.Key}, nil); err != nil {
			return nil, err
		}
		return map[string]int{"version": setRes.Version}, nil
	case "get":
		var p struct {
			Key string `json:"key"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &gwError{"INVALID_PARAMS", "get requires key"}
		}
		maybe, err := e.bloomMaybe(p.Key)
		if err != nil {
			return nil, err
		}
		if !maybe {
			return map[string]any{"hit": false}, nil
		}
		if err := e.admit(p.Key); err != nil {
			return nil, err
		}
		node, err := e.lookupNode(p.Key)
		if err != nil {
			return nil, err
		}
		addr, err := e.cacheAddr(node)
		if err != nil {
			return nil, err
		}
		var getRes struct {
			Hit     bool   `json:"hit"`
			Value   string `json:"value"`
			Version int    `json:"version"`
		}
		if err := rpc(addr, "get", map[string]any{"key": p.Key}, &getRes); err != nil {
			return nil, err
		}
		if !getRes.Hit {
			return map[string]any{"hit": false}, nil
		}
		return map[string]any{"hit": true, "value": getRes.Value, "version": getRes.Version}, nil
	case "delete":
		var p struct {
			Key string `json:"key"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &gwError{"INVALID_PARAMS", "delete requires key"}
		}
		if err := e.admit(p.Key); err != nil {
			return nil, err
		}
		node, err := e.lookupNode(p.Key)
		if err != nil {
			return nil, err
		}
		addr, err := e.cacheAddr(node)
		if err != nil {
			return nil, err
		}
		var delRes struct {
			Deleted bool `json:"deleted"`
		}
		if err := rpc(addr, "delete", map[string]any{"key": p.Key}, &delRes); err != nil {
			return nil, err
		}
		return map[string]bool{"deleted": delRes.Deleted}, nil
	case "mget":
		var p struct {
			Keys []string `json:"keys"`
		}
		if json.Unmarshal(raw, &p) != nil || len(p.Keys) == 0 {
			return nil, &gwError{"INVALID_PARAMS", "mget requires keys"}
		}
		entries := make([]any, len(p.Keys))
		for i, key := range p.Keys {
			b, _ := json.Marshal(map[string]string{"key": key})
			res, err := e.handle("get", b)
			if err != nil {
				if ge, ok := err.(*gwError); ok && ge.code == "RATE_LIMITED" {
					return nil, err
				}
				return nil, err
			}
			m := res.(map[string]any)
			entry := map[string]any{"key": key}
			if hit, _ := m["hit"].(bool); hit {
				entry["hit"] = true
				entry["value"] = m["value"]
				entry["version"] = m["version"]
			} else {
				entry["hit"] = false
			}
			entries[i] = entry
		}
		return map[string]any{"entries": entries}, nil
	default:
		return nil, &gwError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	eng := &engine{
		peers: peers{
			ring:    os.Getenv("HASHRING_ADDR"),
			bloom:   os.Getenv("BLOOM_ADDR"),
			limiter: os.Getenv("LIMITER_ADDR"),
			cache1:  os.Getenv("CACHE_NODE1_ADDR"),
			cache2:  os.Getenv("CACHE_NODE2_ADDR"),
		},
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
