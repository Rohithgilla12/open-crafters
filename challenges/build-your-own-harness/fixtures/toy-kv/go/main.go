// Toy KV fixture for the "Build your own harness" challenge. The real harness
// spawns this binary via the user's spawn method to exercise their grader.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
)

type toyError struct{ code, message string }

func (e *toyError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type store struct {
	mu   sync.Mutex
	data map[string]string
}

func newStore() *store {
	return &store{data: map[string]string{}}
}

func (s *store) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "set":
		var p struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &toyError{"INVALID_PARAMS", "set requires key and value"}
		}
		s.mu.Lock()
		s.data[p.Key] = p.Value
		s.mu.Unlock()
		return map[string]any{}, nil
	case "get":
		var p struct{ Key string `json:"key"` }
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &toyError{"INVALID_PARAMS", "get requires key"}
		}
		s.mu.Lock()
		v, ok := s.data[p.Key]
		s.mu.Unlock()
		if !ok {
			return map[string]any{"hit": false}, nil
		}
		return map[string]any{"hit": true, "value": v}, nil
	default:
		return nil, &toyError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	kv := newStore()
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
				res, err := kv.handle(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					if te, ok := err.(*toyError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": te.code, "message": te.message}}
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
