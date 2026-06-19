// Reference solution for "Build your own Temporal" (Go).
//
// A workflow engine: append-only event histories, workflow/activity task
// dispatch over a non-blocking poll protocol, activity retries with
// exponential backoff, durable timers, signals, and crash-safe persistence
// (atomic JSON snapshot per state change). One mutex, a ticker goroutine for
// timers, full-snapshot persistence. Passes all 10 stages.
package main

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
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

type engineError struct {
	code, message string
}

func (e *engineError) Error() string { return e.code + ": " + e.message }
func errf(code, format string, a ...any) *engineError {
	return &engineError{code, fmt.Sprintf(format, a...)}
}

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type event struct {
	EventID    int            `json:"event_id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes"`
}

type pendingActivity struct {
	ActivityType       string  `json:"activity_type"`
	Input              any     `json:"input"`
	Attempt            int     `json:"attempt"`
	MaximumAttempts    int     `json:"maximum_attempts"`
	InitialIntervalMs  float64 `json:"initial_interval_ms"`
	BackoffCoefficient float64 `json:"backoff_coefficient"`
	AvailableAt        float64 `json:"available_at"`
	claimed            bool    // runtime only; not persisted
}

type workflow struct {
	WorkflowID        string                      `json:"workflow_id"`
	RunID             string                      `json:"run_id"`
	WorkflowType      string                      `json:"workflow_type"`
	TaskQueue         string                      `json:"task_queue"`
	Status            string                      `json:"status"`
	Result            any                         `json:"result"`
	Error             any                         `json:"error"`
	History           []event                     `json:"history"`
	NeedsWFT          bool                        `json:"needs_wft"`
	PendingActivities map[string]*pendingActivity `json:"pending_activities"`
	Timers            map[string]float64          `json:"timers"`
	wftToken          string                      // runtime only; not persisted
}

type snapshot struct {
	Workflows []*workflow `json:"workflows"`
}

type engine struct {
	mu         sync.Mutex
	order      []string // workflow ids in insertion order
	workflows  map[string]*workflow
	wftClaims  map[string]string   // token -> workflow id
	actClaims  map[string][]string // token -> [workflow id, activity id]
	statePath  string
}

func newEngine(dataDir string) (*engine, error) {
	e := &engine{
		workflows: map[string]*workflow{},
		wftClaims: map[string]string{},
		actClaims: map[string][]string{},
		statePath: filepath.Join(dataDir, "state.json"),
	}
	if err := e.load(); err != nil {
		return nil, err
	}
	return e, nil
}

func nowSec() float64 { return float64(time.Now().UnixNano()) / 1e9 }
func token() string {
	var b [16]byte
	rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (e *engine) load() error {
	data, err := os.ReadFile(e.statePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var snap snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	for _, wf := range snap.Workflows {
		wf.wftToken = "" // claims are forgotten on restart so tasks re-deliver
		if wf.PendingActivities == nil {
			wf.PendingActivities = map[string]*pendingActivity{}
		}
		if wf.Timers == nil {
			wf.Timers = map[string]float64{}
		}
		e.workflows[wf.WorkflowID] = wf
		e.order = append(e.order, wf.WorkflowID)
	}
	return nil
}

func (e *engine) persist() {
	snap := snapshot{}
	for _, id := range e.order {
		wf := e.workflows[id]
		// A claimed-but-incomplete workflow task must be re-delivered after a
		// crash, so persist the claim as "needs a task".
		cp := *wf
		cp.NeedsWFT = wf.NeedsWFT || wf.wftToken != ""
		snap.Workflows = append(snap.Workflows, &cp)
	}
	body, _ := json.Marshal(snap)
	tmp := e.statePath + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return
	}
	f.Write(body)
	f.Sync()
	f.Close()
	os.Rename(tmp, e.statePath)
}

func (e *engine) get(id string) (*workflow, *engineError) {
	wf := e.workflows[id]
	if wf == nil {
		return nil, errf("WORKFLOW_NOT_FOUND", "no workflow with id %q", id)
	}
	return wf, nil
}

func (wf *workflow) append(eventType string, attrs map[string]any) {
	wf.History = append(wf.History, event{EventID: len(wf.History) + 1, Type: eventType, Attributes: attrs})
}

func str(p map[string]any, k string) string  { s, _ := p[k].(string); return s }
func num(p map[string]any, k string, def float64) float64 {
	if v, ok := p[k].(float64); ok {
		return v
	}
	return def
}

func (e *engine) handle(method string, p map[string]any) (any, *engineError) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch method {
	case "ping":
		return map[string]any{"message": "pong"}, nil

	case "start_workflow":
		id := str(p, "workflow_id")
		if _, ok := e.workflows[id]; ok {
			return nil, errf("WORKFLOW_ALREADY_EXISTS", "workflow %q already exists", id)
		}
		wf := &workflow{
			WorkflowID: id, RunID: token(), WorkflowType: str(p, "workflow_type"),
			TaskQueue: str(p, "task_queue"), Status: "RUNNING", NeedsWFT: true,
			PendingActivities: map[string]*pendingActivity{}, Timers: map[string]float64{},
		}
		wf.append("WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": str(p, "workflow_type"), "input": p["input"],
		})
		e.workflows[id] = wf
		e.order = append(e.order, id)
		e.persist()
		return map[string]any{"run_id": wf.RunID}, nil

	case "describe_workflow":
		wf, err := e.get(str(p, "workflow_id"))
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"workflow_id": wf.WorkflowID, "run_id": wf.RunID, "workflow_type": wf.WorkflowType,
			"status": wf.Status, "result": wf.Result, "error": wf.Error,
		}, nil

	case "get_history":
		wf, err := e.get(str(p, "workflow_id"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"events": wf.History}, nil

	case "poll_workflow_task":
		queue := str(p, "task_queue")
		for _, id := range e.order {
			wf := e.workflows[id]
			if wf.TaskQueue == queue && wf.Status == "RUNNING" && wf.NeedsWFT && wf.wftToken == "" {
				tok := token()
				wf.NeedsWFT = false
				wf.wftToken = tok
				e.wftClaims[tok] = id
				return map[string]any{"task": map[string]any{
					"task_token": tok, "workflow_id": id, "run_id": wf.RunID,
					"workflow_type": wf.WorkflowType, "history": wf.History,
				}}, nil
			}
		}
		return map[string]any{"task": nil}, nil

	case "complete_workflow_task":
		tok := str(p, "task_token")
		id, ok := e.wftClaims[tok]
		if !ok {
			return nil, errf("TASK_NOT_FOUND", "no claimed workflow task with token %q", tok)
		}
		delete(e.wftClaims, tok)
		wf := e.workflows[id]
		wf.wftToken = ""
		if cmds, ok := p["commands"].([]any); ok {
			for _, c := range cmds {
				if cm, ok := c.(map[string]any); ok {
					if err := e.applyCommand(wf, cm); err != nil {
						return nil, err
					}
				}
			}
		}
		if wf.Status != "RUNNING" {
			wf.NeedsWFT = false
		}
		e.persist()
		return map[string]any{}, nil

	case "poll_activity_task":
		queue := str(p, "task_queue")
		now := nowSec()
		for _, id := range e.order {
			wf := e.workflows[id]
			if wf.TaskQueue != queue || wf.Status != "RUNNING" {
				continue
			}
			for aid, act := range wf.PendingActivities {
				if act.claimed || act.AvailableAt > now {
					continue
				}
				tok := token()
				act.claimed = true
				e.actClaims[tok] = []string{id, aid}
				return map[string]any{"task": map[string]any{
					"task_token": tok, "workflow_id": id, "run_id": wf.RunID,
					"activity_id": aid, "activity_type": act.ActivityType,
					"input": act.Input, "attempt": act.Attempt,
				}}, nil
			}
		}
		return map[string]any{"task": nil}, nil

	case "complete_activity_task":
		wf, aid, err := e.takeActivityClaim(str(p, "task_token"))
		if err != nil {
			return nil, err
		}
		delete(wf.PendingActivities, aid)
		wf.append("ACTIVITY_TASK_COMPLETED", map[string]any{"activity_id": aid, "result": p["result"]})
		wf.NeedsWFT = true
		e.persist()
		return map[string]any{}, nil

	case "fail_activity_task":
		wf, aid, err := e.takeActivityClaim(str(p, "task_token"))
		if err != nil {
			return nil, err
		}
		act := wf.PendingActivities[aid]
		if act.Attempt < act.MaximumAttempts {
			delayMs := act.InitialIntervalMs * math.Pow(act.BackoffCoefficient, float64(act.Attempt-1))
			act.Attempt++
			act.AvailableAt = nowSec() + delayMs/1000.0
			act.claimed = false
		} else {
			delete(wf.PendingActivities, aid)
			wf.append("ACTIVITY_TASK_FAILED", map[string]any{"activity_id": aid, "error": p["error"]})
			wf.NeedsWFT = true
		}
		e.persist()
		return map[string]any{}, nil

	case "signal_workflow":
		wf, err := e.get(str(p, "workflow_id"))
		if err != nil {
			return nil, err
		}
		if wf.Status != "RUNNING" {
			return nil, errf("WORKFLOW_CLOSED", "workflow %q is %s", wf.WorkflowID, wf.Status)
		}
		wf.append("WORKFLOW_EXECUTION_SIGNALED", map[string]any{
			"signal_name": str(p, "signal_name"), "input": p["input"],
		})
		wf.NeedsWFT = true
		e.persist()
		return map[string]any{}, nil

	default:
		return nil, errf("UNKNOWN_METHOD", "unknown method %q", method)
	}
}

func (e *engine) applyCommand(wf *workflow, command map[string]any) *engineError {
	ctype, _ := command["type"].(string)
	attrs, _ := command["attributes"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
	}
	switch ctype {
	case "SCHEDULE_ACTIVITY":
		aid := str(attrs, "activity_id")
		wf.append("ACTIVITY_TASK_SCHEDULED", map[string]any{
			"activity_id": aid, "activity_type": str(attrs, "activity_type"), "input": attrs["input"],
		})
		policy, _ := attrs["retry_policy"].(map[string]any)
		if policy == nil {
			policy = map[string]any{}
		}
		wf.PendingActivities[aid] = &pendingActivity{
			ActivityType:       str(attrs, "activity_type"),
			Input:              attrs["input"],
			Attempt:            1,
			MaximumAttempts:    int(num(policy, "maximum_attempts", 1)),
			InitialIntervalMs:  num(policy, "initial_interval_ms", 1000),
			BackoffCoefficient: num(policy, "backoff_coefficient", 2.0),
			AvailableAt:        nowSec(),
		}
	case "START_TIMER":
		wf.append("TIMER_STARTED", map[string]any{
			"timer_id": str(attrs, "timer_id"), "duration_ms": attrs["duration_ms"],
		})
		wf.Timers[str(attrs, "timer_id")] = nowSec() + num(attrs, "duration_ms", 0)/1000.0
	case "COMPLETE_WORKFLOW":
		wf.append("WORKFLOW_EXECUTION_COMPLETED", map[string]any{"result": attrs["result"]})
		wf.Status = "COMPLETED"
		wf.Result = attrs["result"]
	case "FAIL_WORKFLOW":
		wf.append("WORKFLOW_EXECUTION_FAILED", map[string]any{"error": attrs["error"]})
		wf.Status = "FAILED"
		wf.Error = attrs["error"]
	default:
		return errf("UNKNOWN_COMMAND", "unknown command type %q", ctype)
	}
	return nil
}

func (e *engine) takeActivityClaim(tok string) (*workflow, string, *engineError) {
	claim, ok := e.actClaims[tok]
	if !ok {
		return nil, "", errf("TASK_NOT_FOUND", "no claimed activity task with token %q", tok)
	}
	delete(e.actClaims, tok)
	return e.workflows[claim[0]], claim[1], nil
}

func (e *engine) timerLoop() {
	for {
		time.Sleep(50 * time.Millisecond)
		e.mu.Lock()
		now := nowSec()
		fired := false
		for _, id := range e.order {
			wf := e.workflows[id]
			if wf.Status != "RUNNING" {
				continue
			}
			for tid, fireAt := range wf.Timers {
				if fireAt <= now {
					delete(wf.Timers, tid)
					wf.append("TIMER_FIRED", map[string]any{"timer_id": tid})
					wf.NeedsWFT = true
					fired = true
				}
			}
		}
		if fired {
			e.persist()
		}
		e.mu.Unlock()
	}
}

func handleConn(conn net.Conn, e *engine) {
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
			encoder.Encode(map[string]any{"id": nil, "error": map[string]any{"code": "BAD_REQUEST", "message": err.Error()}})
			continue
		}
		var p map[string]any
		if len(req.Params) > 0 {
			json.Unmarshal(req.Params, &p)
		}
		if p == nil {
			p = map[string]any{}
		}
		result, eerr := e.handle(req.Method, p)
		if eerr != nil {
			encoder.Encode(map[string]any{"id": req.ID, "error": map[string]any{"code": eerr.code, "message": eerr.message}})
		} else {
			encoder.Encode(map[string]any{"id": req.ID, "result": result})
		}
	}
}

func main() {
	port := flag.Int("port", 0, "port to listen on")
	dataDir := flag.String("data-dir", "", "directory for durable state")
	flag.Parse()

	e, err := newEngine(*dataDir)
	if err != nil {
		log.Fatalf("recovery failed: %v", err)
	}
	go e.timerLoop()

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("workflow engine listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go handleConn(conn, e)
	}
}
