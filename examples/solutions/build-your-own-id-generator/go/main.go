// Reference solution for "Build your own ID generator" (Go). Passes all 9 stages.
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
	"time"
)

const (
	snowflakeEpochMS = 1577836800000
	maxWorkerID      = 1023
	maxSequence      = 4095
	maxBatch         = 1024
)

type idError struct{ code, message string }

func (e *idError) Error() string { return e.code + ": " + e.message }

func nowMS() int64 { return time.Now().UnixMilli() }

type persisted struct {
	LastTimestampMS int64 `json:"last_timestamp_ms"`
	LastSequence    int64 `json:"last_sequence"`
	WorkerID        int64 `json:"worker_id"`
}

type engine struct {
	mu        sync.Mutex
	statePath string
	workerID  int64
	lastTS    int64
	lastSeq   int64
}

func newEngine(dataDir string) *engine {
	e := &engine{statePath: filepath.Join(dataDir, "state.json")}
	e.load()
	return e
}

func (e *engine) load() {
	data, err := os.ReadFile(e.statePath)
	if err != nil {
		return
	}
	var st persisted
	if json.Unmarshal(data, &st) == nil {
		e.lastTS = st.LastTimestampMS
		e.lastSeq = st.LastSequence
		e.workerID = st.WorkerID
	}
}

func (e *engine) persist() {
	data, _ := json.Marshal(persisted{
		LastTimestampMS: e.lastTS,
		LastSequence:    e.lastSeq,
		WorkerID:        e.workerID,
	})
	tmp := e.statePath + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, e.statePath)
	}
}

func composeID(ts, worker, seq int64) string {
	rel := ts - snowflakeEpochMS
	if rel < 0 {
		rel = 0
	}
	id := (rel << 22) | (worker << 12) | seq
	return strconv.FormatInt(id, 10)
}

func decodeID(id int64) (timestampMS, workerID, sequence int64) {
	sequence = id & maxSequence
	workerID = (id >> 12) & maxWorkerID
	rel := id >> 22
	timestampMS = rel + snowflakeEpochMS
	return
}

func (e *engine) waitNextMS(last int64) int64 {
	for {
		now := nowMS()
		if now > last {
			return now
		}
		time.Sleep(time.Millisecond)
	}
}

func (e *engine) allocInBucket(bucket *int64) (string, error) {
	if *bucket < e.lastTS {
		return "", &idError{"CLOCK_BACKWARDS", "clock moved backwards"}
	}
	if e.lastTS == *bucket {
		if e.lastSeq >= maxSequence {
			*bucket = e.waitNextMS(*bucket)
			if *bucket < e.lastTS {
				return "", &idError{"CLOCK_BACKWARDS", "clock moved backwards"}
			}
			e.lastTS = *bucket
			e.lastSeq = 0
		} else {
			e.lastSeq++
		}
	} else {
		e.lastTS = *bucket
		e.lastSeq = 0
	}
	return composeID(e.lastTS, e.workerID, e.lastSeq), nil
}

func (e *engine) nextID() (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	bucket := nowMS()
	id, err := e.allocInBucket(&bucket)
	if err != nil {
		return "", err
	}
	e.persist()
	return id, nil
}

func (e *engine) batch(count int64) ([]string, error) {
	if count < 1 {
		return nil, &idError{"INVALID_PARAMS", "count must be >= 1"}
	}
	if count > maxBatch {
		return nil, &idError{"BATCH_TOO_LARGE", fmt.Sprintf("count must be <= %d", maxBatch)}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	bucket := nowMS()
	ids := make([]string, 0, count)
	for i := int64(0); i < count; i++ {
		id, err := e.allocInBucket(&bucket)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	e.persist()
	return ids, nil
}

func (e *engine) configure(workerID int64) error {
	if workerID < 0 || workerID > maxWorkerID {
		return &idError{"INVALID_PARAMS", "worker_id must be 0..1023"}
	}
	e.mu.Lock()
	e.workerID = workerID
	e.mu.Unlock()
	e.persist()
	return nil
}

func (e *engine) parse(idStr string) (map[string]any, error) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return nil, &idError{"INVALID_PARAMS", "id must be a positive decimal integer"}
	}
	ts, worker, seq := decodeID(id)
	return map[string]any{
		"timestamp_ms": ts,
		"worker_id":    worker,
		"sequence":     seq,
	}, nil
}

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func main() {
	port := flag.Int("port", 0, "")
	dataDir := flag.String("data-dir", "", "")
	flag.Parse()
	eng := newEngine(*dataDir)

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("listening on 127.0.0.1:%d\n", *port)

	handle := func(method string, raw json.RawMessage) (any, error) {
		switch method {
		case "ping":
			return map[string]string{"message": "pong"}, nil
		case "configure":
			var p struct {
				WorkerID *int64 `json:"worker_id"`
			}
			if len(raw) > 0 {
				_ = json.Unmarshal(raw, &p)
			}
			if p.WorkerID == nil {
				return nil, &idError{"INVALID_PARAMS", "configure requires worker_id"}
			}
			if err := eng.configure(*p.WorkerID); err != nil {
				return nil, err
			}
			return map[string]any{}, nil
		case "next_id":
			id, err := eng.nextID()
			if err != nil {
				return nil, err
			}
			return map[string]string{"id": id}, nil
		case "batch":
			var p struct {
				Count *int64 `json:"count"`
			}
			if len(raw) > 0 {
				_ = json.Unmarshal(raw, &p)
			}
			if p.Count == nil {
				return nil, &idError{"INVALID_PARAMS", "batch requires count"}
			}
			ids, err := eng.batch(*p.Count)
			if err != nil {
				return nil, err
			}
			return map[string]any{"ids": ids}, nil
		case "parse":
			var p struct {
				ID string `json:"id"`
			}
			if len(raw) > 0 {
				_ = json.Unmarshal(raw, &p)
			}
			if p.ID == "" {
				return nil, &idError{"INVALID_PARAMS", "parse requires id"}
			}
			return eng.parse(p.ID)
		default:
			return nil, &idError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
		}
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			sc := bufio.NewScanner(c)
			sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			w := bufio.NewWriter(c)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				res, err := handle(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					if ie, ok := err.(*idError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": ie.code, "message": ie.message}}
					} else {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": "INTERNAL", "message": err.Error()}}
					}
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
