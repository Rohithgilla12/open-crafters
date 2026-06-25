// Reference solution for the open-crafters "Build your own LSM-tree" challenge (Go).
//
// An LSM-tree key-value store with memtable, SST1 on-disk format, flush, scan,
// compact, and tombstones. Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const sstMagic = "SST1"

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

type memEntry struct {
	value   string
	deleted bool
}

type Store struct {
	mu        sync.Mutex
	sstDir    string
	mem       map[string]memEntry
	sstFiles  []string
	nextSeq   int
}

func NewStore(dataDir string) (*Store, error) {
	s := &Store{
		sstDir: filepath.Join(dataDir, "sst"),
		mem:    map[string]memEntry{},
		nextSeq: 1,
	}
	if err := os.MkdirAll(s.sstDir, 0o755); err != nil {
		return nil, err
	}
	return s, s.loadIndex()
}

func (s *Store) loadIndex() error {
	ents, err := os.ReadDir(s.sstDir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sst" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	s.sstFiles = nil
	for _, n := range names {
		s.sstFiles = append(s.sstFiles, filepath.Join(s.sstDir, n))
	}
	if len(names) > 0 {
		var seq int
		fmt.Sscanf(names[len(names)-1], "%d.sst", &seq)
		s.nextSeq = seq + 1
	}
	return nil
}

func encodeSST(entries [][3]any) []byte {
	buf := make([]byte, 8)
	copy(buf[0:4], sstMagic)
	binary.LittleEndian.PutUint32(buf[4:8], uint32(len(entries)))
	for _, e := range entries {
		key := e[0].(string)
		val := e[1].(string)
		deleted := e[2].(bool)
		keyB := []byte(key)
		var valB []byte
		if !deleted {
			valB = []byte(val)
		}
		recLen := 4 + len(keyB) + 4 + len(valB)
		rec := make([]byte, recLen)
		off := 0
		binary.LittleEndian.PutUint32(rec[off:off+4], uint32(len(keyB)))
		off += 4
		copy(rec[off:off+len(keyB)], keyB)
		off += len(keyB)
		binary.LittleEndian.PutUint32(rec[off:off+4], uint32(len(valB)))
		off += 4
		copy(rec[off:], valB)
		buf = append(buf, rec...)
	}
	return buf
}

func parseSST(data []byte) ([][3]any, error) {
	if len(data) < 8 || string(data[0:4]) != sstMagic {
		return nil, fmt.Errorf("invalid SST")
	}
	count := binary.LittleEndian.Uint32(data[4:8])
	offset := 8
	var out [][3]any
	for i := uint32(0); i < count; i++ {
		if offset+4 > len(data) {
			return nil, fmt.Errorf("truncated SST")
		}
		keyLen := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		if offset+int(keyLen) > len(data) {
			return nil, fmt.Errorf("truncated SST")
		}
		key := string(data[offset : offset+int(keyLen)])
		offset += int(keyLen)
		if offset+4 > len(data) {
			return nil, fmt.Errorf("truncated SST")
		}
		valLen := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		if offset+int(valLen) > len(data) {
			return nil, fmt.Errorf("truncated SST")
		}
		val := string(data[offset : offset+int(valLen)])
		offset += int(valLen)
		out = append(out, [3]any{key, val, valLen == 0})
	}
	return out, nil
}

func (s *Store) readSST(path string) ([][3]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseSST(data)
}

func (s *Store) writeSST(entries [][3]any) (string, error) {
	path := filepath.Join(s.sstDir, fmt.Sprintf("%06d.sst", s.nextSeq))
	data := encodeSST(entries)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return "", err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	s.sstFiles = append(s.sstFiles, path)
	s.nextSeq++
	return path, nil
}

func (s *Store) lookup(key string) (string, bool) {
	if e, ok := s.mem[key]; ok {
		if e.deleted {
			return "", false
		}
		return e.value, true
	}
	for i := len(s.sstFiles) - 1; i >= 0; i-- {
		entries, err := s.readSST(s.sstFiles[i])
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e[0].(string) == key {
				if e[2].(bool) {
					return "", false
				}
				return e[1].(string), true
			}
		}
	}
	return "", false
}

func (s *Store) mergedLive() map[string]string {
	resolved := map[string]*bool{}
	values := map[string]string{}
	for _, path := range s.sstFiles {
		entries, _ := s.readSST(path)
		for _, e := range entries {
			key := e[0].(string)
			if e[2].(bool) {
				t := true
				resolved[key] = &t
			} else {
				f := false
				resolved[key] = &f
				values[key] = e[1].(string)
			}
		}
	}
	for key, e := range s.mem {
		if e.deleted {
			t := true
			resolved[key] = &t
		} else {
			f := false
			resolved[key] = &f
			values[key] = e.value
		}
	}
	live := map[string]string{}
	for key, del := range resolved {
		if !*del {
			live[key] = values[key]
		}
	}
	return live
}

func (s *Store) handle(method string, params json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "put":
		var p struct{ Key, Value string }
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.mem[p.Key] = memEntry{value: p.Value}
		s.mu.Unlock()
		return map[string]any{}, nil
	case "get":
		var p struct{ Key string }
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.mu.Lock()
		val, ok := s.lookup(p.Key)
		s.mu.Unlock()
		if ok {
			return map[string]any{"value": val, "found": true}, nil
		}
		return map[string]any{"value": nil, "found": false}, nil
	case "del":
		var p struct{ Key string }
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.mu.Lock()
		_, existed := s.lookup(p.Key)
		s.mem[p.Key] = memEntry{deleted: true}
		s.mu.Unlock()
		return map[string]any{"deleted": existed}, nil
	case "flush":
		s.mu.Lock()
		defer s.mu.Unlock()
		if len(s.mem) == 0 {
			return map[string]any{}, nil
		}
		var keys []string
		for k := range s.mem {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var entries [][3]any
		for _, k := range keys {
			e := s.mem[k]
			entries = append(entries, [3]any{k, e.value, e.deleted})
		}
		if _, err := s.writeSST(entries); err != nil {
			return nil, err
		}
		s.mem = map[string]memEntry{}
		return map[string]any{}, nil
	case "scan":
		var p struct{ Start, End string }
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		s.mu.Lock()
		live := s.mergedLive()
		s.mu.Unlock()
		var keys []string
		for k := range live {
			if k >= p.Start && k < p.End {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		var entries []map[string]string
		for _, k := range keys {
			entries = append(entries, map[string]string{"key": k, "value": live[k]})
		}
		return map[string]any{"entries": entries}, nil
	case "compact":
		s.mu.Lock()
		defer s.mu.Unlock()
		if len(s.sstFiles) < 2 {
			return map[string]any{}, nil
		}
		resolved := map[string][2]any{}
		for _, path := range s.sstFiles {
			entries, _ := s.readSST(path)
			for _, e := range entries {
				resolved[e[0].(string)] = [2]any{e[1].(string), e[2].(bool)}
			}
		}
		var keys []string
		for k := range resolved {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var merged [][3]any
		for _, k := range keys {
			v, d := resolved[k][0].(string), resolved[k][1].(bool)
			merged = append(merged, [3]any{k, v, d})
		}
		old := append([]string{}, s.sstFiles...)
		if _, err := s.writeSST(merged); err != nil {
			return nil, err
		}
		var kept []string
		for _, p := range s.sstFiles {
			remove := false
			for _, o := range old {
				if p == o {
					remove = true
					break
				}
			}
			if remove {
				os.Remove(p)
			} else {
				kept = append(kept, p)
			}
		}
		s.sstFiles = kept
		return map[string]any{}, nil
	default:
		return nil, &rpcError{Code: "UNKNOWN_METHOD", Message: fmt.Sprintf("unknown method %q", method)}
	}
}

func serveConn(conn net.Conn, store *Store) {
	defer conn.Close()
	enc := json.NewEncoder(conn)
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		var req request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			continue
		}
		result, err := store.handle(req.Method, req.Params)
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

	store, err := NewStore(*dataDir)
	if err != nil {
		log.Fatal(err)
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("lsm kv listening on 127.0.0.1:%d\n", *port)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go serveConn(conn, store)
	}
}
