// Starter template for "Build your own LSM-tree" (Go).
//
// Boots a TCP server speaking newline-delimited JSON and answers ping — enough
// for stage 1. Extend handleRequest stage by stage. See PROTOCOL.md for the
// wire protocol AND the on-disk SST format (graded!).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
)

type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return e.Code + ": " + e.Message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func handleRequest(method string, _ json.RawMessage) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	// TODO (stage 2): put, get, del — in memory
	// TODO (stage 3): flush memtable to <data-dir>/sst/NNNNNN.sst (SST1 format)
	// TODO (stage 4): recovery — load SST files on startup
	// TODO (stage 5): scan — range query across memtable + SST
	// TODO (stage 6): compact — merge all SST files into one
	// TODO (stage 7): tombstones — value_len=0 on flush after del
	return nil, &rpcError{Code: "UNKNOWN_METHOD", Message: fmt.Sprintf("unknown method %q", method)}
}

func serveConn(conn net.Conn) {
	defer conn.Close()
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			return
		}
		result, err := handleRequest(req.Method, req.Params)
		resp := map[string]any{"id": req.ID}
		if err != nil {
			if re, ok := err.(*rpcError); ok {
				resp["error"] = re
			} else {
				resp["error"] = map[string]string{"code": "BAD_REQUEST", "message": err.Error()}
			}
		} else {
			resp["result"] = result
		}
		_ = enc.Encode(resp)
	}
}

func main() {
	port := flag.Int("port", 0, "TCP port")
	dataDir := flag.String("data-dir", "", "data directory")
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
		go serveConn(conn)
	}
}

// silence unused import until you need bufio
var _ = bufio.NewReader
