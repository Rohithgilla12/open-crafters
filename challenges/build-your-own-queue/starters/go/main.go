// Starter template for "Build your own message queue" (Go).
//
// This boots a TCP server speaking newline-delimited JSON and answers `ping` —
// enough to pass the first stage. Extend handleRequest stage by stage.
// See PROTOCOL.md for the wire protocol and the delivery model (the real spec).
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
)

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func handleRequest(req request) (result any, rpcErr *rpcError) {
	switch req.Method {
	case "ping":
		return map[string]any{"message": "pong"}, nil

	// TODO (stage 2): send / receive / ack — an in-memory FIFO queue
	// TODO (stage 3): persist to --data-dir; un-acked messages survive SIGKILL
	// TODO (stage 4): visibility timeouts — redeliver if not acked in time
	// TODO (stage 5): nack — make a message visible again immediately
	// TODO (stage 6): receipt fencing — a receipt is valid for one delivery
	// TODO (stage 7): configure + dead-letter after max_receives failures
	// TODO (stage 8): many independent queues + stats

	default:
		return nil, &rpcError{Code: "UNKNOWN_METHOD", Message: fmt.Sprintf("unknown method %q", req.Method)}
	}
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(conn)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			encoder.Encode(map[string]any{"id": nil, "error": rpcError{Code: "BAD_REQUEST", Message: err.Error()}})
			continue
		}
		result, rpcErr := handleRequest(req)
		if rpcErr != nil {
			encoder.Encode(map[string]any{"id": req.ID, "error": rpcErr})
		} else {
			encoder.Encode(map[string]any{"id": req.ID, "result": result})
		}
	}
}

func main() {
	port := flag.Int("port", 0, "port to listen on")
	dataDir := flag.String("data-dir", "", "directory for durable state (used from stage 3)")
	flag.Parse()
	_ = dataDir

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn)
	}
}
