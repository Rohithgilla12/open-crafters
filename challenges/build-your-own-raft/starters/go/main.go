// Starter template for "Build your own Raft" (Go).
//
// Boots a TCP server speaking newline-delimited JSON and answers `ping` with the
// node id — enough to pass the first stage. Extend handleRequest stage by stage.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
)

type raftError struct {
	code, message string
	extra         map[string]any
}

func (e *raftError) Error() string { return e.code + ": " + e.message }
func errf(code, format string, a ...any) *raftError {
	return &raftError{code, fmt.Sprintf(format, a...), nil}
}

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func parsePeers(peers string) map[string]string {
	out := make(map[string]string)
	for _, part := range strings.Split(peers, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, addr, ok := strings.Cut(part, "=")
		if ok {
			out[id] = addr
		}
	}
	return out
}

func handleRequest(method string, _ json.RawMessage, nodeID string) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong", "node_id": nodeID}, nil
	// TODO (stage 2): leader election — get_status, request_vote, append_entries heartbeats
	// TODO (stage 3): set on leader, replicate log entries to a quorum
	// TODO (stage 4): get — serve reads from committed/applied state
	// TODO (stage 5): tolerate a follower crash (majority still commits)
	// TODO (stage 6): re-elect after leader crash, preserve committed log
	// TODO (stage 7): persist term, voted_for, log, commit_index, KV to --data-dir
	// TODO (stage 8): partition safety — NOT_COMMITTED when quorum unreachable
	// TODO (stage 9): gauntlet — writes, crash, restart
	default:
		return nil, errf("UNKNOWN_METHOD", "unknown method %q", method)
	}
}

func main() {
	nodeID := flag.String("node-id", "", "this node's id")
	peers := flag.String("peers", "", "peer list")
	port := flag.Int("port", 0, "TCP port")
	dataDir := flag.String("data-dir", "", "data directory")
	flag.Parse()
	if *nodeID == "" || *peers == "" || *port == 0 || *dataDir == "" {
		log.Fatal("usage: your_program.sh --node-id ID --peers PEERS --port PORT --data-dir DIR")
	}
	_ = parsePeers(*peers)
	_ = dataDir

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("raft node %s listening on 127.0.0.1:%d\n", *nodeID, *port)

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
				result, err := handleRequest(req.Method, req.Params, *nodeID)
				var resp map[string]any
				if err != nil {
					ee := err.(*raftError)
					errObj := map[string]any{"code": ee.code, "message": ee.message}
					for k, v := range ee.extra {
						errObj[k] = v
					}
					resp = map[string]any{"id": req.ID, "error": errObj}
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
