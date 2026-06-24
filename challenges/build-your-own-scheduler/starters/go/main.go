// Starter template for "Build your own scheduler" (Go). Passes stage 1 only.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
)

type schedError struct{ code, message string }

func (e *schedError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func handle(method string) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	return nil, &schedError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
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
			w := bufio.NewWriter(c)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				res, err := handle(req.Method)
				var resp map[string]any
				if err != nil {
					se := err.(*schedError)
					resp = map[string]any{"id": req.ID, "error": map[string]string{"code": se.code, "message": se.message}}
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
