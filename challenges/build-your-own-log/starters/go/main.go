// Starter template for "Build your own log" (Go).
//
// Boots a TCP server speaking newline-delimited JSON and answers `ping` —
// enough to pass the first stage. Extend handleRequest stage by stage.
// See PROTOCOL.md for the wire protocol and the log model (the real spec).
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

	// TODO (stage 2): append / read — monotonic 0-based offsets per topic
	// TODO (stage 3): persist to --data-dir (records + offsets survive a crash)
	// TODO (stage 4): multiple independent topics
	// TODO (stage 5): commit_offset / committed_offset (consumer groups)
	// TODO (stage 6): read `max` batching; reads are replayable, non-destructive
	// TODO (stage 7): truncate — retention that keeps offsets ABSOLUTE
	// TODO (stage 8): persist committed offsets and retention state

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
	dataDir := flag.String("data-dir", "", "directory for durable state (used from the durability stage)")
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
