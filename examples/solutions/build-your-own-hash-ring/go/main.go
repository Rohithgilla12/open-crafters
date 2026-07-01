// Reference solution for "Build your own hash ring" (Go). Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"
	"sync"
)

const (
	fnvOffset64 = uint64(14695981039346656037)
	fnvPrime64  = uint64(1099511628211)
)

type hrError struct{ code, message string }

func (e *hrError) Error() string { return e.code + ": " + e.message }

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

func hashKey(key string) uint64 {
	return fnv1a64([]byte(key))
}

func vnodePosition(nodeID string, replica int) uint64 {
	return fnv1a64([]byte(nodeID + "#" + strconv.Itoa(replica)))
}

type vnode struct {
	position uint64
	nodeID   string
}

type ring struct {
	replicas int
	nodes    map[string]struct{}
}

func (r *ring) sortedNodes() []string {
	out := make([]string, 0, len(r.nodes))
	for id := range r.nodes {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func (r *ring) lookup(key string) (string, error) {
	if len(r.nodes) == 0 {
		return "", &hrError{"NO_NODES", "ring has no nodes"}
	}
	nodes := r.sortedNodes()
	vnodes := make([]vnode, 0, len(nodes)*r.replicas)
	for _, nodeID := range nodes {
		for i := 0; i < r.replicas; i++ {
			vnodes = append(vnodes, vnode{position: vnodePosition(nodeID, i), nodeID: nodeID})
		}
	}
	sort.Slice(vnodes, func(i, j int) bool {
		if vnodes[i].position != vnodes[j].position {
			return vnodes[i].position < vnodes[j].position
		}
		return vnodes[i].nodeID < vnodes[j].nodeID
	})
	h := hashKey(key)
	for _, v := range vnodes {
		if v.position >= h {
			return v.nodeID, nil
		}
	}
	return vnodes[0].nodeID, nil
}

type engine struct {
	mu    sync.Mutex
	rings map[string]*ring
}

func newEngine() *engine {
	return &engine{rings: map[string]*ring{}}
}

type createParams struct {
	RingID   string `json:"ring_id"`
	Replicas int    `json:"replicas"`
}

type ringNodeParams struct {
	RingID string `json:"ring_id"`
	NodeID string `json:"node_id"`
}

type lookupParams struct {
	RingID string `json:"ring_id"`
	Key    string `json:"key"`
}

type ringIDParams struct {
	RingID string `json:"ring_id"`
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "create_ring":
		var p createParams
		if json.Unmarshal(raw, &p) != nil || p.RingID == "" || p.Replicas < 1 {
			return nil, &hrError{"INVALID_PARAMS", "create_ring requires ring_id and replicas>=1"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		if _, ok := e.rings[p.RingID]; ok {
			return nil, &hrError{"RING_EXISTS", fmt.Sprintf("ring %q already exists", p.RingID)}
		}
		e.rings[p.RingID] = &ring{replicas: p.Replicas, nodes: map[string]struct{}{}}
		return map[string]any{}, nil
	case "add_node":
		var p ringNodeParams
		if json.Unmarshal(raw, &p) != nil || p.RingID == "" || p.NodeID == "" {
			return nil, &hrError{"INVALID_PARAMS", "add_node requires ring_id and node_id"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		r, ok := e.rings[p.RingID]
		if !ok {
			return nil, &hrError{"RING_NOT_FOUND", fmt.Sprintf("no ring %q", p.RingID)}
		}
		if _, ok := r.nodes[p.NodeID]; ok {
			return nil, &hrError{"NODE_EXISTS", fmt.Sprintf("node %q already on ring", p.NodeID)}
		}
		r.nodes[p.NodeID] = struct{}{}
		return map[string]any{}, nil
	case "remove_node":
		var p ringNodeParams
		if json.Unmarshal(raw, &p) != nil || p.RingID == "" || p.NodeID == "" {
			return nil, &hrError{"INVALID_PARAMS", "remove_node requires ring_id and node_id"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		r, ok := e.rings[p.RingID]
		if !ok {
			return nil, &hrError{"RING_NOT_FOUND", fmt.Sprintf("no ring %q", p.RingID)}
		}
		if _, ok := r.nodes[p.NodeID]; !ok {
			return map[string]any{"removed": false}, nil
		}
		delete(r.nodes, p.NodeID)
		return map[string]any{"removed": true}, nil
	case "lookup":
		var p lookupParams
		if json.Unmarshal(raw, &p) != nil || p.RingID == "" || p.Key == "" {
			return nil, &hrError{"INVALID_PARAMS", "lookup requires ring_id and key"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		r, ok := e.rings[p.RingID]
		if !ok {
			return nil, &hrError{"RING_NOT_FOUND", fmt.Sprintf("no ring %q", p.RingID)}
		}
		node, err := r.lookup(p.Key)
		if err != nil {
			return nil, err
		}
		return map[string]any{"node_id": node}, nil
	case "list_nodes":
		var p ringIDParams
		if json.Unmarshal(raw, &p) != nil || p.RingID == "" {
			return nil, &hrError{"INVALID_PARAMS", "list_nodes requires ring_id"}
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		r, ok := e.rings[p.RingID]
		if !ok {
			return nil, &hrError{"RING_NOT_FOUND", fmt.Sprintf("no ring %q", p.RingID)}
		}
		return map[string]any{"nodes": r.sortedNodes()}, nil
	default:
		return nil, &hrError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
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
					he := err.(*hrError)
					enc.Encode(map[string]any{"id": req.ID, "error": map[string]string{"code": he.code, "message": he.message}})
				} else {
					enc.Encode(map[string]any{"id": req.ID, "result": res})
				}
			}
		}(conn)
	}
}
