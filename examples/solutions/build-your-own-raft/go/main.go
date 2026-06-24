// Reference solution for the open-crafters "Build your own Raft" challenge.
//
// A minimal but correct 3-node Raft cluster: newline-delimited JSON over TCP,
// leader election, log replication, quorum commit, crash-safe persistence, and
// partition safety. Passes all 9 stages. One mutex, election/heartbeat goroutine.
package main

import (
	"bufio"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type raftError struct {
	code    string
	message string
	extra   map[string]any
}

func (e *raftError) Error() string { return e.message }

func newRaftError(code, message string, extra map[string]any) *raftError {
	return &raftError{code: code, message: message, extra: extra}
}

type logEntry struct {
	Index int    `json:"index"`
	Term  int    `json:"term"`
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type persistedState struct {
	Term         int            `json:"term"`
	VotedFor     *string        `json:"voted_for"`
	Log          []logEntry     `json:"log"`
	CommitIndex  int            `json:"commit_index"`
	LastApplied  int            `json:"last_applied"`
	KV           map[string]any `json:"kv"`
}

type raftNode struct {
	mu sync.Mutex

	nodeID  string
	peers   map[string]string
	peerIDs []string
	quorum  int

	statePath string

	currentTerm int
	votedFor    *string
	log         []logEntry
	commitIndex int
	lastApplied int
	kv          map[string]any

	role             string
	leaderID         string
	nextIndex        map[string]int
	matchIndex       map[string]int
	votesReceived    map[string]struct{}
	electionDeadline time.Time
	lastQuorumContact time.Time
}

func parsePeers(peersStr string) map[string]string {
	peers := map[string]string{}
	for _, part := range strings.Split(peersStr, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		nodeID, addr, _ := strings.Cut(part, "=")
		peers[nodeID] = addr
	}
	return peers
}

func parseAddr(addr string) (string, int) {
	host, portStr, _ := strings.Cut(addr, ":")
	port, _ := strconv.Atoi(portStr)
	return host, port
}

func newRaftNode(nodeID string, peers map[string]string, dataDir string) (*raftNode, error) {
	peerIDs := make([]string, 0, len(peers))
	for id := range peers {
		peerIDs = append(peerIDs, id)
	}
	sort.Slice(peerIDs, func(i, j int) bool {
		a, _ := strconv.Atoi(peerIDs[i])
		b, _ := strconv.Atoi(peerIDs[j])
		return a < b
	})

	n := &raftNode{
		nodeID:    nodeID,
		peers:     peers,
		peerIDs:   peerIDs,
		quorum:    len(peerIDs)/2 + 1,
		statePath: filepath.Join(dataDir, "state.json"),
		kv:        map[string]any{},
		role:      "follower",
		leaderID:  "0",
		nextIndex: map[string]int{},
		matchIndex: map[string]int{},
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	if err := n.load(); err != nil {
		return nil, err
	}
	n.resetElectionTimer()
	return n, nil
}

func (n *raftNode) load() error {
	data, err := os.ReadFile(n.statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return err
	}
	n.currentTerm = state.Term
	n.votedFor = state.VotedFor
	n.log = state.Log
	if n.log == nil {
		n.log = []logEntry{}
	}
	n.commitIndex = state.CommitIndex
	n.lastApplied = state.LastApplied
	if state.KV != nil {
		n.kv = state.KV
	}
	return nil
}

func (n *raftNode) persist() {
	state := persistedState{
		Term:        n.currentTerm,
		VotedFor:    n.votedFor,
		Log:         n.log,
		CommitIndex: n.commitIndex,
		LastApplied: n.lastApplied,
		KV:          n.kv,
	}
	body, _ := json.Marshal(state)
	tmp := n.statePath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	f.Write(body)
	f.Sync()
	f.Close()
	os.Rename(tmp, n.statePath)
}

func (n *raftNode) lastLogIndex() int  { return len(n.log) }
func (n *raftNode) lastLogTerm() int {
	if len(n.log) == 0 {
		return 0
	}
	return n.log[len(n.log)-1].Term
}

func (n *raftNode) resetElectionTimer() {
	n.electionDeadline = time.Now().Add(time.Duration(300+rand.Intn(200)) * time.Millisecond)
}

func (n *raftNode) stepDown(term int) {
	n.currentTerm = term
	n.role = "follower"
	n.votedFor = nil
	n.leaderID = "0"
	n.persist()
	n.resetElectionTimer()
}

func (n *raftNode) becomeLeader() {
	n.role = "leader"
	n.leaderID = n.nodeID
	n.lastQuorumContact = time.Now()
	lastIdx := n.lastLogIndex()
	for _, pid := range n.peerIDs {
		if pid != n.nodeID {
			n.nextIndex[pid] = lastIdx + 1
			n.matchIndex[pid] = 0
		}
	}
}

func (n *raftNode) maybeStepDownLeader() {
	if n.role != "leader" {
		return
	}
	if time.Since(n.lastQuorumContact) > 500*time.Millisecond {
		n.role = "follower"
		n.leaderID = "0"
		n.resetElectionTimer()
	}
}

func (n *raftNode) applyCommitted() {
	for n.lastApplied < n.commitIndex {
		n.lastApplied++
		entry := n.log[n.lastApplied-1]
		n.kv[entry.Key] = entry.Value
	}
	n.persist()
}

func (n *raftNode) updateCommitIndex() {
	for idx := n.lastLogIndex(); idx > n.commitIndex; idx-- {
		count := 1
		for _, pid := range n.peerIDs {
			if pid != n.nodeID && n.matchIndex[pid] >= idx {
				count++
			}
		}
		if count >= n.quorum && n.log[idx-1].Term == n.currentTerm {
			n.commitIndex = idx
			n.applyCommitted()
			break
		}
	}
}

type rpcRequest struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func rpcID() string {
	var b [16]byte
	crand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (n *raftNode) rpc(peerID, method string, params any) (map[string]any, bool) {
	addr, ok := n.peers[peerID]
	if !ok {
		return nil, false
	}
	host, port := parseAddr(addr)
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 500*time.Millisecond)
	if err != nil {
		return nil, false
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(500 * time.Millisecond))

	payload, _ := json.Marshal(map[string]any{
		"id":     rpcID(),
		"method": method,
		"params": params,
	})
	payload = append(payload, '\n')
	if _, err := conn.Write(payload); err != nil {
		return nil, false
	}

	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return nil, false
	}
	var resp struct {
		Error  map[string]any `json:"error"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(scanner.Bytes(), &resp); err != nil {
		return nil, false
	}
	if resp.Error != nil {
		return nil, false
	}
	return resp.Result, true
}

func (n *raftNode) startElection() {
	n.mu.Lock()
	n.role = "candidate"
	n.currentTerm++
	self := n.nodeID
	n.votedFor = &self
	n.leaderID = "0"
	n.votesReceived = map[string]struct{}{n.nodeID: {}}
	n.persist()
	term := n.currentTerm
	lastIdx := n.lastLogIndex()
	lastTerm := n.lastLogTerm()
	n.resetElectionTimer()
	n.mu.Unlock()

	for _, pid := range n.peerIDs {
		if pid == n.nodeID {
			continue
		}
		go n.requestVote(pid, term, lastIdx, lastTerm)
	}
}

func (n *raftNode) requestVote(peerID string, term, lastIdx, lastTerm int) {
	result, ok := n.rpc(peerID, "request_vote", map[string]any{
		"term":            term,
		"candidate_id":    n.nodeID,
		"last_log_index":  lastIdx,
		"last_log_term":   lastTerm,
	})
	if !ok || result == nil {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	resultTerm := int(result["term"].(float64))
	if resultTerm > n.currentTerm {
		n.stepDown(resultTerm)
		return
	}
	if n.role != "candidate" || n.currentTerm != term {
		return
	}
	if result["vote_granted"] == true {
		n.votesReceived[peerID] = struct{}{}
		if len(n.votesReceived) >= n.quorum {
			n.becomeLeader()
		}
	}
}

func (n *raftNode) replicateTo(peerID string) {
	n.mu.Lock()
	if n.role != "leader" {
		n.mu.Unlock()
		return
	}
	nextIdx := n.nextIndex[peerID]
	if nextIdx == 0 {
		nextIdx = 1
	}
	prevLogIndex := nextIdx - 1
	prevLogTerm := 0
	if prevLogIndex > 0 {
		prevLogTerm = n.log[prevLogIndex-1].Term
	}
	var entries []logEntry
	if nextIdx <= n.lastLogIndex() {
		entries = append([]logEntry(nil), n.log[nextIdx-1:]...)
	}
	term := n.currentTerm
	leaderCommit := n.commitIndex
	n.mu.Unlock()

	result, ok := n.rpc(peerID, "append_entries", map[string]any{
		"term":             term,
		"leader_id":        n.nodeID,
		"prev_log_index":   prevLogIndex,
		"prev_log_term":    prevLogTerm,
		"entries":          entries,
		"leader_commit":    leaderCommit,
	})
	if !ok || result == nil {
		return
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	resultTerm := int(result["term"].(float64))
	if resultTerm > n.currentTerm {
		n.stepDown(resultTerm)
		return
	}
	if n.role != "leader" || n.currentTerm != term {
		return
	}
	if result["success"] == true {
		n.lastQuorumContact = time.Now()
		if len(entries) > 0 {
			n.matchIndex[peerID] = nextIdx + len(entries) - 1
			n.nextIndex[peerID] = n.matchIndex[peerID] + 1
		}
		n.updateCommitIndex()
	} else if n.nextIndex[peerID] > 1 {
		n.nextIndex[peerID]--
	}
}

func (n *raftNode) leaderHeartbeat() {
	for _, pid := range n.peerIDs {
		if pid != n.nodeID {
			go n.replicateTo(pid)
		}
	}
}

func (n *raftNode) runRaftLoop() {
	lastHeartbeat := time.Time{}
	for {
		time.Sleep(50 * time.Millisecond)
		now := time.Now()
		needElection := false

		n.mu.Lock()
		if n.role == "leader" {
			n.maybeStepDownLeader()
			if n.role == "leader" && now.Sub(lastHeartbeat) >= 100*time.Millisecond {
				lastHeartbeat = now
				n.mu.Unlock()
				n.leaderHeartbeat()
				continue
			}
		} else if now.After(n.electionDeadline) {
			needElection = true
		}
		n.mu.Unlock()

		if needElection {
			n.startElection()
		}
	}
}

func paramsMap(raw json.RawMessage) map[string]any {
	p := map[string]any{}
	if len(raw) > 0 {
		json.Unmarshal(raw, &p)
	}
	return p
}

func intParam(p map[string]any, key string) int {
	if v, ok := p[key].(float64); ok {
		return int(v)
	}
	return 0
}

func strParam(p map[string]any, key string) string {
	if v, ok := p[key].(string); ok {
		return v
	}
	return ""
}

func (n *raftNode) handle(method string, p map[string]any) (any, *raftError) {
	switch method {
	case "ping":
		return map[string]any{"message": "pong", "node_id": n.nodeID}, nil

	case "get_status":
		n.mu.Lock()
		defer n.mu.Unlock()
		return map[string]any{
			"node_id":      n.nodeID,
			"role":         n.role,
			"term":         n.currentTerm,
			"leader_id":    n.leaderID,
			"commit_index": n.commitIndex,
			"last_applied": n.lastApplied,
		}, nil

	case "set":
		n.mu.Lock()
		if n.role != "leader" {
			lid := n.leaderID
			if lid == "" {
				lid = "0"
			}
			n.mu.Unlock()
			return nil, newRaftError("NOT_LEADER", "not the leader", map[string]any{"leader_id": lid})
		}
		index := n.lastLogIndex() + 1
		entry := logEntry{
			Index: index,
			Term:  n.currentTerm,
			Key:   strParam(p, "key"),
			Value: p["value"],
		}
		n.log = append(n.log, entry)
		n.persist()
		targetIndex := index
		n.mu.Unlock()

		n.leaderHeartbeat()

		deadline := time.Now().Add(1500 * time.Millisecond)
		for time.Now().Before(deadline) {
			n.mu.Lock()
			if n.commitIndex >= targetIndex {
				idx := targetIndex
				n.mu.Unlock()
				return map[string]any{"index": idx}, nil
			}
			if n.role != "leader" {
				lid := n.leaderID
				if lid == "" {
					lid = "0"
				}
				n.mu.Unlock()
				return nil, newRaftError("NOT_LEADER", "not the leader", map[string]any{"leader_id": lid})
			}
			n.mu.Unlock()
			time.Sleep(10 * time.Millisecond)
			n.leaderHeartbeat()
		}
		return nil, newRaftError("NOT_COMMITTED", "could not replicate to a quorum", nil)

	case "get":
		n.mu.Lock()
		defer n.mu.Unlock()
		n.applyCommitted()
		key := strParam(p, "key")
		if val, ok := n.kv[key]; ok {
			return map[string]any{"found": true, "value": val}, nil
		}
		return map[string]any{"found": false}, nil

	case "request_vote":
		n.mu.Lock()
		defer n.mu.Unlock()

		term := intParam(p, "term")
		candidateID := strParam(p, "candidate_id")
		lastLogIndex := intParam(p, "last_log_index")
		lastLogTerm := intParam(p, "last_log_term")

		if term < n.currentTerm {
			return map[string]any{"term": n.currentTerm, "vote_granted": false}, nil
		}
		if term > n.currentTerm {
			n.stepDown(term)
		}

		upToDate := lastLogTerm > n.lastLogTerm() ||
			(lastLogTerm == n.lastLogTerm() && lastLogIndex >= n.lastLogIndex())

		voteGranted := false
		if upToDate && (n.votedFor == nil || *n.votedFor == candidateID) {
			n.votedFor = &candidateID
			voteGranted = true
			n.persist()
		}
		n.resetElectionTimer()
		return map[string]any{"term": n.currentTerm, "vote_granted": voteGranted}, nil

	case "append_entries":
		n.mu.Lock()
		defer n.mu.Unlock()

		term := intParam(p, "term")
		leaderID := strParam(p, "leader_id")

		if term < n.currentTerm {
			return map[string]any{"term": n.currentTerm, "success": false}, nil
		}
		if term > n.currentTerm {
			n.stepDown(term)
		}

		n.role = "follower"
		n.leaderID = leaderID
		n.resetElectionTimer()

		prevLogIndex := intParam(p, "prev_log_index")
		prevLogTerm := intParam(p, "prev_log_term")

		if prevLogIndex > 0 {
			if prevLogIndex > n.lastLogIndex() ||
				n.log[prevLogIndex-1].Term != prevLogTerm {
				return map[string]any{"term": n.currentTerm, "success": false}, nil
			}
		}

		var entries []logEntry
		if raw, ok := p["entries"]; ok && raw != nil {
			b, _ := json.Marshal(raw)
			json.Unmarshal(b, &entries)
		}
		if len(entries) > 0 {
			n.log = n.log[:prevLogIndex]
			n.log = append(n.log, entries...)
			n.persist()
		}

		leaderCommit := intParam(p, "leader_commit")
		if leaderCommit > n.commitIndex {
			n.commitIndex = leaderCommit
			if n.commitIndex > n.lastLogIndex() {
				n.commitIndex = n.lastLogIndex()
			}
			n.applyCommitted()
		}
		return map[string]any{"term": n.currentTerm, "success": true}, nil

	default:
		return nil, newRaftError("UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method), nil)
	}
}

var methods = map[string]struct{}{
	"ping": {}, "get_status": {}, "set": {}, "get": {},
	"request_vote": {}, "append_entries": {},
}

func handleConn(conn net.Conn, node *raftNode) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(conn)
	for scanner.Scan() {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			encoder.Encode(map[string]any{
				"id":    nil,
				"error": map[string]any{"code": "BAD_REQUEST", "message": err.Error()},
			})
			continue
		}
		if _, ok := methods[req.Method]; !ok {
			errBody := map[string]any{"code": "UNKNOWN_METHOD", "message": fmt.Sprintf("unknown method %q", req.Method)}
			encoder.Encode(map[string]any{"id": req.ID, "error": errBody})
			continue
		}
		p := paramsMap(req.Params)
		result, rerr := node.handle(req.Method, p)
		if rerr != nil {
			errBody := map[string]any{"code": rerr.code, "message": rerr.message}
			for k, v := range rerr.extra {
				errBody[k] = v
			}
			encoder.Encode(map[string]any{"id": req.ID, "error": errBody})
		} else {
			encoder.Encode(map[string]any{"id": req.ID, "result": result})
		}
	}
}

func main() {
	nodeID := flag.String("node-id", "", "this node's id")
	peersStr := flag.String("peers", "", "peer list")
	port := flag.Int("port", 0, "port to listen on")
	dataDir := flag.String("data-dir", "", "persistent state directory")
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	peers := parsePeers(*peersStr)
	node, err := newRaftNode(*nodeID, peers, *dataDir)
	if err != nil {
		log.Fatalf("recovery failed: %v", err)
	}
	go node.runRaftLoop()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("raft node %s listening on %s", *nodeID, ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn, node)
	}
}
