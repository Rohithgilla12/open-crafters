// Reference solution for the open-crafters "Build your own WAL" challenge (Go).
//
// A key-value store made durable by a write-ahead log:
//   - record format: crc32(4, LE) | length(4, LE) | JSON payload, with the CRC
//     covering the length bytes followed by the payload (see PROTOCOL.md)
//   - fsync before acknowledging any write
//   - recovery replays wal.log on top of snapshot.json, stops at the first
//     invalid record, and truncates the torn/corrupt tail before accepting new
//     appends
//   - checkpoint: atomically snapshot full state, then reset the log
//
// Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
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

type record struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// Store is the durable key-value engine. One mutex guards both the in-memory
// map and the append handle, so concurrent connections can't interleave a
// write's log append with another's.
type Store struct {
	mu       sync.Mutex
	data     map[string]string
	walPath  string
	snapPath string
	wal      *os.File
}

func NewStore(dataDir string) (*Store, error) {
	s := &Store{
		data:     map[string]string{},
		walPath:  filepath.Join(dataDir, "wal.log"),
		snapPath: filepath.Join(dataDir, "snapshot.json"),
	}
	if err := s.recover(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.walPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	s.wal = f
	return s, nil
}

func encodeRecord(rec record) []byte {
	body, _ := json.Marshal(rec)
	buf := make([]byte, 8+len(body))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(body)))
	copy(buf[8:], body)
	binary.LittleEndian.PutUint32(buf[0:4], crc32.ChecksumIEEE(buf[4:]))
	return buf
}

func (s *Store) recover() error {
	if body, err := os.ReadFile(s.snapPath); err == nil {
		var snap struct {
			Data map[string]string `json:"data"`
		}
		if err := json.Unmarshal(body, &snap); err != nil {
			return fmt.Errorf("snapshot.json is not valid JSON: %w", err)
		}
		for k, v := range snap.Data {
			s.data[k] = v
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	raw, err := os.ReadFile(s.walPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	offset, validEnd := 0, 0
	for offset+8 <= len(raw) {
		storedCRC := binary.LittleEndian.Uint32(raw[offset : offset+4])
		length := int(binary.LittleEndian.Uint32(raw[offset+4 : offset+8]))
		if offset+8+length > len(raw) {
			break // torn payload
		}
		framed := raw[offset+4 : offset+8+length]
		if crc32.ChecksumIEEE(framed) != storedCRC {
			break // corrupt record: stop replay here
		}
		var rec record
		if err := json.Unmarshal(raw[offset+8:offset+8+length], &rec); err != nil {
			break
		}
		switch rec.Op {
		case "set":
			s.data[rec.Key] = rec.Value
		case "del":
			delete(s.data, rec.Key)
		}
		offset += 8 + length
		validEnd = offset
	}

	if validEnd < len(raw) {
		// Drop the torn/corrupt tail so the log parses cleanly from byte 0
		// and new appends don't land after garbage.
		f, err := os.OpenFile(s.walPath, os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := f.Truncate(int64(validEnd)); err != nil {
			return err
		}
		return f.Sync()
	}
	return nil
}

func (s *Store) appendRecord(rec record) error {
	if _, err := s.wal.Write(encodeRecord(rec)); err != nil {
		return err
	}
	return s.wal.Sync()
}

func (s *Store) set(params json.RawMessage) (any, *rpcError) {
	var p struct{ Key, Value string }
	json.Unmarshal(params, &p)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.appendRecord(record{Op: "set", Key: p.Key, Value: p.Value}); err != nil {
		return nil, &rpcError{"IO_ERROR", err.Error()}
	}
	s.data[p.Key] = p.Value
	return struct{}{}, nil
}

func (s *Store) get(params json.RawMessage) (any, *rpcError) {
	var p struct{ Key string }
	json.Unmarshal(params, &p)
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.data[p.Key]; ok {
		return map[string]any{"value": v, "found": true}, nil
	}
	return map[string]any{"value": nil, "found": false}, nil
}

func (s *Store) del(params json.RawMessage) (any, *rpcError) {
	var p struct{ Key string }
	json.Unmarshal(params, &p)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, existed := s.data[p.Key]
	if err := s.appendRecord(record{Op: "del", Key: p.Key}); err != nil {
		return nil, &rpcError{"IO_ERROR", err.Error()}
	}
	delete(s.data, p.Key)
	return map[string]any{"deleted": existed}, nil
}

func (s *Store) checkpoint(json.RawMessage) (any, *rpcError) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Snapshot must be durable BEFORE the log is reset: a crash in between
	// just replays the old log onto the new snapshot, which is harmless
	// because set/del are absolute.
	body, _ := json.Marshal(map[string]any{"data": s.data})
	tmp := s.snapPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, &rpcError{"IO_ERROR", err.Error()}
	}
	if _, err := f.Write(body); err != nil {
		f.Close()
		return nil, &rpcError{"IO_ERROR", err.Error()}
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return nil, &rpcError{"IO_ERROR", err.Error()}
	}
	f.Close()
	if err := os.Rename(tmp, s.snapPath); err != nil {
		return nil, &rpcError{"IO_ERROR", err.Error()}
	}

	s.wal.Close()
	reset, err := os.OpenFile(s.walPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, &rpcError{"IO_ERROR", err.Error()}
	}
	reset.Sync()
	reset.Close()
	s.wal, err = os.OpenFile(s.walPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, &rpcError{"IO_ERROR", err.Error()}
	}
	return struct{}{}, nil
}

func (s *Store) handle(req request) (any, *rpcError) {
	switch req.Method {
	case "ping":
		return map[string]any{"message": "pong"}, nil
	case "set":
		return s.set(req.Params)
	case "get":
		return s.get(req.Params)
	case "del":
		return s.del(req.Params)
	case "checkpoint":
		return s.checkpoint(req.Params)
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
	log.Printf("kv store listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn, store)
	}
}
