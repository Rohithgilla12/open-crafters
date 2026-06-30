// Reference solution for "Build your own bloom filter" (Go). Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
)

const (
	fnvOffset64 = uint64(14695981039346656037)
	fnvPrime64  = uint64(1099511628211)
)

type bfError struct{ code, message string }

func (e *bfError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func fnv1a64(data []byte) uint64 {
	hash := fnvOffset64
	for _, b := range data {
		hash ^= uint64(b)
		hash *= fnvPrime64
	}
	return hash
}

func hashPositions(item string, m, k int) []int {
	itemBytes := []byte(item)
	h1 := fnv1a64(itemBytes)
	h2Data := append(append([]byte(nil), itemBytes...), 0x01)
	h2 := fnv1a64(h2Data)
	positions := make([]int, k)
	for i := 0; i < k; i++ {
		positions[i] = int((h1 + uint64(i)*h2) % uint64(m))
	}
	return positions
}

type bloomFilter struct {
	m    int
	k    int
	bits []byte
}

func newBloomFilter(m, k int) *bloomFilter {
	return &bloomFilter{m: m, k: k, bits: make([]byte, (m+7)/8)}
}

func (f *bloomFilter) setBit(i int) {
	f.bits[i/8] |= 1 << (i % 8)
}

func (f *bloomFilter) getBit(i int) bool {
	return f.bits[i/8]&(1<<(i%8)) != 0
}

func (f *bloomFilter) add(item string) {
	for _, pos := range hashPositions(item, f.m, f.k) {
		f.setBit(pos)
	}
}

func (f *bloomFilter) contains(item string) bool {
	for _, pos := range hashPositions(item, f.m, f.k) {
		if !f.getBit(pos) {
			return false
		}
	}
	return true
}

type engine struct {
	mu      sync.Mutex
	filters map[string]*bloomFilter
}

func newEngine() *engine {
	return &engine{filters: map[string]*bloomFilter{}}
}

type createParams struct {
	FilterID string `json:"filter_id"`
	M        int    `json:"m"`
	K        int    `json:"k"`
}

type filterItemParams struct {
	FilterID string `json:"filter_id"`
	Item     string `json:"item"`
}

type filterIDParams struct {
	FilterID string `json:"filter_id"`
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "create":
		var p createParams
		if json.Unmarshal(raw, &p) != nil || p.FilterID == "" || p.M < 8 || p.K < 1 {
			return nil, &bfError{"INVALID_PARAMS", "create requires filter_id, m>=8, k>=1"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		if _, ok := e.filters[p.FilterID]; ok {
			return nil, &bfError{"FILTER_EXISTS", fmt.Sprintf("filter %q already exists", p.FilterID)}
		}
		e.filters[p.FilterID] = newBloomFilter(p.M, p.K)
		return map[string]any{}, nil
	case "add":
		var p filterItemParams
		if json.Unmarshal(raw, &p) != nil || p.FilterID == "" || p.Item == "" {
			return nil, &bfError{"INVALID_PARAMS", "add requires filter_id and item"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		f, ok := e.filters[p.FilterID]
		if !ok {
			return nil, &bfError{"FILTER_NOT_FOUND", fmt.Sprintf("no filter %q", p.FilterID)}
		}
		f.add(p.Item)
		return map[string]any{}, nil
	case "contains":
		var p filterItemParams
		if json.Unmarshal(raw, &p) != nil || p.FilterID == "" || p.Item == "" {
			return nil, &bfError{"INVALID_PARAMS", "contains requires filter_id and item"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		f, ok := e.filters[p.FilterID]
		if !ok {
			return nil, &bfError{"FILTER_NOT_FOUND", fmt.Sprintf("no filter %q", p.FilterID)}
		}
		return map[string]any{"maybe_present": f.contains(p.Item)}, nil
	case "delete_filter":
		var p filterIDParams
		if json.Unmarshal(raw, &p) != nil || p.FilterID == "" {
			return nil, &bfError{"INVALID_PARAMS", "delete_filter requires filter_id"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		if _, ok := e.filters[p.FilterID]; !ok {
			return map[string]any{"deleted": false}, nil
		}
		delete(e.filters, p.FilterID)
		return map[string]any{"deleted": true}, nil
	default:
		return nil, &bfError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	eng := newEngine()

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
		go func(c net.Conn) {
			defer c.Close()
			sc := bufio.NewScanner(c)
			sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			enc := json.NewEncoder(c)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				res, err := eng.handle(req.Method, req.Params)
				if err != nil {
					be := err.(*bfError)
					enc.Encode(map[string]any{"id": req.ID, "error": map[string]string{"code": be.code, "message": be.message}})
				} else {
					enc.Encode(map[string]any{"id": req.ID, "result": res})
				}
			}
		}(conn)
	}
}
