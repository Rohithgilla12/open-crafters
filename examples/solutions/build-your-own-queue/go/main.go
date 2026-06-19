// Reference solution for "Build your own message queue" (Go).
//
// A durable broker with at-least-once delivery: visibility timeouts, nack,
// receipt fencing (a receipt is valid for one delivery), and dead-letter
// queues after max_receives. Un-acked messages survive SIGKILL; acked ones
// stay gone (state snapshotted atomically). Passes all 9 stages.
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
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

type message struct {
	id, body       string
	seq            int
	receives       int
	inflight       bool
	invisibleUntil time.Time
	receipt        string
}

type queue struct {
	messages    map[string]*message
	maxReceives int    // -1 = no policy
	dlq         string // dead-letter queue name
}

func newQueue() *queue { return &queue{messages: map[string]*message{}, maxReceives: -1} }

// --- persistence model ---

type persistMsg struct {
	ID       string `json:"id"`
	Body     string `json:"body"`
	Seq      int    `json:"seq"`
	Receives int    `json:"receives"`
}
type persistQueue struct {
	MaxReceives int          `json:"max_receives"`
	DLQ         string       `json:"dead_letter_queue"`
	Messages    []persistMsg `json:"messages"`
}
type persistState struct {
	Seq    int                     `json:"seq"`
	Queues map[string]persistQueue `json:"queues"`
}

type broker struct {
	mu       sync.Mutex
	queues   map[string]*queue
	seq      int
	snapPath string
}

// randHex returns a globally-unique handle (ids/receipts). A counter would
// reset on restart and collide with recovered message ids — losing messages.
func randHex() string {
	var b [12]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func newBroker(dataDir string) (*broker, error) {
	b := &broker{queues: map[string]*queue{}, snapPath: filepath.Join(dataDir, "state.json")}
	if err := b.recover(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *broker) recover() error {
	data, err := os.ReadFile(b.snapPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var st persistState
	if err := json.Unmarshal(data, &st); err != nil {
		return err
	}
	b.seq = st.Seq
	for name, pq := range st.Queues {
		q := newQueue()
		q.maxReceives = pq.MaxReceives
		q.dlq = pq.DLQ
		for _, pm := range pq.Messages {
			// Everything un-acked comes back visible; in-flight state does
			// not survive a crash.
			q.messages[pm.ID] = &message{id: pm.ID, body: pm.Body, seq: pm.Seq, receives: pm.Receives}
		}
		b.queues[name] = q
	}
	return nil
}

func (b *broker) persist() error {
	st := persistState{Seq: b.seq, Queues: map[string]persistQueue{}}
	for name, q := range b.queues {
		pq := persistQueue{MaxReceives: q.maxReceives, DLQ: q.dlq}
		for _, m := range q.messages {
			pq.Messages = append(pq.Messages, persistMsg{ID: m.id, Body: m.body, Seq: m.seq, Receives: m.receives})
		}
		st.Queues[name] = pq
	}
	body, _ := json.Marshal(st)
	tmp := b.snapPath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(body); err != nil {
		f.Close()
		return err
	}
	f.Sync()
	f.Close()
	return os.Rename(tmp, b.snapPath)
}

func (b *broker) queue(name string) *queue {
	q := b.queues[name]
	if q == nil {
		q = newQueue()
		b.queues[name] = q
	}
	return q
}

func (b *broker) nextSeq() int { b.seq++; return b.seq }

// maybeDeadLetter moves m to the DLQ if it has hit the policy; returns true if moved.
func (b *broker) maybeDeadLetter(q *queue, m *message) bool {
	if q.maxReceives < 0 || m.receives < q.maxReceives {
		return false
	}
	delete(q.messages, m.id)
	dlq := b.queue(q.dlq)
	dlq.messages[m.id] = &message{id: m.id, body: m.body, seq: b.nextSeq()}
	return true
}

// expire flips lapsed in-flight messages back to visible (or dead-letters them).
func (b *broker) expire(q *queue, now time.Time) (changed bool) {
	for _, m := range q.messages {
		if m.inflight && !m.invisibleUntil.After(now) {
			if b.maybeDeadLetter(q, m) {
				changed = true
			} else {
				m.inflight = false
				m.receipt = ""
			}
		}
	}
	return changed
}

func (b *broker) findInflight(q *queue, receipt string) *message {
	for _, m := range q.messages {
		if m.inflight && m.receipt == receipt {
			return m
		}
	}
	return nil
}

func (b *broker) handle(req request) (any, *rpcError) {
	var p map[string]any
	if len(req.Params) > 0 {
		json.Unmarshal(req.Params, &p)
	}
	str := func(k string) string { s, _ := p[k].(string); return s }

	b.mu.Lock()
	defer b.mu.Unlock()

	switch req.Method {
	case "ping":
		return map[string]any{"message": "pong"}, nil

	case "send":
		q := b.queue(str("queue"))
		id := randHex()
		q.messages[id] = &message{id: id, body: str("body"), seq: b.nextSeq()}
		if err := b.persist(); err != nil {
			return nil, &rpcError{"IO_ERROR", err.Error()}
		}
		return map[string]any{"id": id}, nil

	case "receive":
		timeoutMs := 30000
		if v, ok := p["visibility_timeout_ms"].(float64); ok {
			timeoutMs = int(v)
		}
		q := b.queue(str("queue"))
		now := time.Now()
		if b.expire(q, now) {
			b.persist()
		}
		var pick *message
		for _, m := range q.messages {
			if !m.inflight && (pick == nil || m.seq < pick.seq) {
				pick = m
			}
		}
		if pick == nil {
			return map[string]any{"message": nil}, nil
		}
		pick.receives++
		pick.inflight = true
		pick.invisibleUntil = now.Add(time.Duration(timeoutMs) * time.Millisecond)
		pick.receipt = randHex()
		return map[string]any{"message": map[string]any{
			"id": pick.id, "body": pick.body, "receipt": pick.receipt, "receives": pick.receives,
		}}, nil

	case "ack":
		q := b.queue(str("queue"))
		m := b.findInflight(q, str("receipt"))
		if m == nil {
			return map[string]any{"acked": false}, nil
		}
		delete(q.messages, m.id)
		if err := b.persist(); err != nil {
			return nil, &rpcError{"IO_ERROR", err.Error()}
		}
		return map[string]any{"acked": true}, nil

	case "nack":
		q := b.queue(str("queue"))
		m := b.findInflight(q, str("receipt"))
		if m == nil {
			return map[string]any{"nacked": false}, nil
		}
		if b.maybeDeadLetter(q, m) {
			b.persist()
		} else {
			m.inflight = false
			m.invisibleUntil = time.Time{}
			m.receipt = ""
		}
		return map[string]any{"nacked": true}, nil

	case "stats":
		q := b.queues[str("queue")]
		if q == nil {
			return map[string]any{"visible": 0, "inflight": 0}, nil
		}
		now := time.Now()
		if b.expire(q, now) {
			b.persist()
		}
		visible, inflight := 0, 0
		for _, m := range q.messages {
			if m.inflight {
				inflight++
			} else {
				visible++
			}
		}
		return map[string]any{"visible": visible, "inflight": inflight}, nil

	case "configure":
		q := b.queue(str("queue"))
		if v, ok := p["max_receives"].(float64); ok {
			q.maxReceives = int(v)
		}
		q.dlq = str("dead_letter_queue")
		if err := b.persist(); err != nil {
			return nil, &rpcError{"IO_ERROR", err.Error()}
		}
		return map[string]any{}, nil

	default:
		return nil, &rpcError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", req.Method)}
	}
}

func handleConn(conn net.Conn, b *broker) {
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
		result, rpcErr := b.handle(req)
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

	b, err := newBroker(*dataDir)
	if err != nil {
		log.Fatalf("recovery failed: %v", err)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("message broker listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn, b)
	}
}
