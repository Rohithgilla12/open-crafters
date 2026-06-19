// Reference solution for "Build your own log" (Go).
//
// An append-only, replayable log with absolute offsets, consumer-group offset
// tracking, and retention that never renumbers. Appends, commits, and
// truncations are durable across a crash (replayed from an event log on boot).
// Passes all 9 stages.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
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

type topic struct {
	base   int // offset of values[0]; rises with retention
	values []string
}

func (t *topic) end() int { return t.base + len(t.values) }

type offsetKey struct{ group, topic string }

type event struct {
	T      string `json:"t"`
	Topic  string `json:"topic,omitempty"`
	Value  string `json:"value,omitempty"`
	Before int    `json:"before,omitempty"`
	Group  string `json:"group,omitempty"`
	Offset int    `json:"offset,omitempty"`
}

type Store struct {
	mu      sync.Mutex
	topics  map[string]*topic
	offsets map[offsetKey]int
	logPath string
	logFile *os.File
}

func NewStore(dataDir string) (*Store, error) {
	s := &Store{
		topics:  map[string]*topic{},
		offsets: map[offsetKey]int{},
		logPath: filepath.Join(dataDir, "log.jsonl"),
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

func (s *Store) topic(name string) *topic {
	t := s.topics[name]
	if t == nil {
		t = &topic{}
		s.topics[name] = t
	}
	return t
}

func (s *Store) applyEvent(e event) {
	switch e.T {
	case "append":
		t := s.topic(e.Topic)
		t.values = append(t.values, e.Value)
	case "truncate":
		t := s.topic(e.Topic)
		if e.Before > t.base {
			drop := min(e.Before, t.end()) - t.base
			t.values = t.values[drop:]
			t.base += drop
		}
	case "commit":
		s.offsets[offsetKey{e.Group, e.Topic}] = e.Offset
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
	dec := json.NewDecoder(bytes.NewReader(data))
	for dec.More() {
		var e event
		if err := dec.Decode(&e); err != nil {
			return fmt.Errorf("corrupt log: %w", err)
		}
		s.applyEvent(e)
	}
	return nil
}

func (s *Store) persist(e event) error {
	body, _ := json.Marshal(e)
	if _, err := s.logFile.Write(append(body, '\n')); err != nil {
		return err
	}
	return s.logFile.Sync()
}

func intParam(p map[string]any, key string) int {
	if v, ok := p[key].(float64); ok {
		return int(v)
	}
	return 0
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

	case "append":
		name, _ := p["topic"].(string)
		val, _ := p["value"].(string)
		t := s.topic(name)
		offset := t.end()
		if err := s.persist(event{T: "append", Topic: name, Value: val}); err != nil {
			return nil, &rpcError{"IO_ERROR", err.Error()}
		}
		t.values = append(t.values, val)
		return map[string]any{"offset": offset}, nil

	case "read":
		name, _ := p["topic"].(string)
		offset := intParam(p, "offset")
		limit := 100
		if _, ok := p["max"]; ok {
			limit = intParam(p, "max")
		}
		t := s.topics[name]
		if t == nil {
			t = &topic{}
		}
		if offset < t.base {
			return nil, &rpcError{"OUT_OF_RANGE", fmt.Sprintf("offset %d is below the earliest retained offset %d", offset, t.base)}
		}
		records := []map[string]any{}
		i := offset
		for i < t.end() && len(records) < limit {
			records = append(records, map[string]any{"offset": i, "value": t.values[i-t.base]})
			i++
		}
		return map[string]any{"records": records, "next_offset": i}, nil

	case "commit_offset":
		group, _ := p["group"].(string)
		name, _ := p["topic"].(string)
		offset := intParam(p, "offset")
		if err := s.persist(event{T: "commit", Group: group, Topic: name, Offset: offset}); err != nil {
			return nil, &rpcError{"IO_ERROR", err.Error()}
		}
		s.offsets[offsetKey{group, name}] = offset
		return map[string]any{}, nil

	case "committed_offset":
		group, _ := p["group"].(string)
		name, _ := p["topic"].(string)
		return map[string]any{"offset": s.offsets[offsetKey{group, name}]}, nil

	case "truncate":
		name, _ := p["topic"].(string)
		before := intParam(p, "before")
		t := s.topic(name)
		if err := s.persist(event{T: "truncate", Topic: name, Before: before}); err != nil {
			return nil, &rpcError{"IO_ERROR", err.Error()}
		}
		if before > t.base {
			drop := min(before, t.end()) - t.base
			t.values = t.values[drop:]
			t.base += drop
		}
		return map[string]any{}, nil

	case "stats":
		name, _ := p["topic"].(string)
		t := s.topics[name]
		if t == nil {
			return map[string]any{"start_offset": 0, "end_offset": 0}, nil
		}
		return map[string]any{"start_offset": t.base, "end_offset": t.end()}, nil

	default:
		return nil, &rpcError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", req.Method)}
	}
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
	log.Printf("log store listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn, store)
	}
}
