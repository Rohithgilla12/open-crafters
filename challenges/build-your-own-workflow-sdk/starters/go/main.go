// Starter template for "Build your own workflow SDK" (Go).
//
// Boots a TCP server speaking newline-delimited JSON and answers `ping` —
// enough to pass the first stage. Extend handleRequest stage by stage.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
)

type engineError struct {
	code, message string
}

func (e *engineError) Error() string { return e.code + ": " + e.message }
func errf(code, format string, a ...any) *engineError {
	return &engineError{code, fmt.Sprintf(format, a...)}
}

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func handleRequest(method string, _ json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	// TODO (stage 2): replay — greet workflow → COMPLETE_WORKFLOW
	// TODO (stage 3): fetch workflow → SCHEDULE_ACTIVITY
	// TODO (stage 4): fetch after ACTIVITY_TASK_COMPLETED → COMPLETE_WORKFLOW
	// TODO (stage 5): waiting states → empty commands
	// TODO (stage 6): timer_wait workflow
	// TODO (stage 7): signal_wait workflow
	// TODO (stage 8): determinism — no randomness or wall clock in replay
	// TODO (stage 9): pipeline workflow (gauntlet)
	default:
		return nil, errf("UNKNOWN_METHOD", "unknown method %q", method)
	}
}

func main() {
	port := flag.Int("port", 0, "TCP port")
	dataDir := flag.String("data-dir", "", "data directory")
	flag.Parse()
	if *port == 0 || *dataDir == "" {
		log.Fatal("usage: your_program.sh --port PORT --data-dir DIR")
	}
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
				if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
					continue
				}
				result, err := handleRequest(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					ee := err.(*engineError)
					resp = map[string]any{"id": req.ID, "error": map[string]string{"code": ee.code, "message": ee.message}}
				} else {
					resp = map[string]any{"id": req.ID, "result": result}
				}
				b, _ := json.Marshal(resp)
				w.Write(b)
				w.WriteByte('\n')
				w.Flush()
			}
		}(conn)
	}
}
