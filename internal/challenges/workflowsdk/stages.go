// Package workflowsdk implements the stage tests for the
// "Build your own workflow SDK" challenge. See
// challenges/build-your-own-workflow-sdk/PROTOCOL.md for the wire protocol.
package workflowsdk

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Rohithgilla12/open-crafters/internal/challenges/temporal"
	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-workflow-sdk/stages/"
	return harness.Challenge{
		Slug: "build-your-own-workflow-sdk",
		Name: "Build your own workflow SDK",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "simple-complete", Name: "Replay to completion", Difficulty: "easy", Instructions: docs + "02-simple-complete.md", Test: testSimpleComplete},
			{Slug: "schedule-activity", Name: "Schedule an activity", Difficulty: "medium", Instructions: docs + "03-schedule-activity.md", Test: testScheduleActivity},
			{Slug: "activity-result", Name: "React to activity completion", Difficulty: "medium", Instructions: docs + "04-activity-result.md", Test: testActivityResult},
			{Slug: "waiting", Name: "Waiting means empty commands", Difficulty: "medium", Instructions: docs + "05-waiting.md", Test: testWaiting},
			{Slug: "timers", Name: "Durable timers in replay", Difficulty: "medium", Instructions: docs + "06-timers.md", Test: testTimers},
			{Slug: "signals", Name: "Signals in replay", Difficulty: "medium", Instructions: docs + "07-signals.md", Test: testSignals},
			{Slug: "determinism", Name: "Same history, same commands", Difficulty: "hard", Instructions: docs + "08-determinism.md", Test: testDeterminism},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", Test: testGauntlet},
		},
	}
}

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

func replay(c *harness.Client, workflowType string, history []temporal.Event) ([]map[string]any, error) {
	var res struct {
		Commands []map[string]any `json:"commands"`
	}
	if err := c.Call("replay", map[string]any{
		"workflow_type": workflowType,
		"history":         history,
	}, &res); err != nil {
		return nil, err
	}
	if res.Commands == nil {
		res.Commands = []map[string]any{}
	}
	return res.Commands, nil
}

func event(id int, typ string, attrs map[string]any) temporal.Event {
	return temporal.Event{EventID: id, Type: typ, Attributes: attrs}
}

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

func checkCommands(got []map[string]any, want []map[string]any) error {
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		return fmt.Errorf("expected commands %s, got %s", wantJSON, gotJSON)
	}
	return nil
}

func testBind(ctx *harness.Context) error {
	ctx.Logf("connecting two concurrent clients to %s", ctx.Addr())
	c1, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c1.Close()
	c2, err := ctx.Dial()
	if err != nil {
		return fmt.Errorf("second concurrent connection: %w", err)
	}
	defer c2.Close()

	for i := 0; i < 3; i++ {
		if err := ping(c1); err != nil {
			return fmt.Errorf("ping on connection 1: %w", err)
		}
		if err := ping(c2); err != nil {
			return fmt.Errorf("ping on connection 2: %w", err)
		}
	}
	ctx.Logf("both connections answered ping")
	return nil
}

func testSimpleComplete(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	input := map[string]any{"name": "world"}
	history := []temporal.Event{
		event(1, "WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": "greet",
			"input":         input,
		}),
	}

	ctx.Logf("replaying greet with only WORKFLOW_EXECUTION_STARTED")
	cmds, err := replay(c, "greet", history)
	if err != nil {
		return fmt.Errorf("replay: %w", err)
	}
	want := []map[string]any{
		{"type": "COMPLETE_WORKFLOW", "attributes": map[string]any{"result": map[string]any{"greeting": "hello world"}}},
	}
	if err := checkCommands(cmds, want); err != nil {
		return err
	}

	doneHistory := append(history, event(2, "WORKFLOW_EXECUTION_COMPLETED", map[string]any{
		"result": map[string]any{"greeting": "hello world"},
	}))
	cmds, err = replay(c, "greet", doneHistory)
	if err != nil {
		return fmt.Errorf("replay completed workflow: %w", err)
	}
	if len(cmds) != 0 {
		return fmt.Errorf("completed workflow should emit no commands, got %v", cmds)
	}

	_, err = replay(c, "nope", history)
	if err := expectRPCError(err, "WORKFLOW_TYPE_NOT_FOUND", "unknown workflow type"); err != nil {
		return err
	}
	ctx.Logf("greet replays to COMPLETE_WORKFLOW; terminal histories return empty commands")
	return nil
}

func testScheduleActivity(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	input := map[string]any{"url": "https://example.com"}
	history := []temporal.Event{
		event(1, "WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": "fetch",
			"input":         input,
		}),
	}

	ctx.Logf("replaying fetch after start — should schedule activity")
	cmds, err := replay(c, "fetch", history)
	if err != nil {
		return fmt.Errorf("replay: %w", err)
	}
	want := []map[string]any{
		{"type": "SCHEDULE_ACTIVITY", "attributes": map[string]any{
			"activity_id": "fetch", "activity_type": "fetch", "input": input,
		}},
	}
	if err := checkCommands(cmds, want); err != nil {
		return err
	}
	ctx.Logf("fetch schedules activity with correct id, type, and input")
	return nil
}

func testActivityResult(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	input := map[string]any{"url": "https://example.com"}
	activityResult := map[string]any{"status": 200, "body": "ok"}
	history := []temporal.Event{
		event(1, "WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": "fetch", "input": input,
		}),
		event(2, "ACTIVITY_TASK_SCHEDULED", map[string]any{
			"activity_id": "fetch", "activity_type": "fetch", "input": input,
		}),
		event(3, "ACTIVITY_TASK_COMPLETED", map[string]any{
			"activity_id": "fetch", "result": activityResult,
		}),
	}

	ctx.Logf("replaying fetch after activity completion")
	cmds, err := replay(c, "fetch", history)
	if err != nil {
		return fmt.Errorf("replay: %w", err)
	}
	want := []map[string]any{
		{"type": "COMPLETE_WORKFLOW", "attributes": map[string]any{"result": activityResult}},
	}
	if err := checkCommands(cmds, want); err != nil {
		return err
	}
	ctx.Logf("activity result flows through to COMPLETE_WORKFLOW without re-scheduling")
	return nil
}

func testWaiting(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	input := map[string]any{"url": "https://example.com"}
	started := []temporal.Event{
		event(1, "WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": "fetch", "input": input,
		}),
	}

	ctx.Logf("fetch after activity scheduled but not completed — should wait")
	scheduled := append(started, event(2, "ACTIVITY_TASK_SCHEDULED", map[string]any{
		"activity_id": "fetch", "activity_type": "fetch", "input": input,
	}))
	cmds, err := replay(c, "fetch", scheduled)
	if err != nil {
		return fmt.Errorf("replay waiting for activity: %w", err)
	}
	if len(cmds) != 0 {
		return fmt.Errorf("workflow waiting for activity completion must emit no commands, got %v", cmds)
	}

	ctx.Logf("signal_wait after start — waiting for signal")
	signalStarted := []temporal.Event{
		event(1, "WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": "signal_wait", "input": nil,
		}),
	}
	cmds, err = replay(c, "signal_wait", signalStarted)
	if err != nil {
		return fmt.Errorf("replay signal_wait: %w", err)
	}
	if len(cmds) != 0 {
		return fmt.Errorf("workflow waiting for signal must emit no commands, got %v", cmds)
	}
	ctx.Logf("waiting states correctly return empty command lists")
	return nil
}

func testTimers(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	started := []temporal.Event{
		event(1, "WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": "timer_wait", "input": nil,
		}),
	}

	ctx.Logf("timer_wait after start — should start timer")
	cmds, err := replay(c, "timer_wait", started)
	if err != nil {
		return fmt.Errorf("replay: %w", err)
	}
	wantStart := []map[string]any{
		{"type": "START_TIMER", "attributes": map[string]any{"timer_id": "t1", "duration_ms": 500}},
	}
	if err := checkCommands(cmds, wantStart); err != nil {
		return err
	}

	waiting := append(started, event(2, "TIMER_STARTED", map[string]any{
		"timer_id": "t1", "duration_ms": 500,
	}))
	cmds, err = replay(c, "timer_wait", waiting)
	if err != nil {
		return fmt.Errorf("replay waiting for timer: %w", err)
	}
	if len(cmds) != 0 {
		return fmt.Errorf("workflow waiting for timer must emit no commands, got %v", cmds)
	}

	fired := append(waiting, event(3, "TIMER_FIRED", map[string]any{"timer_id": "t1"}))
	cmds, err = replay(c, "timer_wait", fired)
	if err != nil {
		return fmt.Errorf("replay after timer fired: %w", err)
	}
	wantComplete := []map[string]any{
		{"type": "COMPLETE_WORKFLOW", "attributes": map[string]any{"result": "timer fired"}},
	}
	if err := checkCommands(cmds, wantComplete); err != nil {
		return err
	}
	ctx.Logf("timer_wait schedules, waits, then completes after TIMER_FIRED")
	return nil
}

func testSignals(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	signalInput := map[string]any{"value": 42}
	history := []temporal.Event{
		event(1, "WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": "signal_wait", "input": nil,
		}),
		event(2, "WORKFLOW_EXECUTION_SIGNALED", map[string]any{
			"signal_name": "go", "input": signalInput,
		}),
	}

	ctx.Logf("signal_wait after signal received")
	cmds, err := replay(c, "signal_wait", history)
	if err != nil {
		return fmt.Errorf("replay: %w", err)
	}
	want := []map[string]any{
		{"type": "COMPLETE_WORKFLOW", "attributes": map[string]any{"result": signalInput}},
	}
	if err := checkCommands(cmds, want); err != nil {
		return err
	}
	ctx.Logf("signal input becomes workflow result")
	return nil
}

func testDeterminism(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	history := []temporal.Event{
		event(1, "WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": "greet", "input": map[string]any{"name": "determinism"},
		}),
	}

	ctx.Logf("replaying the same history 20 times — commands must be identical")
	var first []byte
	for i := 0; i < 20; i++ {
		cmds, err := replay(c, "greet", history)
		if err != nil {
			return fmt.Errorf("replay iteration %d: %w", i, err)
		}
		b, err := json.Marshal(cmds)
		if err != nil {
			return err
		}
		if i == 0 {
			first = b
		} else if string(b) != string(first) {
			return fmt.Errorf("determinism violated on iteration %d: first replay returned %s, got %s", i, first, b)
		}
	}
	ctx.Logf("20 replays produced byte-identical commands")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	input := map[string]any{"task": "pipeline"}

	h1 := []temporal.Event{
		event(1, "WORKFLOW_EXECUTION_STARTED", map[string]any{
			"workflow_type": "pipeline", "input": input,
		}),
	}
	cmds, err := replay(c, "pipeline", h1)
	if err != nil {
		return fmt.Errorf("pipeline step 1: %w", err)
	}
	if err := checkCommands(cmds, []map[string]any{
		{"type": "SCHEDULE_ACTIVITY", "attributes": map[string]any{
			"activity_id": "step1", "activity_type": "work", "input": nil,
		}},
	}); err != nil {
		return fmt.Errorf("after start: %w", err)
	}

	h2 := append(h1, event(2, "ACTIVITY_TASK_SCHEDULED", map[string]any{
		"activity_id": "step1", "activity_type": "work", "input": nil,
	}))
	cmds, err = replay(c, "pipeline", h2)
	if err != nil {
		return fmt.Errorf("pipeline waiting: %w", err)
	}
	if len(cmds) != 0 {
		return fmt.Errorf("pipeline waiting for activity: expected no commands, got %v", cmds)
	}

	h3 := append(h2, event(3, "ACTIVITY_TASK_COMPLETED", map[string]any{
		"activity_id": "step1", "result": "step1 done",
	}))
	cmds, err = replay(c, "pipeline", h3)
	if err != nil {
		return fmt.Errorf("pipeline after activity: %w", err)
	}
	if err := checkCommands(cmds, []map[string]any{
		{"type": "START_TIMER", "attributes": map[string]any{"timer_id": "pause", "duration_ms": 100}},
	}); err != nil {
		return err
	}

	h4 := append(h3, event(4, "TIMER_STARTED", map[string]any{
		"timer_id": "pause", "duration_ms": 100,
	}))
	cmds, err = replay(c, "pipeline", h4)
	if err != nil {
		return fmt.Errorf("pipeline waiting for timer: %w", err)
	}
	if len(cmds) != 0 {
		return fmt.Errorf("pipeline waiting for timer: expected no commands, got %v", cmds)
	}

	h5 := append(h4, event(5, "TIMER_FIRED", map[string]any{"timer_id": "pause"}))
	cmds, err = replay(c, "pipeline", h5)
	if err != nil {
		return fmt.Errorf("pipeline after timer: %w", err)
	}
	if err := checkCommands(cmds, []map[string]any{
		{"type": "COMPLETE_WORKFLOW", "attributes": map[string]any{"result": "done"}},
	}); err != nil {
		return err
	}

	ctx.Logf("pipeline gauntlet: activity → timer → complete with correct waiting states")
	return nil
}
