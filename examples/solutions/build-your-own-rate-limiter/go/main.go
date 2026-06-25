// Reference solution for "Build your own rate limiter" (Go). Passes all 9 stages.
//
// Keyed limiters with three algorithms (fixed window, token bucket, sliding
// window), atomic admission under concurrency, and crash-durable state.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type rlError struct{ code, message string }

func (e *rlError) Error() string { return e.code + ": " + e.message }

func nowMS() int64 { return time.Now().UnixMilli() }

type limiter struct {
	Algorithm        string     `json:"algorithm"`
	Capacity         int        `json:"capacity"`
	RefillTokens     int        `json:"refill_tokens"`
	RefillIntervalMS int64      `json:"refill_interval_ms"`
	Tokens           float64    `json:"tokens"`
	AsOfMS           int64      `json:"as_of_ms"`
	Limit            int        `json:"limit"`
	WindowMS         int64      `json:"window_ms"`
	WindowIndex      int64      `json:"window_index"`
	Count            int        `json:"count"`
	Log              [][2]int64 `json:"log"` // [timestamp_ms, cost], oldest first
}

type engine struct {
	mu        sync.Mutex
	statePath string
	limiters  map[string]*limiter
}

func newEngine(dataDir string) *engine {
	e := &engine{statePath: filepath.Join(dataDir, "state.json"), limiters: map[string]*limiter{}}
	e.load()
	return e
}

func (e *engine) load() {
	data, err := os.ReadFile(e.statePath)
	if err != nil {
		return
	}
	var wrap struct {
		Limiters map[string]*limiter `json:"limiters"`
	}
	if json.Unmarshal(data, &wrap) == nil && wrap.Limiters != nil {
		e.limiters = wrap.Limiters
	}
}

// persist atomically rewrites the (tiny) state. SIGKILL, not power loss, is the
// threat model, so no fsync is needed — which keeps the hot path fast.
func (e *engine) persist() {
	data, _ := json.Marshal(struct {
		Limiters map[string]*limiter `json:"limiters"`
	}{e.limiters})
	tmp := e.statePath + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, e.statePath)
	}
}

// refill brings a limiter up to `now` and returns currently available units.
func (e *engine) refill(l *limiter, now int64) float64 {
	switch l.Algorithm {
	case "token_bucket":
		elapsed := now - l.AsOfMS
		if elapsed < 0 {
			elapsed = 0
		}
		accrued := float64(elapsed) / float64(l.RefillIntervalMS) * float64(l.RefillTokens)
		l.Tokens = math.Min(float64(l.Capacity), l.Tokens+accrued)
		l.AsOfMS = now
		return l.Tokens
	case "fixed_window":
		idx := now / l.WindowMS
		if idx != l.WindowIndex {
			l.WindowIndex = idx
			l.Count = 0
		}
		return float64(l.Limit - l.Count)
	case "sliding_window":
		cutoff := now - l.WindowMS
		kept := l.Log[:0]
		used := 0
		for _, e := range l.Log {
			if e[0] > cutoff {
				kept = append(kept, e)
				used += int(e[1])
			}
		}
		l.Log = kept
		return float64(l.Limit - used)
	}
	return 0
}

func (l *limiter) limitVal() int {
	if l.Algorithm == "token_bucket" {
		return l.Capacity
	}
	return l.Limit
}

func (e *engine) retryAfter(l *limiter, now int64, cost int, available float64) int64 {
	if available >= float64(cost) {
		return 0
	}
	switch l.Algorithm {
	case "token_bucket":
		deficit := float64(cost) - l.Tokens
		return int64(math.Ceil(deficit / float64(l.RefillTokens) * float64(l.RefillIntervalMS)))
	case "fixed_window":
		return (l.WindowIndex+1)*l.WindowMS - now
	case "sliding_window":
		need := float64(cost) - available
		freed := 0.0
		for _, ent := range l.Log {
			freed += float64(ent[1])
			if freed >= need {
				return ent[0] + l.WindowMS - now
			}
		}
		return l.WindowMS
	}
	return 0
}

func (e *engine) consume(l *limiter, now int64, cost int) {
	switch l.Algorithm {
	case "token_bucket":
		l.Tokens -= float64(cost)
	case "fixed_window":
		l.Count += cost
	case "sliding_window":
		l.Log = append(l.Log, [2]int64{now, int64(cost)})
	}
}

type rlParams struct {
	Key              string `json:"key"`
	Algorithm        string `json:"algorithm"`
	Capacity         *int   `json:"capacity"`
	RefillTokens     *int   `json:"refill_tokens"`
	RefillIntervalMS *int64 `json:"refill_interval_ms"`
	Limit            *int   `json:"limit"`
	WindowMS         *int64 `json:"window_ms"`
	Cost             *int   `json:"cost"`
}

func (p *rlParams) cost() int {
	if p.Cost != nil {
		return *p.Cost
	}
	return 1
}

func (e *engine) get(key string) (*limiter, error) {
	l := e.limiters[key]
	if l == nil {
		return nil, &rlError{"KEY_NOT_FOUND", fmt.Sprintf("no limiter for key %q", key)}
	}
	return l, nil
}

func (e *engine) configure(p *rlParams) (any, error) {
	if p.Key == "" {
		return nil, &rlError{"INVALID_PARAMS", "configure requires key"}
	}
	var l *limiter
	switch p.Algorithm {
	case "token_bucket":
		if p.Capacity == nil || p.RefillTokens == nil || p.RefillIntervalMS == nil {
			return nil, &rlError{"INVALID_PARAMS", "token_bucket requires capacity, refill_tokens, refill_interval_ms"}
		}
		l = &limiter{Algorithm: p.Algorithm, Capacity: *p.Capacity, RefillTokens: *p.RefillTokens,
			RefillIntervalMS: *p.RefillIntervalMS, Tokens: float64(*p.Capacity), AsOfMS: nowMS()}
	case "fixed_window", "sliding_window":
		if p.Limit == nil || p.WindowMS == nil {
			return nil, &rlError{"INVALID_PARAMS", p.Algorithm + " requires limit, window_ms"}
		}
		l = &limiter{Algorithm: p.Algorithm, Limit: *p.Limit, WindowMS: *p.WindowMS}
		if p.Algorithm == "fixed_window" {
			l.WindowIndex = nowMS() / l.WindowMS
		}
	case "":
		return nil, &rlError{"INVALID_PARAMS", "configure requires algorithm"}
	default:
		return nil, &rlError{"INVALID_ALGORITHM", fmt.Sprintf("unknown algorithm %q", p.Algorithm)}
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.limiters[p.Key] = l
	e.persist()
	return map[string]any{}, nil
}

func (e *engine) take(p *rlParams) (any, error) {
	cost := p.cost()
	e.mu.Lock()
	defer e.mu.Unlock()
	l, err := e.get(p.Key)
	if err != nil {
		return nil, err
	}
	now := nowMS()
	available := e.refill(l, now)
	limit := l.limitVal()
	if available >= float64(cost) {
		e.consume(l, now, cost)
		available -= float64(cost)
		e.persist()
		return map[string]any{"allowed": true, "remaining": int(math.Floor(available)), "limit": limit, "retry_after_ms": 0}, nil
	}
	return map[string]any{"allowed": false, "remaining": int(math.Floor(available)), "limit": limit,
		"retry_after_ms": e.retryAfter(l, now, cost, available)}, nil
}

func (e *engine) peek(p *rlParams) (any, error) {
	cost := p.cost()
	e.mu.Lock()
	defer e.mu.Unlock()
	l, err := e.get(p.Key)
	if err != nil {
		return nil, err
	}
	now := nowMS()
	available := e.refill(l, now)
	return map[string]any{"remaining": int(math.Floor(available)), "limit": l.limitVal(),
		"retry_after_ms": e.retryAfter(l, now, cost, available)}, nil
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	var p rlParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, &rlError{"INVALID_PARAMS", "params is not valid JSON"}
		}
	}
	switch method {
	case "configure":
		return e.configure(&p)
	case "take":
		return e.take(&p)
	case "peek":
		return e.peek(&p)
	}
	return nil, &rlError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
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
	if *dataDir != "" {
		os.MkdirAll(*dataDir, 0o755)
	}
	e := newEngine(*dataDir)

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
				res, err := e.handle(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					re := err.(*rlError)
					resp = map[string]any{"id": req.ID, "error": map[string]string{"code": re.code, "message": re.message}}
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
