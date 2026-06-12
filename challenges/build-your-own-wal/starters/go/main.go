// Starter template for "Build your own WAL" (Go).
//
// This boots a TCP server speaking newline-delimited JSON and answers `ping` —
// enough to pass the first stage. Extend handleRequest stage by stage.
// See PROTOCOL.md for the wire protocol AND the on-disk log format (graded!).
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

	// TODO (stage 2): set, get, del — in memory
	// TODO (stage 3): persist to --data-dir, write-before-ack
	// TODO (stage 4): the WAL record format: crc32 | length | JSON payload
	// TODO (stage 5): recovery = replay wal.log from byte 0
	// TODO (stage 6): stop at torn records, truncate the tail
	// TODO (stage 7): stop at the first CRC mismatch too
	// TODO (stage 8): checkpoint — snapshot.json, then reset the log

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
