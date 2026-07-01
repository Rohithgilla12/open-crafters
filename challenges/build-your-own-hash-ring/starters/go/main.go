// Starter template for "Build your own hash ring" (Go). Passes stage 1 only.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
)

type hrError struct{ code, message string }

func (e *hrError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func handle(method string) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	// TODO (stage 2): create_ring + RING_EXISTS / INVALID_PARAMS
	// TODO (stage 3): add_node, lookup + RING_NOT_FOUND / NODE_EXISTS / NO_NODES
	// TODO (stage 4): deterministic FNV-1a vnode walk per PROTOCOL.md
	// TODO (stage 5): even key spread across 3 nodes
	// TODO (stage 6): add 4th node — fewer than 45% of keys move
	// TODO (stage 7): remove_node — keys remap, never return removed node
	// TODO (stage 8): replicas flatten load (virtual nodes)
	// TODO (stage 9): concurrent add/remove/lookup across 2 rings
	return nil, &hrError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "") // ignored; harness may pass it
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
					re := err.(*hrError)
					resp = map[string]any{"id": req.ID, "error": map[string]string{"code": re.code, "message": re.message}}
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
