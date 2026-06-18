package temporal

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/open-crafters/open-crafters/tester/internal/harness"
)

const (
	defaultQueue = "default"
	pollInterval = 50 * time.Millisecond
	pollTimeout  = 5 * time.Second
)

type Event struct {
	EventID    int            `json:"event_id"`
	Type       string         `json:"type"`
	Attributes map[string]any `json:"attributes"`
}

type WorkflowTask struct {
	TaskToken    string  `json:"task_token"`
	WorkflowID   string  `json:"workflow_id"`
	RunID        string  `json:"run_id"`
	WorkflowType string  `json:"workflow_type"`
	History      []Event `json:"history"`
}

type ActivityTask struct {
	TaskToken    string `json:"task_token"`
	WorkflowID   string `json:"workflow_id"`
	RunID        string `json:"run_id"`
	ActivityID   string `json:"activity_id"`
	ActivityType string `json:"activity_type"`
	Input        any    `json:"input"`
	Attempt      int    `json:"attempt"`
}

type WorkflowDescription struct {
	WorkflowID   string `json:"workflow_id"`
	RunID        string `json:"run_id"`
	WorkflowType string `json:"workflow_type"`
	Status       string `json:"status"`
	Result       any    `json:"result"`
	Error        any    `json:"error"`
}

// --- typed wrappers over the wire protocol ---

func ping(c *harness.Client) error {
	var res struct {
		Message string `json:"message"`
	}
	if err := c.Call("ping", nil, &res); err != nil {
		return err
	}
	if res.Message != "pong" {
		return fmt.Errorf(`ping result: expected {"message": "pong"}, got message %q`, res.Message)
	}
	return nil
}

func startWorkflow(c *harness.Client, workflowID, workflowType string, input any) (runID string, err error) {
	var res struct {
		RunID string `json:"run_id"`
	}
	err = c.Call("start_workflow", map[string]any{
		"workflow_id":   workflowID,
		"workflow_type": workflowType,
		"input":         input,
		"task_queue":    defaultQueue,
	}, &res)
	if err != nil {
		return "", err
	}
	if res.RunID == "" {
		return "", errors.New("start_workflow result: run_id is missing or empty")
	}
	return res.RunID, nil
}

func describeWorkflow(c *harness.Client, workflowID string) (*WorkflowDescription, error) {
	var res WorkflowDescription
	if err := c.Call("describe_workflow", map[string]any{"workflow_id": workflowID}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func getHistory(c *harness.Client, workflowID string) ([]Event, error) {
	var res struct {
		Events []Event `json:"events"`
	}
	if err := c.Call("get_history", map[string]any{"workflow_id": workflowID}, &res); err != nil {
		return nil, err
	}
	return res.Events, nil
}

func pollWorkflowTask(c *harness.Client) (*WorkflowTask, error) {
	var res struct {
		Task *WorkflowTask `json:"task"`
	}
	if err := c.Call("poll_workflow_task", map[string]any{"task_queue": defaultQueue}, &res); err != nil {
		return nil, err
	}
	return res.Task, nil
}

func pollActivityTask(c *harness.Client) (*ActivityTask, error) {
	var res struct {
		Task *ActivityTask `json:"task"`
	}
	if err := c.Call("poll_activity_task", map[string]any{"task_queue": defaultQueue}, &res); err != nil {
		return nil, err
	}
	return res.Task, nil
}

func completeWorkflowTask(c *harness.Client, token string, commands ...map[string]any) error {
	if commands == nil {
		commands = []map[string]any{}
	}
	return c.Call("complete_workflow_task", map[string]any{"task_token": token, "commands": commands}, nil)
}

func completeActivityTask(c *harness.Client, token string, result any) error {
	return c.Call("complete_activity_task", map[string]any{"task_token": token, "result": result}, nil)
}

func failActivityTask(c *harness.Client, token, errMsg string) error {
	return c.Call("fail_activity_task", map[string]any{"task_token": token, "error": errMsg}, nil)
}

func signalWorkflow(c *harness.Client, workflowID, signalName string, input any) error {
	return c.Call("signal_workflow", map[string]any{
		"workflow_id": workflowID, "signal_name": signalName, "input": input,
	}, nil)
}

// --- polling helpers ---

// pollWorkflowTaskUntil polls until a workflow task is delivered or the
// timeout elapses.
func pollWorkflowTaskUntil(c *harness.Client, timeout time.Duration) (*WorkflowTask, error) {
	deadline := time.Now().Add(timeout)
	for {
		task, err := pollWorkflowTask(c)
		if err != nil {
			return nil, err
		}
		if task != nil {
			if task.TaskToken == "" {
				return nil, errors.New("workflow task has missing or empty task_token")
			}
			return task, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("no workflow task delivered within %s", timeout)
		}
		time.Sleep(pollInterval)
	}
}

func pollActivityTaskUntil(c *harness.Client, timeout time.Duration) (*ActivityTask, error) {
	deadline := time.Now().Add(timeout)
	for {
		task, err := pollActivityTask(c)
		if err != nil {
			return nil, err
		}
		if task != nil {
			if task.TaskToken == "" {
				return nil, errors.New("activity task has missing or empty task_token")
			}
			return task, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("no activity task delivered within %s", timeout)
		}
		time.Sleep(pollInterval)
	}
}

// expectNoWorkflowTask asserts that no workflow task is delivered for the
// given duration.
func expectNoWorkflowTask(c *harness.Client, d time.Duration) error {
	return expectNoWorkflowTaskBefore(c, time.Now().Add(d))
}

// expectNoWorkflowTaskBefore asserts that no workflow task is delivered until
// the given absolute deadline. Anchoring to an absolute instant (rather than a
// duration measured from the call) is what keeps a timer test honest on a
// loaded CI runner: a slow preceding RPC can't push the "not yet" window across
// the timer's fire time and flag a perfectly-timed fire as "too early".
func expectNoWorkflowTaskBefore(c *harness.Client, deadline time.Time) error {
	for time.Now().Before(deadline) {
		task, err := pollWorkflowTask(c)
		if err != nil {
			return err
		}
		if task != nil {
			return fmt.Errorf("expected no workflow task, but one was delivered (workflow_id=%q)", task.WorkflowID)
		}
		time.Sleep(pollInterval)
	}
	return nil
}

// --- command constructors ---

func cmdCompleteWorkflow(result any) map[string]any {
	return map[string]any{"type": "COMPLETE_WORKFLOW", "attributes": map[string]any{"result": result}}
}

func cmdFailWorkflow(errMsg string) map[string]any {
	return map[string]any{"type": "FAIL_WORKFLOW", "attributes": map[string]any{"error": errMsg}}
}

func cmdStartTimer(timerID string, durationMS int) map[string]any {
	return map[string]any{"type": "START_TIMER", "attributes": map[string]any{"timer_id": timerID, "duration_ms": durationMS}}
}

func cmdScheduleActivity(activityID, activityType string, input any, retryPolicy map[string]any) map[string]any {
	attrs := map[string]any{"activity_id": activityID, "activity_type": activityType, "input": input}
	if retryPolicy != nil {
		attrs["retry_policy"] = retryPolicy
	}
	return map[string]any{"type": "SCHEDULE_ACTIVITY", "attributes": attrs}
}

// --- assertion helpers ---

// jsonEq compares two values by their canonical JSON encoding.
func jsonEq(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ja) == string(jb)
}

type wantEvent struct {
	Type  string
	Attrs map[string]any // required attributes; extra attributes are allowed
}

// checkHistory verifies event count, sequential 1-based event_ids, types, and
// required attributes.
func checkHistory(events []Event, want []wantEvent) error {
	var types []string
	for _, e := range events {
		types = append(types, e.Type)
	}
	if len(events) != len(want) {
		var wantTypes []string
		for _, w := range want {
			wantTypes = append(wantTypes, w.Type)
		}
		return fmt.Errorf("expected history %v, got %v", wantTypes, types)
	}
	for i, e := range events {
		if e.EventID != i+1 {
			return fmt.Errorf("event #%d: expected event_id %d, got %d (event_ids must be sequential starting at 1)", i, i+1, e.EventID)
		}
		if e.Type != want[i].Type {
			return fmt.Errorf("event %d: expected type %s, got %s (full history: %v)", e.EventID, want[i].Type, e.Type, types)
		}
		for key, wantVal := range want[i].Attrs {
			gotVal, ok := e.Attributes[key]
			if !ok {
				return fmt.Errorf("event %d (%s): missing attribute %q", e.EventID, e.Type, key)
			}
			if !jsonEq(gotVal, wantVal) {
				wantJSON, _ := json.Marshal(wantVal)
				gotJSON, _ := json.Marshal(gotVal)
				return fmt.Errorf("event %d (%s): attribute %q: expected %s, got %s", e.EventID, e.Type, key, wantJSON, gotJSON)
			}
		}
	}
	return nil
}

// expectRPCError asserts that err is a protocol error with the given code.
func expectRPCError(err error, code string, context string) error {
	if err == nil {
		return fmt.Errorf("%s: expected error with code %q, but the call succeeded", context, code)
	}
	var rpcErr *harness.RPCError
	if !errors.As(err, &rpcErr) {
		return fmt.Errorf("%s: expected protocol error with code %q, got: %v", context, code, err)
	}
	if rpcErr.Code != code {
		return fmt.Errorf("%s: expected error code %q, got %q (%s)", context, code, rpcErr.Code, rpcErr.Message)
	}
	return nil
}
