// Reference solution for "Build your own distributed cache" (Go). Passes all 9 stages.
package main

import (
	"bufio"
	"container/list"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type cacheError struct{ code, message string }

func (e *cacheError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type entry struct {
	value     string
	version   int
	expiresAt int64 // 0 = never
	elem      *list.Element
}

type cache struct {
	mu      sync.Mutex
	maxKeys int
	data    map[string]*entry
	order   *list.List
}

func newCache() *cache {
	return &cache{data: map[string]*entry{}, order: list.New()}
}

func (c *cache) isExpired(e *entry, now int64) bool {
	return e.expiresAt > 0 && now >= e.expiresAt
}

func (c *cache) removeKey(key string) {
	if e, ok := c.data[key]; ok {
		c.order.Remove(e.elem)
		delete(c.data, key)
	}
}

func (c *cache) touch(e *entry) {
	c.order.MoveToBack(e.elem)
}

func (c *cache) getEntry(key string) (*entry, bool) {
	now := time.Now().UnixMilli()
	e, ok := c.data[key]
	if !ok || c.isExpired(e, now) {
		if ok {
			c.removeKey(key)
		}
		return nil, false
	}
	c.touch(e)
	return e, true
}

func (c *cache) evictIfNeeded() {
	for c.maxKeys > 0 && len(c.data) >= c.maxKeys {
		front := c.order.Front()
		if front == nil {
			break
		}
		c.removeKey(front.Value.(string))
	}
}

func (c *cache) store(key, value string, ttlMS int, onlyIfAbsent bool, expectedVersion int, cas bool) (stored bool, swapped bool, version int, err error) {
	if key == "" || value == "" {
		return false, false, 0, &cacheError{"INVALID_PARAMS", "key and value required"}
	}
	now := time.Now().UnixMilli()
	var expiresAt int64
	if ttlMS > 0 {
		expiresAt = now + int64(ttlMS)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.data[key]; ok {
		if c.isExpired(e, now) {
			c.removeKey(key)
		} else {
			if onlyIfAbsent {
				return false, false, 0, nil
			}
			if cas {
				if e.version != expectedVersion {
					return false, false, 0, nil
				}
				e.value = value
				e.version++
				if ttlMS > 0 {
					e.expiresAt = expiresAt
				} else {
					e.expiresAt = 0
				}
				c.touch(e)
				return true, true, e.version, nil
			}
			e.value = value
			e.version++
			if ttlMS > 0 {
				e.expiresAt = expiresAt
			} else {
				e.expiresAt = 0
			}
			c.touch(e)
			return true, false, e.version, nil
		}
	}

	if onlyIfAbsent || !cas {
		c.evictIfNeeded()
		e := &entry{value: value, version: 1, expiresAt: expiresAt}
		e.elem = c.order.PushBack(key)
		c.data[key] = e
		if onlyIfAbsent {
			return true, false, 1, nil
		}
		return true, false, 1, nil
	}
	return false, false, 0, nil
}

type engine struct {
	cache *cache
}

func (eng *engine) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "configure":
		var p struct {
			MaxKeys int `json:"max_keys"`
		}
		if json.Unmarshal(raw, &p) != nil || p.MaxKeys < 1 {
			return nil, &cacheError{"INVALID_PARAMS", "configure requires max_keys >= 1"}
		}
		eng.cache.mu.Lock()
		eng.cache.maxKeys = p.MaxKeys
		eng.cache.mu.Unlock()
		return map[string]any{}, nil
	case "set":
		var p struct {
			Key   string `json:"key"`
			Value string `json:"value"`
			TTLMS int    `json:"ttl_ms"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" || p.Value == "" {
			return nil, &cacheError{"INVALID_PARAMS", "set requires key and value"}
		}
		_, _, ver, err := eng.cache.store(p.Key, p.Value, p.TTLMS, false, 0, false)
		if err != nil {
			return nil, err
		}
		return map[string]any{"version": ver}, nil
	case "get":
		var p struct{ Key string `json:"key"` }
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &cacheError{"INVALID_PARAMS", "get requires key"}
		}
		eng.cache.mu.Lock()
		e, ok := eng.cache.getEntry(p.Key)
		eng.cache.mu.Unlock()
		if !ok {
			return map[string]any{"hit": false}, nil
		}
		return map[string]any{"hit": true, "value": e.value, "version": e.version}, nil
	case "delete":
		var p struct{ Key string `json:"key"` }
		if json.Unmarshal(raw, &p) != nil || p.Key == "" {
			return nil, &cacheError{"INVALID_PARAMS", "delete requires key"}
		}
		eng.cache.mu.Lock()
		_, ok := eng.cache.getEntry(p.Key)
		if ok {
			eng.cache.removeKey(p.Key)
		}
		eng.cache.mu.Unlock()
		return map[string]any{"deleted": ok}, nil
	case "setnx":
		var p struct {
			Key   string `json:"key"`
			Value string `json:"value"`
			TTLMS int    `json:"ttl_ms"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" || p.Value == "" {
			return nil, &cacheError{"INVALID_PARAMS", "setnx requires key and value"}
		}
		stored, _, ver, err := eng.cache.store(p.Key, p.Value, p.TTLMS, true, 0, false)
		if err != nil {
			return nil, err
		}
		if !stored {
			return map[string]any{"stored": false}, nil
		}
		return map[string]any{"stored": true, "version": ver}, nil
	case "cas":
		var p struct {
			Key             string `json:"key"`
			ExpectedVersion int    `json:"expected_version"`
			Value           string `json:"value"`
			TTLMS           int    `json:"ttl_ms"`
		}
		if json.Unmarshal(raw, &p) != nil || p.Key == "" || p.Value == "" {
			return nil, &cacheError{"INVALID_PARAMS", "cas requires key, expected_version, value"}
		}
		_, swapped, ver, err := eng.cache.store(p.Key, p.Value, p.TTLMS, false, p.ExpectedVersion, true)
		if err != nil {
			return nil, err
		}
		if !swapped {
			return map[string]any{"swapped": false}, nil
		}
		return map[string]any{"swapped": true, "version": ver}, nil
	case "mget":
		var p struct {
			Keys []string `json:"keys"`
		}
		if json.Unmarshal(raw, &p) != nil || len(p.Keys) == 0 || len(p.Keys) > 50 {
			return nil, &cacheError{"INVALID_PARAMS", "mget requires 1..50 keys"}
		}
		entries := make([]map[string]any, len(p.Keys))
		eng.cache.mu.Lock()
		for i, key := range p.Keys {
			e, ok := eng.cache.getEntry(key)
			if !ok {
				entries[i] = map[string]any{"key": key, "hit": false}
				continue
			}
			entries[i] = map[string]any{"key": key, "hit": true, "value": e.value, "version": e.version}
		}
		eng.cache.mu.Unlock()
		return map[string]any{"entries": entries}, nil
	default:
		return nil, &cacheError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "")
	flag.String("data-dir", "", "")
	flag.Parse()
	eng := &engine{cache: newCache()}
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
			sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			w := bufio.NewWriter(c)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				res, err := eng.handle(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					if ce, ok := err.(*cacheError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": ce.code, "message": ce.message}}
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
