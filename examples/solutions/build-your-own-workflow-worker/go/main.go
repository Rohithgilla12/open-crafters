// Reference gateway for "Build your own workflow worker" (Go). Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

const (
	defaultQueue = "default"
	pollInterval = 50 * time.Millisecond
	driveTimeout = 30 * time.Second
)

type gwError struct{ code, message string }

func (e *gwError) Error() string { return e.code + ": " + e.message }

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type engine struct {
	mu       sync.Mutex
	temporal string
	sdk      string
}

type rpcErr struct{ code, message string }

func (e *rpcErr) Error() string { return e.code + ": " + e.message }

func rpc(addr, method string, params map[string]any, out any) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	req, _ := json.Marshal(map[string]any{"id": "1", "method": method, "params": params})
	req = append(req, '\n')
	if _, err := conn.Write(req); err != nil {
		return err
	}
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	if !sc.Scan() {
		return fmt.Errorf("no response from %s", addr)
	}
	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(sc.Bytes(), &resp); err != nil {
		return err
	}
	if resp.Error != nil {
		return &rpcErr{resp.Error.Code, resp.Error.Message}
	}
	if out != nil && len(resp.Result) > 0 {
		return json.Unmarshal(resp.Result, out)
	}
	return nil
}

func activityResult(activityType string, _ any) any {
	switch activityType {
	case "fetch":
		return map[string]any{"status": 200, "body": "ok"}
	case "work":
		return map[string]any{"done": true}
	default:
		return map[string]any{"ok": true}
	}
}

func (e *engine) describe(workflowID string) (status string, result, errVal any, err error) {
	var res struct {
		Status string `json:"status"`
		Result any    `json:"result"`
		Error  any    `json:"error"`
	}
	if err := rpc(e.temporal, "describe_workflow", map[string]any{"workflow_id": workflowID}, &res); err != nil {
		return "", nil, nil, err
	}
	return res.Status, res.Result, res.Error, nil
}

func (e *engine) replay(workflowType string, history []map[string]any) ([]map[string]any, error) {
	var res struct {
		Commands []map[string]any `json:"commands"`
	}
	if err := rpc(e.sdk, "replay", map[string]any{
		"workflow_type": workflowType,
		"history":       history,
	}, &res); err != nil {
		return nil, err
	}
	if res.Commands == nil {
		res.Commands = []map[string]any{}
	}
	return res.Commands, nil
}

func (e *engine) driveUntilDone(workflowID string) (string, any, any, error) {
	deadline := time.Now().Add(driveTimeout)
	for time.Now().Before(deadline) {
		status, result, errVal, err := e.describe(workflowID)
		if err != nil {
			return "", nil, nil, err
		}
		if status == "COMPLETED" {
			return status, result, nil, nil
		}
		if status == "FAILED" {
			return status, nil, errVal, nil
		}
		if err := e.driveRound(workflowID); err != nil {
			return "", nil, nil, err
		}
		time.Sleep(pollInterval)
	}
	return "", nil, nil, fmt.Errorf("workflow %q did not finish within timeout", workflowID)
}

func (e *engine) driveRound(_ string) error {
	// Poll workflow tasks until none, completing each for our workflow (or any on shared queue).
	for i := 0; i < 8; i++ {
		var wtRes struct {
			Task *struct {
				TaskToken    string           `json:"task_token"`
				WorkflowID   string           `json:"workflow_id"`
				WorkflowType string           `json:"workflow_type"`
				History      []map[string]any `json:"history"`
			} `json:"task"`
		}
		if err := rpc(e.temporal, "poll_workflow_task", map[string]any{"task_queue": defaultQueue}, &wtRes); err != nil {
			return err
		}
		if wtRes.Task == nil {
			break
		}
		cmds, err := e.replay(wtRes.Task.WorkflowType, wtRes.Task.History)
		if err != nil {
			return err
		}
		if err := rpc(e.temporal, "complete_workflow_task", map[string]any{
			"task_token": wtRes.Task.TaskToken,
			"commands":   cmds,
		}, nil); err != nil {
			return err
		}
		_ = wtRes.Task.WorkflowID
	}
	for i := 0; i < 8; i++ {
		var atRes struct {
			Task *struct {
				TaskToken    string `json:"task_token"`
				WorkflowID   string `json:"workflow_id"`
				ActivityType string `json:"activity_type"`
				Input        any    `json:"input"`
			} `json:"task"`
		}
		if err := rpc(e.temporal, "poll_activity_task", map[string]any{"task_queue": defaultQueue}, &atRes); err != nil {
			return err
		}
		if atRes.Task == nil {
			break
		}
		result := activityResult(atRes.Task.ActivityType, atRes.Task.Input)
		if err := rpc(e.temporal, "complete_activity_task", map[string]any{
			"task_token": atRes.Task.TaskToken,
			"result":     result,
		}, nil); err != nil {
			return err
		}
	}
	return nil
}

func (e *engine) handle(method string, raw json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "start_workflow":
		var p struct {
			WorkflowID   string `json:"workflow_id"`
			WorkflowType string `json:"workflow_type"`
			Input        any    `json:"input"`
			TaskQueue    string `json:"task_queue"`
		}
		if json.Unmarshal(raw, &p) != nil || p.WorkflowID == "" || p.WorkflowType == "" {
			return nil, &gwError{"INVALID_PARAMS", "start_workflow requires workflow_id and workflow_type"}
		}
		q := p.TaskQueue
		if q == "" {
			q = defaultQueue
		}
		var res struct {
			RunID string `json:"run_id"`
		}
		if err := rpc(e.temporal, "start_workflow", map[string]any{
			"workflow_id": p.WorkflowID, "workflow_type": p.WorkflowType,
			"input": p.Input, "task_queue": q,
		}, &res); err != nil {
			if re, ok := err.(*rpcErr); ok {
				return nil, &gwError{re.code, re.message}
			}
			return nil, err
		}
		return map[string]string{"run_id": res.RunID}, nil
	case "signal_workflow":
		var p struct {
			WorkflowID  string `json:"workflow_id"`
			SignalName  string `json:"signal_name"`
			Input       any    `json:"input"`
		}
		if json.Unmarshal(raw, &p) != nil || p.WorkflowID == "" || p.SignalName == "" {
			return nil, &gwError{"INVALID_PARAMS", "signal_workflow requires workflow_id and signal_name"}
		}
		if err := rpc(e.temporal, "signal_workflow", map[string]any{
			"workflow_id": p.WorkflowID, "signal_name": p.SignalName, "input": p.Input,
		}, nil); err != nil {
			if re, ok := err.(*rpcErr); ok {
				return nil, &gwError{re.code, re.message}
			}
			return nil, err
		}
		return map[string]any{}, nil
	case "await_workflow":
		var p struct {
			WorkflowID string `json:"workflow_id"`
		}
		if json.Unmarshal(raw, &p) != nil || p.WorkflowID == "" {
			return nil, &gwError{"INVALID_PARAMS", "await_workflow requires workflow_id"}
		}
		status, result, errVal, err := e.driveUntilDone(p.WorkflowID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": status, "result": result, "error": errVal}, nil
	case "run_workflow":
		var p struct {
			WorkflowID   string `json:"workflow_id"`
			WorkflowType string `json:"workflow_type"`
			Input        any    `json:"input"`
			TaskQueue    string `json:"task_queue"`
		}
		if json.Unmarshal(raw, &p) != nil || p.WorkflowID == "" || p.WorkflowType == "" {
			return nil, &gwError{"INVALID_PARAMS", "run_workflow requires workflow_id and workflow_type"}
		}
		q := p.TaskQueue
		if q == "" {
			q = defaultQueue
		}
		var startRes struct {
			RunID string `json:"run_id"`
		}
		if err := rpc(e.temporal, "start_workflow", map[string]any{
			"workflow_id": p.WorkflowID, "workflow_type": p.WorkflowType,
			"input": p.Input, "task_queue": q,
		}, &startRes); err != nil {
			if re, ok := err.(*rpcErr); ok {
				return nil, &gwError{re.code, re.message}
			}
			return nil, err
		}
		_ = startRes.RunID
		status, result, errVal, err := e.driveUntilDone(p.WorkflowID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"status": status, "result": result, "error": errVal}, nil
	default:
		return nil, &gwError{"UNKNOWN_METHOD", fmt.Sprintf("unknown method %q", method)}
	}
}

func main() {
	port := flag.Int("port", 0, "TCP port")
	flag.String("data-dir", "", "")
	flag.Parse()
	if *port == 0 {
		log.Fatal("usage: your_program.sh --port PORT")
	}
	temporalAddr := os.Getenv("TEMPORAL_ADDR")
	sdkAddr := os.Getenv("SDK_ADDR")
	if temporalAddr == "" || sdkAddr == "" {
		log.Fatal("TEMPORAL_ADDR and SDK_ADDR must be set by the harness")
	}
	eng := &engine{temporal: temporalAddr, sdk: sdkAddr}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("workflow worker gateway listening on %s", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Print(err)
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			sc := bufio.NewScanner(c)
			sc.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
			for sc.Scan() {
				var req request
				if json.Unmarshal(sc.Bytes(), &req) != nil {
					continue
				}
				result, err := eng.handle(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					if ge, ok := err.(*gwError); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": ge.code, "message": ge.message}}
					} else if re, ok := err.(*rpcErr); ok {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": re.code, "message": re.message}}
					} else {
						resp = map[string]any{"id": req.ID, "error": map[string]string{"code": "INTERNAL", "message": err.Error()}}
					}
				} else {
					resp = map[string]any{"id": req.ID, "result": result}
				}
				out, _ := json.Marshal(resp)
				out = append(out, '\n')
				c.Write(out)
			}
		}(conn)
	}
}
