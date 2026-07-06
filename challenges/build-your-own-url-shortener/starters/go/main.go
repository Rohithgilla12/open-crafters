// Starter for URL shortener gateway (Go). Passes bind only.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
)

type gwError struct{ code, message string }

func (e *gwError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func handle(method string) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	_ = os.Getenv("IDGEN_ADDR")
	_ = os.Getenv("BLOOM_ADDR")
	_ = os.Getenv("STORE_ADDR")
	// TODO: shorten, resolve, record_click — call child services over TCP
	return nil, &gwError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
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
			w := bufio.NewWriter(c)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				res, err := handle(req.Method)
				var resp map[string]any
				if err != nil {
					re := err.(*gwError)
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
