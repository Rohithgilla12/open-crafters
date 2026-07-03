// Starter template for "Build your own ID generator" (Go). Passes stage 1 only.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
)

type idError struct{ code, message string }

func (e *idError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func handle(method string) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	// TODO (stage 2): next_id, parse, configure
	// TODO (stage 5): per-ms sequence counter
	// TODO (stage 6): wait for next ms when sequence == 4095
	// TODO (stage 7): batch
	// TODO (stage 8): persist last timestamp + sequence to --data-dir
	return nil, &idError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
}

func main() {
	port := flag.Int("port", 0, "")
	dataDir := flag.String("data-dir", "", "")
	flag.Parse()
	_ = dataDir
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
					re := err.(*idError)
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
