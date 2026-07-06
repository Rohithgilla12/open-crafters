// Starter template for "Build your own harness" (Go). Passes stage 1 only.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
)

type harnessError struct{ code, message string }

func (e *harnessError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func handle(method string) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	// TODO (stage 2): spawn {program} → {addr}
	// TODO (stage 3): call {addr, method, params} → child result
	// TODO (stage 4): call set/get on spawned toy KV
	// TODO (stage 5): run_case {program, method, params, expect}
	// TODO (stage 6): multiple run_case assertions
	// TODO (stage 7): independent spawns with fresh ports
	// TODO (stage 8): concurrent call proxying
	// TODO (stage 9): run_case isolation + multi-key proxy gauntlet
	return nil, &harnessError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
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
				res, err := handle(req.Method)
				var resp map[string]any
				if err != nil {
					he := err.(*harnessError)
					resp = map[string]any{"id": req.ID, "error": map[string]string{"code": he.code, "message": he.message}}
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
