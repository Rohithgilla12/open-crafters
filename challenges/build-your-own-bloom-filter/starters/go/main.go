// Starter template for "Build your own bloom filter" (Go). Passes stage 1 only.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
)

type bfError struct{ code, message string }

func (e *bfError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func handle(method string) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	// TODO (stage 2): create filter (m bits, k hashes) + FILTER_EXISTS / INVALID_PARAMS
	// TODO (stage 3): add item via FNV-1a double hash + FILTER_NOT_FOUND
	// TODO (stage 4): contains → maybe_present (all k bits set)
	// TODO (stage 5): sparse filter — never-added items usually false
	// TODO (stage 6): no false negatives under bulk insert
	// TODO (stage 7): independent filters per filter_id
	// TODO (stage 8): all k positions — (h1 + i*h2) % m, not just h1 % m
	// TODO (stage 9): concurrent add/contains + optional delete_filter
	return nil, &bfError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
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
					re := err.(*bfError)
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
