// Reference solution for "Build your own scheduler" (Go). Passes all 9 stages.
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

type schedError struct{ code, message string }

func (e *schedError) Error() string { return e.code + ": " + e.message }
func errf(code, msg string, a ...any) *schedError {
	return &schedError{code, fmt.Sprintf(msg, a...)}
}

type job struct {
	JobID            string `json:"job_id"`
	Payload          any    `json:"payload"`
	RunAtMS          int64  `json:"run_at_ms"`
	Status           string `json:"status"`
	Attempt          int    `json:"attempt"`
	LeaseMS          int    `json:"lease_ms"`
	MaxAttempts      int    `json:"max_attempts"`
	RetryDelayMS     int    `json:"retry_delay_ms"`
	IntervalMS       *int   `json:"interval_ms,omitempty"`
	Result           any    `json:"result"`
	Error            any    `json:"error"`
	LeaseToken       string `json:"lease_token,omitempty"`
	LeaseExpiresAtMS int64  `json:"lease_expires_at_ms,omitempty"`
}

type engine struct {
	mu        sync.Mutex
	statePath string
	jobs      map[string]*job
	leases    map[string]string
}

func newEngine(dataDir string) *engine {
	e := &engine{
		statePath: filepath.Join(dataDir, "state.json"),
		jobs:      map[string]*job{},
		leases:    map[string]string{},
	}
	e.load()
	return e
}

func (e *engine) load() {
	data, err := os.ReadFile(e.statePath)
	if err != nil {
		return
	}
	var snap struct {
		Jobs []*job `json:"jobs"`
	}
	if json.Unmarshal(data, &snap) != nil {
		return
	}
	for _, j := range snap.Jobs {
		e.jobs[j.JobID] = j
	}
}

func (e *engine) persist() {
	var jobs []*job
	for _, j := range e.jobs {
		cp := *j
		cp.LeaseToken = ""
		cp.LeaseExpiresAtMS = 0
		jobs = append(jobs, &cp)
	}
	b, _ := json.Marshal(map[string]any{"jobs": jobs})
	tmp := e.statePath + ".tmp"
	os.WriteFile(tmp, b, 0644)
	os.Rename(tmp, e.statePath)
}

func newID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "j-" + hex.EncodeToString(b)
}

func newToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (e *engine) releaseExpired() {
	now := nowMS()
	for _, j := range e.jobs {
		if j.Status == "leased" && j.LeaseExpiresAtMS <= now {
			j.Status = "pending"
			j.LeaseToken = ""
			j.LeaseExpiresAtMS = 0
		}
	}
}

func nowMS() int64 { return time.Now().UnixMilli() }

func pollable(j *job) bool {
	if j.Status == "cancelled" || j.Status == "completed" || j.Status == "failed" {
		return false
	}
	now := nowMS()
	if j.RunAtMS > now {
		return false
	}
	if j.Status == "leased" {
		return j.LeaseExpiresAtMS <= now
	}
	return j.Status == "pending"
}

func (e *engine) handle(method string, params map[string]any) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "schedule":
		payload := params["payload"]
		var runAt int64
		if d, ok := params["delay_ms"].(float64); ok {
			runAt = nowMS() + int64(d)
		} else if r, ok := params["run_at_ms"].(float64); ok {
			runAt = int64(r)
		} else {
			return nil, errf("INVALID_PARAMS", "schedule requires delay_ms or run_at_ms")
		}
		maxAttempts := 1
		retryDelay := 0
		if rp, ok := params["retry_policy"].(map[string]any); ok {
			if v, ok := rp["maximum_attempts"].(float64); ok {
				maxAttempts = int(v)
			}
			if v, ok := rp["retry_delay_ms"].(float64); ok {
				retryDelay = int(v)
			}
		}
		leaseMS := 3000
		if v, ok := params["lease_ms"].(float64); ok {
			leaseMS = int(v)
		}
		var interval *int
		if v, ok := params["interval_ms"].(float64); ok {
			i := int(v)
			interval = &i
		}
		id := newID()
		j := &job{
			JobID: id, Payload: payload, RunAtMS: runAt, Status: "pending",
			Attempt: 1, LeaseMS: leaseMS, MaxAttempts: maxAttempts,
			RetryDelayMS: retryDelay, IntervalMS: interval,
		}
		e.mu.Lock()
		e.jobs[id] = j
		e.persist()
		e.mu.Unlock()
		return map[string]string{"job_id": id}, nil

	case "poll":
		e.mu.Lock()
		defer e.mu.Unlock()
		e.releaseExpired()
		var best *job
		for _, j := range e.jobs {
			if pollable(j) && (best == nil || j.RunAtMS < best.RunAtMS) {
				best = j
			}
		}
		if best == nil {
			return map[string]any{"job": nil}, nil
		}
		tok := newToken()
		best.Status = "leased"
		best.LeaseToken = tok
		best.LeaseExpiresAtMS = nowMS() + int64(best.LeaseMS)
		e.leases[tok] = best.JobID
		return map[string]any{"job": map[string]any{
			"job_id": best.JobID, "payload": best.Payload,
			"attempt": best.Attempt, "lease_token": tok,
		}}, nil

	case "complete":
		tok, _ := params["lease_token"].(string)
		e.mu.Lock()
		defer e.mu.Unlock()
		j, err := e.jobByToken(tok)
		if err != nil {
			return nil, err
		}
		j.Status = "completed"
		j.Result = params["result"]
		j.LeaseToken = ""
		j.LeaseExpiresAtMS = 0
		delete(e.leases, tok)
		if j.IntervalMS != nil {
			e.spawnNext(j, *j.IntervalMS)
		}
		e.persist()
		return map[string]any{}, nil

	case "fail":
		tok, _ := params["lease_token"].(string)
		errMsg, _ := params["error"].(string)
		e.mu.Lock()
		defer e.mu.Unlock()
		j, err := e.jobByToken(tok)
		if err != nil {
			return nil, err
		}
		delete(e.leases, tok)
		j.LeaseToken = ""
		j.LeaseExpiresAtMS = 0
		if j.Attempt < j.MaxAttempts {
			j.Attempt++
			j.Status = "pending"
			j.RunAtMS = nowMS() + int64(j.RetryDelayMS)
		} else {
			j.Status = "failed"
			j.Error = errMsg
		}
		e.persist()
		return map[string]any{}, nil

	case "cancel":
		id, _ := params["job_id"].(string)
		e.mu.Lock()
		defer e.mu.Unlock()
		j := e.jobs[id]
		if j == nil {
			return nil, errf("JOB_NOT_FOUND", "no job %q", id)
		}
		if j.Status != "pending" {
			return map[string]bool{"cancelled": false}, nil
		}
		j.Status = "cancelled"
		e.persist()
		return map[string]bool{"cancelled": true}, nil

	case "get_job":
		id, _ := params["job_id"].(string)
		e.mu.Lock()
		defer e.mu.Unlock()
		e.releaseExpired()
		j := e.jobs[id]
		if j == nil {
			return nil, errf("JOB_NOT_FOUND", "no job %q", id)
		}
		return map[string]any{
			"job_id": j.JobID, "status": j.Status, "payload": j.Payload,
			"run_at_ms": j.RunAtMS, "attempt": j.Attempt,
			"result": j.Result, "error": j.Error,
		}, nil
	default:
		return nil, errf("UNKNOWN_METHOD", "unknown method %q", method)
	}
}

func (e *engine) jobByToken(tok string) (*job, error) {
	id := e.leases[tok]
	if id == "" {
		return nil, errf("LEASE_NOT_FOUND", "unknown lease token")
	}
	j := e.jobs[id]
	if j == nil || j.LeaseToken != tok || j.LeaseExpiresAtMS <= nowMS() {
		return nil, errf("LEASE_NOT_FOUND", "lease expired or invalid")
	}
	return j, nil
}

func (e *engine) spawnNext(parent *job, interval int) {
	id := newID()
	e.jobs[id] = &job{
		JobID: id, Payload: parent.Payload, RunAtMS: nowMS() + int64(interval),
		Status: "pending", Attempt: 1, LeaseMS: parent.LeaseMS,
		MaxAttempts: parent.MaxAttempts, RetryDelayMS: parent.RetryDelayMS,
		IntervalMS: parent.IntervalMS,
	}
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
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go func(c net.Conn) {
			defer c.Close()
			sc := bufio.NewScanner(c)
			w := bufio.NewWriter(c)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				var params map[string]any
				json.Unmarshal(req.Params, &params)
				res, err := eng.handle(req.Method, params)
				var resp map[string]any
				if err != nil {
					if se, ok := err.(*schedError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": se.code, "message": se.message}}
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
