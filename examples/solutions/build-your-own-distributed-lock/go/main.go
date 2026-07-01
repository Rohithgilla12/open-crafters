// Reference solution for "Build your own distributed lock" (Go). Passes all 9 stages.
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

type dlError struct{ code, message string }

func (e *dlError) Error() string { return e.code + ": " + e.message }

func nowMS() int64 { return time.Now().UnixMilli() }

func newToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

type lockState struct {
	HolderID    string `json:"holder_id"`
	Token       string `json:"token"`
	ExpiresAtMS int64  `json:"expires_at_ms"`
}

type engine struct {
	mu        sync.Mutex
	statePath string
	locks     map[string]*lockState
}

func newEngine(dataDir string) *engine {
	e := &engine{statePath: filepath.Join(dataDir, "state.json"), locks: map[string]*lockState{}}
	e.load()
	return e
}

func (e *engine) load() {
	data, err := os.ReadFile(e.statePath)
	if err != nil {
		return
	}
	var wrap struct {
		Locks map[string]*lockState `json:"locks"`
	}
	if json.Unmarshal(data, &wrap) == nil && wrap.Locks != nil {
		e.locks = wrap.Locks
	}
}

func (e *engine) persist() {
	data, _ := json.Marshal(struct {
		Locks map[string]*lockState `json:"locks"`
	}{e.locks})
	tmp := e.statePath + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		os.Rename(tmp, e.statePath)
	}
}

func held(l *lockState, now int64) bool {
	return l != nil && l.ExpiresAtMS > now
}

type dlParams struct {
	Name     string  `json:"name"`
	HolderID string  `json:"holder_id"`
	LeaseMS  *int64  `json:"lease_ms"`
	Token    string  `json:"token"`
}

func invalidParams(msg string) error {
	return &dlError{"INVALID_PARAMS", msg}
}

func (e *engine) validateAcquire(p *dlParams) error {
	if p.Name == "" || p.HolderID == "" || p.LeaseMS == nil {
		return invalidParams("acquire requires name, holder_id, lease_ms")
	}
	if *p.LeaseMS < 1 {
		return invalidParams("lease_ms must be >= 1")
	}
	return nil
}

func (e *engine) grant(p *dlParams, now int64) *lockState {
	exp := now + *p.LeaseMS
	st := &lockState{HolderID: p.HolderID, Token: newToken(), ExpiresAtMS: exp}
	e.locks[p.Name] = st
	e.persist()
	return st
}

func (e *engine) acquire(p *dlParams, try bool) (any, error) {
	if err := e.validateAcquire(p); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	now := nowMS()
	cur := e.locks[p.Name]
	if held(cur, now) {
		if try {
			return map[string]any{"acquired": false}, nil
		}
		return nil, &dlError{"LOCK_HELD", fmt.Sprintf("lock %q is held", p.Name)}
	}
	st := e.grant(p, now)
	return map[string]any{"token": st.Token, "expires_at_ms": st.ExpiresAtMS}, nil
}

func (e *engine) tryAcquire(p *dlParams) (any, error) {
	if err := e.validateAcquire(p); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	now := nowMS()
	cur := e.locks[p.Name]
	if held(cur, now) {
		return map[string]any{"acquired": false}, nil
	}
	st := e.grant(p, now)
	return map[string]any{
		"acquired": true, "token": st.Token, "expires_at_ms": st.ExpiresAtMS,
	}, nil
}

func (e *engine) release(p *dlParams) (any, error) {
	if p.Name == "" || p.Token == "" {
		return nil, invalidParams("release requires name and token")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	now := nowMS()
	cur := e.locks[p.Name]
	if !held(cur, now) || cur.Token != p.Token {
		return map[string]any{"released": false}, nil
	}
	delete(e.locks, p.Name)
	e.persist()
	return map[string]any{"released": true}, nil
}

func (e *engine) renew(p *dlParams) (any, error) {
	if p.Name == "" || p.Token == "" || p.LeaseMS == nil {
		return nil, invalidParams("renew requires name, token, lease_ms")
	}
	if *p.LeaseMS < 1 {
		return nil, invalidParams("lease_ms must be >= 1")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	now := nowMS()
	cur := e.locks[p.Name]
	if !held(cur, now) || cur.Token != p.Token {
		return nil, &dlError{"NOT_HOLDER", "token does not match current holder"}
	}
	base := cur.ExpiresAtMS
	if now > base {
		base = now
	}
	cur.ExpiresAtMS = base + *p.LeaseMS
	e.persist()
	return map[string]any{"expires_at_ms": cur.ExpiresAtMS}, nil
}

func (e *engine) status(p *dlParams) (any, error) {
	if p.Name == "" {
		return nil, invalidParams("status requires name")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	now := nowMS()
	cur := e.locks[p.Name]
	if !held(cur, now) {
		return map[string]any{"held": false}, nil
	}
	return map[string]any{
		"held": true, "holder_id": cur.HolderID,
		"expires_at_ms": cur.ExpiresAtMS, "token": cur.Token,
	}, nil
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	if method == "ping" {
		return map[string]string{"message": "pong"}, nil
	}
	var p dlParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, invalidParams("params is not valid JSON")
		}
	}
	switch method {
	case "acquire":
		return e.acquire(&p, false)
	case "try_acquire":
		return e.tryAcquire(&p)
	case "release":
		return e.release(&p)
	case "renew":
		return e.renew(&p)
	case "status":
		return e.status(&p)
	}
	return nil, &dlError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
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
					re := err.(*dlError)
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
