// Reference gateway for "Build your own URL shortener" (Go). Passes all 9 stages.
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

type gwError struct{ code, message string }

func (e *gwError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type peer struct {
	addr string
}

type engine struct {
	mu    sync.Mutex
	idgen peer
	bloom peer
	store peer
	ready bool
}

func envAddr(key string) string {
	return os.Getenv(key)
}

func (e *engine) ensureBloom() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.ready {
		return nil
	}
	if e.idgen.addr == "" || e.bloom.addr == "" || e.store.addr == "" {
		return &gwError{"INTERNAL", "missing IDGEN_ADDR, BLOOM_ADDR, or STORE_ADDR"}
	}
	if err := rpc(e.bloom.addr, "create", map[string]any{"filter_id": "codes", "m": 8192, "k": 4}, nil); err != nil {
		if err2, ok := err.(*rpcErr); !ok || err2.code != "FILTER_EXISTS" {
			return err
		}
	}
	e.ready = true
	return nil
}

type rpcErr struct{ code, message string }

func (e *rpcErr) Error() string { return e.code + ": " + e.message }

func rpc(addr, method string, params map[string]any, out any) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	req, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
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
		ID     string          `json:"id"`
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

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "shorten":
		if err := e.ensureBloom(); err != nil {
			return nil, err
		}
		var p struct {
			URL string `json:"url"`
		}
		if json.Unmarshal(raw, &p) != nil || p.URL == "" {
			return nil, &gwError{"INVALID_PARAMS", "shorten requires url"}
		}
		var idRes struct {
			ID string `json:"id"`
		}
		if err := rpc(e.idgen.addr, "next_id", map[string]any{}, &idRes); err != nil {
			return nil, err
		}
		code := idRes.ID
		if err := rpc(e.bloom.addr, "add", map[string]any{"filter_id": "codes", "item": code}, nil); err != nil {
			return nil, err
		}
		if err := rpc(e.store.addr, "put", map[string]any{"key": "links/" + code, "body": p.URL}, nil); err != nil {
			return nil, err
		}
		return map[string]string{"code": code}, nil
	case "resolve":
		if err := e.ensureBloom(); err != nil {
			return nil, err
		}
		var p struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Code == "" {
			return nil, &gwError{"INVALID_PARAMS", "resolve requires code"}
		}
		var bloomRes struct {
			MaybePresent bool `json:"maybe_present"`
		}
		if err := rpc(e.bloom.addr, "contains", map[string]any{"filter_id": "codes", "item": p.Code}, &bloomRes); err != nil {
			return nil, err
		}
		if !bloomRes.MaybePresent {
			return map[string]any{"found": false}, nil
		}
		var getRes struct {
			Found bool   `json:"found"`
			Body  string `json:"body"`
		}
		if err := rpc(e.store.addr, "get", map[string]any{"key": "links/" + p.Code}, &getRes); err != nil {
			if re, ok := err.(*rpcErr); ok && re.code == "NOT_FOUND" {
				return map[string]any{"found": false}, nil
			}
			return nil, err
		}
		if !getRes.Found {
			return map[string]any{"found": false}, nil
		}
		return map[string]any{"found": true, "url": getRes.Body}, nil
	case "record_click":
		if err := e.ensureBloom(); err != nil {
			return nil, err
		}
		var p struct {
			Code string `json:"code"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Code == "" {
			return nil, &gwError{"INVALID_PARAMS", "record_click requires code"}
		}
		body := fmt.Sprintf("%d", time.Now().UnixMilli())
		key := "clicks/" + p.Code
		if err := rpc(e.store.addr, "put", map[string]any{"key": key, "body": body}, nil); err != nil {
			return nil, err
		}
		return map[string]any{}, nil
	default:
		return nil, &gwError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	eng := &engine{
		idgen: peer{addr: envAddr("IDGEN_ADDR")},
		bloom: peer{addr: envAddr("BLOOM_ADDR")},
		store: peer{addr: envAddr("STORE_ADDR")},
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
