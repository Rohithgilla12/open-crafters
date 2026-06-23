// Reference solution for "Build your own workflow SDK" (Go).
//
// A deterministic workflow replay engine. Passes all 9 stages.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"slices"
)

var knownTypes = []string{"greet", "fetch", "timer_wait", "signal_wait", "pipeline"}

type engineError struct {
	code, message string
}

func (e *engineError) Error() string { return e.code + ": " + e.message }
func errf(code, format string, a ...any) *engineError {
	return &engineError{code, fmt.Sprintf(format, a...)}
}

type event struct {
	EventID    int            `json:"event_id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes"`
}

type request struct {
	ID     string          `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func validateHistory(history []event) error {
	if len(history) == 0 {
		return errf("INVALID_HISTORY", "history must not be empty")
	}
	for i, e := range history {
		if e.EventID != i+1 {
			return errf("INVALID_HISTORY", "event_id must be sequential starting at 1, got %d at index %d", e.EventID, i)
		}
	}
	return nil
}

func replay(workflowType string, history []event) ([]map[string]any, error) {
	if !slices.Contains(knownTypes, workflowType) {
		return nil, errf("WORKFLOW_TYPE_NOT_FOUND", `unknown workflow type %q`, workflowType)
	}
	if err := validateHistory(history); err != nil {
		return nil, err
	}

	last := history[len(history)-1].Type
	if last == "WORKFLOW_EXECUTION_COMPLETED" || last == "WORKFLOW_EXECUTION_FAILED" {
		return []map[string]any{}, nil
	}

	switch workflowType {
	case "greet":
		if last != "WORKFLOW_EXECUTION_STARTED" {
			return nil, errf("INVALID_HISTORY", "unexpected last event %s for greet", last)
		}
		inp, _ := history[0].Attributes["input"].(map[string]any)
		name, _ := inp["name"].(string)
		return []map[string]any{{
			"type": "COMPLETE_WORKFLOW",
			"attributes": map[string]any{"result": map[string]any{"greeting": "hello " + name}},
		}}, nil

	case "fetch":
		inp := history[0].Attributes["input"]
		switch last {
		case "WORKFLOW_EXECUTION_STARTED":
			return []map[string]any{{
				"type": "SCHEDULE_ACTIVITY",
				"attributes": map[string]any{
					"activity_id": "fetch", "activity_type": "fetch", "input": inp,
				},
			}}, nil
		case "ACTIVITY_TASK_SCHEDULED":
			return []map[string]any{}, nil
		case "ACTIVITY_TASK_COMPLETED":
			result := history[len(history)-1].Attributes["result"]
			return []map[string]any{{
				"type":       "COMPLETE_WORKFLOW",
				"attributes": map[string]any{"result": result},
			}}, nil
		default:
			return nil, errf("INVALID_HISTORY", "unexpected last event %s for fetch", last)
		}

	case "timer_wait":
		switch last {
		case "WORKFLOW_EXECUTION_STARTED":
			return []map[string]any{{
				"type": "START_TIMER",
				"attributes": map[string]any{"timer_id": "t1", "duration_ms": 500},
			}}, nil
		case "TIMER_STARTED":
			return []map[string]any{}, nil
		case "TIMER_FIRED":
			return []map[string]any{{
				"type":       "COMPLETE_WORKFLOW",
				"attributes": map[string]any{"result": "timer fired"},
			}}, nil
		default:
			return nil, errf("INVALID_HISTORY", "unexpected last event %s for timer_wait", last)
		}

	case "signal_wait":
		switch last {
		case "WORKFLOW_EXECUTION_STARTED":
			return []map[string]any{}, nil
		case "WORKFLOW_EXECUTION_SIGNALED":
			result := history[len(history)-1].Attributes["input"]
			return []map[string]any{{
				"type":       "COMPLETE_WORKFLOW",
				"attributes": map[string]any{"result": result},
			}}, nil
		default:
			return nil, errf("INVALID_HISTORY", "unexpected last event %s for signal_wait", last)
		}

	case "pipeline":
		switch last {
		case "WORKFLOW_EXECUTION_STARTED":
			return []map[string]any{{
				"type": "SCHEDULE_ACTIVITY",
				"attributes": map[string]any{
					"activity_id": "step1", "activity_type": "work", "input": nil,
				},
			}}, nil
		case "ACTIVITY_TASK_SCHEDULED":
			return []map[string]any{}, nil
		case "ACTIVITY_TASK_COMPLETED":
			return []map[string]any{{
				"type": "START_TIMER",
				"attributes": map[string]any{"timer_id": "pause", "duration_ms": 100},
			}}, nil
		case "TIMER_STARTED":
			return []map[string]any{}, nil
		case "TIMER_FIRED":
			return []map[string]any{{
				"type":       "COMPLETE_WORKFLOW",
				"attributes": map[string]any{"result": "done"},
			}}, nil
		default:
			return nil, errf("INVALID_HISTORY", "unexpected last event %s for pipeline", last)
		}
	}

	return nil, errf("WORKFLOW_TYPE_NOT_FOUND", `unknown workflow type %q`, workflowType)
}

func handleRequest(method string, params json.RawMessage) (any, error) {
	switch method {
	case "ping":
		return map[string]string{"message": "pong"}, nil
	case "replay":
		var p struct {
			WorkflowType string  `json:"workflow_type"`
			History      []event `json:"history"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, errf("INVALID_HISTORY", "invalid replay params: %v", err)
		}
		cmds, err := replay(p.WorkflowType, p.History)
		if err != nil {
			return nil, err
		}
		return map[string]any{"commands": cmds}, nil
	default:
		return nil, errf("UNKNOWN_METHOD", "unknown method %q", method)
	}
}

func main() {
	port := flag.Int("port", 0, "TCP port")
	dataDir := flag.String("data-dir", "", "data directory")
	flag.Parse()
	if *port == 0 || *dataDir == "" {
		log.Fatal("usage: your_program.sh --port PORT --data-dir DIR")
	}
	_ = dataDir

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
				if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
					continue
				}
				result, err := handleRequest(req.Method, req.Params)
				var resp map[string]any
				if err != nil {
					var ee *engineError
					if e, ok := err.(*engineError); ok {
						ee = e
					} else {
						ee = errf("INTERNAL", "%v", err)
					}
					resp = map[string]any{"id": req.ID, "error": map[string]string{"code": ee.code, "message": ee.message}}
				} else {
					resp = map[string]any{"id": req.ID, "result": result}
				}
				b, _ := json.Marshal(resp)
				w.Write(b)
				w.WriteByte('\n')
				w.Flush()
			}
		}(conn)
	}
}
