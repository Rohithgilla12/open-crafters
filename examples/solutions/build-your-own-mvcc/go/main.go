// Reference solution for "Build your own MVCC" (Go).
//
// A transactional KV store with snapshot isolation: begin captures a snapshot
// (the latest committed sequence), reads are multi-version and frozen at that
// point, commit is durable and assigns a monotonic sequence, and a write-write
// conflict (a key we wrote was committed by someone else after our snapshot)
// aborts with CONFLICT. Recovery replays the commit log. Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
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

type version struct {
	seq int
	val *string // nil = tombstone
}

type txn struct {
	snapshot int
	writes   map[string]*string // nil value = buffered tombstone
}

type commitRecord struct {
	Seq    int                `json:"seq"`
	Writes map[string]*string `json:"writes"`
}

type Store struct {
	mu         sync.Mutex
	versions   map[string][]version
	commitSeq  int
	txns       map[string]*txn
	txnCounter int
	logPath    string
	logFile    *os.File
}

func NewStore(dataDir string) (*Store, error) {
	s := &Store{
		versions: map[string][]version{},
		txns:     map[string]*txn{},
		logPath:  filepath.Join(dataDir, "commits.log"),
	}
	if err := s.recover(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	s.logFile = f
	return s, nil
}

func (s *Store) apply(rec commitRecord) {
	for key, val := range rec.Writes {
		s.versions[key] = append(s.versions[key], version{seq: rec.Seq, val: val})
	}
	if rec.Seq > s.commitSeq {
		s.commitSeq = rec.Seq
	}
}

func (s *Store) recover() error {
	data, err := os.ReadFile(s.logPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, line := range splitLines(data) {
		if len(line) == 0 {
			continue
		}
		var rec commitRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("corrupt commit log: %w", err)
		}
		s.apply(rec)
	}
	return nil
}

func splitLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			out = append(out, b[start:i])
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

func (s *Store) readCommitted(key string, snapshot int) (string, bool) {
	var found *string
	for _, v := range s.versions[key] { // increasing seq order
		if v.seq <= snapshot {
			found = v.val
		}
	}
	if found == nil {
		return "", false
	}
	return *found, true
}

func (s *Store) txnOf(params map[string]any) (*txn, *rpcError) {
	id, _ := params["txn"].(string)
	t := s.txns[id]
	if t == nil {
		return nil, &rpcError{"UNKNOWN_TXN", fmt.Sprintf("no open transaction %q", id)}
	}
	return t, nil
}

func (s *Store) handle(req request) (any, *rpcError) {
	var p map[string]any
	if len(req.Params) > 0 {
		json.Unmarshal(req.Params, &p)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	switch req.Method {
	case "ping":
		return map[string]any{"message": "pong"}, nil

	case "begin":
		s.txnCounter++
		id := "t" + strconv.Itoa(s.txnCounter)
		s.txns[id] = &txn{snapshot: s.commitSeq, writes: map[string]*string{}}
		return map[string]any{"txn": id}, nil

	case "get":
		t, e := s.txnOf(p)
		if e != nil {
			return nil, e
		}
		key, _ := p["key"].(string)
		if val, ok := t.writes[key]; ok {
			if val == nil {
				return map[string]any{"value": nil, "found": false}, nil
			}
			return map[string]any{"value": *val, "found": true}, nil
		}
		if val, ok := s.readCommitted(key, t.snapshot); ok {
			return map[string]any{"value": val, "found": true}, nil
		}
		return map[string]any{"value": nil, "found": false}, nil

	case "set":
		t, e := s.txnOf(p)
		if e != nil {
			return nil, e
		}
		key, _ := p["key"].(string)
		val, _ := p["value"].(string)
		t.writes[key] = &val
		return map[string]any{}, nil

	case "delete":
		t, e := s.txnOf(p)
		if e != nil {
			return nil, e
		}
		key, _ := p["key"].(string)
		t.writes[key] = nil
		return map[string]any{}, nil

	case "commit":
		id, _ := p["txn"].(string)
		t, e := s.txnOf(p)
		if e != nil {
			return nil, e
		}
		for key := range t.writes {
			hist := s.versions[key]
			if len(hist) > 0 && hist[len(hist)-1].seq > t.snapshot {
				delete(s.txns, id)
				return nil, &rpcError{"CONFLICT", fmt.Sprintf("key %q was modified by a concurrent transaction", key)}
			}
		}
		if len(t.writes) > 0 {
			s.commitSeq++
			rec := commitRecord{Seq: s.commitSeq, Writes: t.writes}
			if err := s.persist(rec); err != nil {
				return nil, &rpcError{"IO_ERROR", err.Error()}
			}
			s.apply(rec)
		}
		delete(s.txns, id)
		return map[string]any{"committed": true}, nil

	case "rollback":
		id, _ := p["txn"].(string)
		if _, e := s.txnOf(p); e != nil {
			return nil, e
		}
		delete(s.txns, id)
		return map[string]any{}, nil

	default:
		return nil, &rpcError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", req.Method)}
	}
}

func (s *Store) persist(rec commitRecord) error {
	body, _ := json.Marshal(rec)
	if _, err := s.logFile.Write(append(body, '\n')); err != nil {
		return err
	}
	return s.logFile.Sync()
}

func handleConn(conn net.Conn, store *Store) {
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
			encoder.Encode(map[string]any{"id": nil, "error": rpcError{"BAD_REQUEST", err.Error()}})
			continue
		}
		result, rpcErr := store.handle(req)
		if rpcErr != nil {
			encoder.Encode(map[string]any{"id": req.ID, "error": rpcErr})
		} else {
			encoder.Encode(map[string]any{"id": req.ID, "result": result})
		}
	}
}

func main() {
	port := flag.Int("port", 0, "port to listen on")
	dataDir := flag.String("data-dir", "", "directory for durable state")
	flag.Parse()

	store, err := NewStore(*dataDir)
	if err != nil {
		log.Fatalf("recovery failed: %v", err)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("mvcc store listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn, store)
	}
}
