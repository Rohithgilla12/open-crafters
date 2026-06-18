// Package temporal implements the stage tests for the
// "Build your own Temporal" challenge. See challenges/build-your-own-temporal/PROTOCOL.md
// for the wire protocol these tests exercise.
package temporal

import (
	"fmt"
	"time"

	"github.com/open-crafters/open-crafters/tester/internal/harness"
)

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-temporal/stages/"
	return harness.Challenge{
		Slug: "build-your-own-temporal",
		Name: "Build your own Temporal",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "start-workflow", Name: "Start a workflow", Instructions: docs + "02-start-workflow.md", Test: testStartWorkflow},
			{Slug: "complete-workflow", Name: "Dispatch and complete a workflow task", Instructions: docs + "03-complete-workflow.md", Test: testCompleteWorkflow},
			{Slug: "history", Name: "Append-only event history", Instructions: docs + "04-history.md", Test: testHistory},
			{Slug: "activities", Name: "Schedule and run activities", Instructions: docs + "05-activities.md", Test: testActivities},
			{Slug: "retries", Name: "Activity retries with backoff", Instructions: docs + "06-retries.md", Test: testRetries},
			{Slug: "timers", Name: "Durable timers", Instructions: docs + "07-timers.md", Test: testTimers},
			{Slug: "durability", Name: "Survive a crash", Instructions: docs + "08-durability.md", Test: testDurability},
			{Slug: "signals", Name: "Signals", Instructions: docs + "09-signals.md", Test: testSignals},
			{Slug: "concurrency", Name: "Concurrent workflows", Instructions: docs + "10-concurrency.md", Test: testConcurrency},
		},
	}
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

	// Interleave requests across the two connections: both must stay usable.
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

func testStartWorkflow(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	input := map[string]any{"name": "world"}
	ctx.Logf("starting workflow wf-1 (type=greet)")
	runID, err := startWorkflow(c, "wf-1", "greet", input)
	if err != nil {
		return fmt.Errorf("start_workflow: %w", err)
	}

	desc, err := describeWorkflow(c, "wf-1")
	if err != nil {
		return fmt.Errorf("describe_workflow: %w", err)
	}
	if desc.Status != "RUNNING" {
		return fmt.Errorf("describe_workflow: expected status RUNNING, got %q", desc.Status)
	}
	if desc.WorkflowID != "wf-1" || desc.RunID != runID || desc.WorkflowType != "greet" {
		return fmt.Errorf("describe_workflow: expected workflow_id=wf-1 run_id=%s workflow_type=greet, got workflow_id=%s run_id=%s workflow_type=%s",
			runID, desc.WorkflowID, desc.RunID, desc.WorkflowType)
	}
	ctx.Logf("workflow is RUNNING with run_id %s", runID)

	_, err = startWorkflow(c, "wf-1", "greet", input)
	if err := expectRPCError(err, "WORKFLOW_ALREADY_EXISTS", "starting wf-1 a second time"); err != nil {
		return err
	}
	_, err = describeWorkflow(c, "no-such-workflow")
	if err := expectRPCError(err, "WORKFLOW_NOT_FOUND", "describing an unknown workflow"); err != nil {
		return err
	}
	ctx.Logf("duplicate start and unknown workflow are rejected with the right error codes")
	return nil
}

func testCompleteWorkflow(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	input := map[string]any{"name": "world"}
	runID, err := startWorkflow(c, "wf-1", "greet", input)
	if err != nil {
		return fmt.Errorf("start_workflow: %w", err)
	}

	ctx.Logf("polling for a workflow task")
	task, err := pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return err
	}
	if task.WorkflowID != "wf-1" || task.RunID != runID || task.WorkflowType != "greet" {
		return fmt.Errorf("workflow task: expected workflow_id=wf-1 run_id=%s workflow_type=greet, got workflow_id=%s run_id=%s workflow_type=%s",
			runID, task.WorkflowID, task.RunID, task.WorkflowType)
	}
	if err := checkHistory(task.History, []wantEvent{
		{Type: "WORKFLOW_EXECUTION_STARTED", Attrs: map[string]any{"workflow_type": "greet", "input": input}},
	}); err != nil {
		return fmt.Errorf("workflow task history: %w", err)
	}
	ctx.Logf("got workflow task with WORKFLOW_EXECUTION_STARTED history")

	// A claimed task must not be delivered again.
	if err := expectNoWorkflowTask(c, 300*time.Millisecond); err != nil {
		return fmt.Errorf("after claiming the workflow task: %w", err)
	}

	result := map[string]any{"greeting": "hello world"}
	if err := completeWorkflowTask(c, task.TaskToken, cmdCompleteWorkflow(result)); err != nil {
		return fmt.Errorf("complete_workflow_task: %w", err)
	}
	desc, err := describeWorkflow(c, "wf-1")
	if err != nil {
		return err
	}
	if desc.Status != "COMPLETED" {
		return fmt.Errorf("after COMPLETE_WORKFLOW: expected status COMPLETED, got %q", desc.Status)
	}
	if !jsonEq(desc.Result, result) {
		return fmt.Errorf(`after COMPLETE_WORKFLOW: expected result {"greeting": "hello world"}, got %v`, desc.Result)
	}
	ctx.Logf("workflow COMPLETED with the worker-provided result")

	err = completeWorkflowTask(c, "bogus-token")
	if err := expectRPCError(err, "TASK_NOT_FOUND", "completing a workflow task with an invalid token"); err != nil {
		return err
	}
	return nil
}

func testHistory(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// Workflow 1: an empty completion means "waiting for more events" — the
	// server must not finish the workflow, and must not re-deliver a task
	// when nothing new happened.
	if _, err := startWorkflow(c, "wf-waiting", "waiter", nil); err != nil {
		return err
	}
	task, err := pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return err
	}
	if err := completeWorkflowTask(c, task.TaskToken); err != nil {
		return fmt.Errorf("complete_workflow_task with empty commands: %w", err)
	}
	if err := expectNoWorkflowTask(c, 400*time.Millisecond); err != nil {
		return fmt.Errorf("after an empty completion with no new events: %w", err)
	}
	desc, err := describeWorkflow(c, "wf-waiting")
	if err != nil {
		return err
	}
	if desc.Status != "RUNNING" {
		return fmt.Errorf("a workflow task completed with no commands must leave the workflow RUNNING, got %q", desc.Status)
	}
	ctx.Logf("empty command list keeps the workflow RUNNING with no spurious task re-delivery")

	// Workflow 2: history must be complete, ordered, and 1-indexed.
	input := map[string]any{"n": 7}
	if _, err := startWorkflow(c, "wf-2", "echo", input); err != nil {
		return err
	}
	task, err = pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return err
	}
	if task.WorkflowID != "wf-2" {
		return fmt.Errorf("expected a workflow task for wf-2, got one for %q", task.WorkflowID)
	}
	if err := completeWorkflowTask(c, task.TaskToken, cmdCompleteWorkflow(7)); err != nil {
		return err
	}
	events, err := getHistory(c, "wf-2")
	if err != nil {
		return fmt.Errorf("get_history: %w", err)
	}
	if err := checkHistory(events, []wantEvent{
		{Type: "WORKFLOW_EXECUTION_STARTED", Attrs: map[string]any{"workflow_type": "echo", "input": input}},
		{Type: "WORKFLOW_EXECUTION_COMPLETED", Attrs: map[string]any{"result": 7}},
	}); err != nil {
		return fmt.Errorf("get_history for wf-2: %w", err)
	}
	ctx.Logf("history for wf-2 is correct: STARTED, COMPLETED with sequential event_ids")

	_, err = getHistory(c, "no-such-workflow")
	return expectRPCError(err, "WORKFLOW_NOT_FOUND", "get_history for an unknown workflow")
}

func testActivities(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	input := map[string]any{"a": 2, "b": 3}
	if _, err := startWorkflow(c, "wf-act", "calculator", input); err != nil {
		return err
	}
	task, err := pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return err
	}
	ctx.Logf("scheduling activity a1 (type=multiply)")
	if err := completeWorkflowTask(c, task.TaskToken,
		cmdScheduleActivity("a1", "multiply", input, nil)); err != nil {
		return fmt.Errorf("complete_workflow_task with SCHEDULE_ACTIVITY: %w", err)
	}

	at, err := pollActivityTaskUntil(c, pollTimeout)
	if err != nil {
		return err
	}
	if at.WorkflowID != "wf-act" || at.ActivityID != "a1" || at.ActivityType != "multiply" {
		return fmt.Errorf("activity task: expected workflow_id=wf-act activity_id=a1 activity_type=multiply, got workflow_id=%s activity_id=%s activity_type=%s",
			at.WorkflowID, at.ActivityID, at.ActivityType)
	}
	if !jsonEq(at.Input, input) {
		return fmt.Errorf("activity task: expected input %v, got %v", input, at.Input)
	}
	if at.Attempt != 1 {
		return fmt.Errorf("activity task: expected attempt 1, got %d", at.Attempt)
	}
	ctx.Logf("worker received activity task (attempt 1)")

	// The workflow is blocked on the activity: no workflow task yet.
	if err := expectNoWorkflowTask(c, 300*time.Millisecond); err != nil {
		return fmt.Errorf("while the activity is outstanding: %w", err)
	}

	if err := completeActivityTask(c, at.TaskToken, 6); err != nil {
		return fmt.Errorf("complete_activity_task: %w", err)
	}
	task, err = pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return fmt.Errorf("after completing the activity: %w", err)
	}
	if err := checkHistory(task.History, []wantEvent{
		{Type: "WORKFLOW_EXECUTION_STARTED", Attrs: map[string]any{"workflow_type": "calculator", "input": input}},
		{Type: "ACTIVITY_TASK_SCHEDULED", Attrs: map[string]any{"activity_id": "a1", "activity_type": "multiply", "input": input}},
		{Type: "ACTIVITY_TASK_COMPLETED", Attrs: map[string]any{"activity_id": "a1", "result": 6}},
	}); err != nil {
		return fmt.Errorf("workflow task history after activity completion: %w", err)
	}
	ctx.Logf("workflow task delivered with ACTIVITY_TASK_COMPLETED in history")

	if err := completeWorkflowTask(c, task.TaskToken, cmdCompleteWorkflow(6)); err != nil {
		return err
	}
	desc, err := describeWorkflow(c, "wf-act")
	if err != nil {
		return err
	}
	if desc.Status != "COMPLETED" || !jsonEq(desc.Result, 6) {
		return fmt.Errorf("expected COMPLETED with result 6, got status=%s result=%v", desc.Status, desc.Result)
	}
	return nil
}

func testRetries(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := startWorkflow(c, "wf-retry", "flaky", nil); err != nil {
		return err
	}
	task, err := pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return err
	}
	retryPolicy := map[string]any{"maximum_attempts": 3, "initial_interval_ms": 200, "backoff_coefficient": 2.0}
	ctx.Logf("scheduling activity with retry policy: max 3 attempts, 200ms initial backoff, 2.0 coefficient")
	if err := completeWorkflowTask(c, task.TaskToken,
		cmdScheduleActivity("a1", "always-fails", nil, retryPolicy)); err != nil {
		return err
	}

	// Expected delays before attempts 2 and 3: 200ms, then 400ms. We assert a
	// lower bound (with slack for scheduling) to catch immediate re-delivery.
	minDelays := []time.Duration{0, 150 * time.Millisecond, 350 * time.Millisecond}
	for attempt := 1; attempt <= 3; attempt++ {
		start := time.Now()
		at, err := pollActivityTaskUntil(c, pollTimeout)
		if err != nil {
			return fmt.Errorf("waiting for attempt %d: %w", attempt, err)
		}
		elapsed := time.Since(start)
		if at.Attempt != attempt {
			return fmt.Errorf("expected activity task attempt %d, got %d", attempt, at.Attempt)
		}
		if elapsed < minDelays[attempt-1] {
			return fmt.Errorf("attempt %d was delivered after %s — too early for the backoff schedule (expected ≥ %s)",
				attempt, elapsed.Round(time.Millisecond), minDelays[attempt-1])
		}
		ctx.Logf("attempt %d delivered after %s", attempt, elapsed.Round(time.Millisecond))
		if err := failActivityTask(c, at.TaskToken, "transient failure"); err != nil {
			return fmt.Errorf("fail_activity_task (attempt %d): %w", attempt, err)
		}
	}

	// Attempts exhausted: the failure becomes a history event and the
	// workflow is woken up.
	task, err = pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return fmt.Errorf("after exhausting retries: %w", err)
	}
	if err := checkHistory(task.History, []wantEvent{
		{Type: "WORKFLOW_EXECUTION_STARTED"},
		{Type: "ACTIVITY_TASK_SCHEDULED", Attrs: map[string]any{"activity_id": "a1"}},
		{Type: "ACTIVITY_TASK_FAILED", Attrs: map[string]any{"activity_id": "a1", "error": "transient failure"}},
	}); err != nil {
		return fmt.Errorf("history after exhausted retries (intermediate failures must NOT be recorded): %w", err)
	}
	ctx.Logf("only the final failure was recorded in history")

	if err := completeWorkflowTask(c, task.TaskToken, cmdFailWorkflow("activity a1 failed")); err != nil {
		return err
	}
	desc, err := describeWorkflow(c, "wf-retry")
	if err != nil {
		return err
	}
	if desc.Status != "FAILED" || !jsonEq(desc.Error, "activity a1 failed") {
		return fmt.Errorf(`expected status FAILED with error "activity a1 failed", got status=%s error=%v`, desc.Status, desc.Error)
	}
	return nil
}

func testTimers(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := startWorkflow(c, "wf-timer", "sleeper", nil); err != nil {
		return err
	}
	task, err := pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return err
	}
	ctx.Logf("starting a 500ms timer")
	timerSet := time.Now()
	if err := completeWorkflowTask(c, task.TaskToken, cmdStartTimer("t1", 500)); err != nil {
		return err
	}

	// The timer must not fire early. Anchor the check to an absolute instant
	// well before the 500ms fire time so a slow StartTimer RPC on a loaded
	// runner can't drift the window across the fire time. The real lower-bound
	// guarantee is the elapsed >= ~500ms assertion below; this just catches a
	// timer that fires immediately.
	if err := expectNoWorkflowTaskBefore(c, timerSet.Add(250*time.Millisecond)); err != nil {
		return fmt.Errorf("the timer fired too early: %w", err)
	}
	task, err = pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return fmt.Errorf("waiting for the timer to fire: %w", err)
	}
	elapsed := time.Since(timerSet)
	if elapsed < 400*time.Millisecond {
		return fmt.Errorf("timer fired after %s, expected ≥ 500ms", elapsed.Round(time.Millisecond))
	}
	ctx.Logf("timer fired after %s", elapsed.Round(time.Millisecond))
	if err := checkHistory(task.History, []wantEvent{
		{Type: "WORKFLOW_EXECUTION_STARTED"},
		{Type: "TIMER_STARTED", Attrs: map[string]any{"timer_id": "t1", "duration_ms": 500}},
		{Type: "TIMER_FIRED", Attrs: map[string]any{"timer_id": "t1"}},
	}); err != nil {
		return fmt.Errorf("history after timer fired: %w", err)
	}

	if err := completeWorkflowTask(c, task.TaskToken, cmdCompleteWorkflow("woke up")); err != nil {
		return err
	}
	desc, err := describeWorkflow(c, "wf-timer")
	if err != nil {
		return err
	}
	if desc.Status != "COMPLETED" {
		return fmt.Errorf("expected COMPLETED, got %q", desc.Status)
	}
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}

	if _, err := startWorkflow(c, "wf-durable", "sleeper", map[string]any{"important": true}); err != nil {
		return err
	}
	task, err := pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return err
	}
	if err := completeWorkflowTask(c, task.TaskToken, cmdStartTimer("t1", 1500)); err != nil {
		return err
	}

	ctx.Logf("killing your server (SIGKILL) while a 1500ms timer is pending...")
	c.Close()
	ctx.KillProgram()
	if err := ctx.StartProgram(); err != nil {
		return fmt.Errorf("restarting your program: %w", err)
	}
	c, err = ctx.Dial()
	if err != nil {
		return fmt.Errorf("reconnecting after restart: %w", err)
	}

	desc, err := describeWorkflow(c, "wf-durable")
	if err != nil {
		return fmt.Errorf("describe_workflow after restart: %w", err)
	}
	if desc.Status != "RUNNING" {
		return fmt.Errorf("after restart: expected wf-durable to still be RUNNING, got %q", desc.Status)
	}
	events, err := getHistory(c, "wf-durable")
	if err != nil {
		return err
	}
	if err := checkHistory(events, []wantEvent{
		{Type: "WORKFLOW_EXECUTION_STARTED", Attrs: map[string]any{"input": map[string]any{"important": true}}},
		{Type: "TIMER_STARTED", Attrs: map[string]any{"timer_id": "t1"}},
	}); err != nil {
		return fmt.Errorf("history after restart: %w", err)
	}
	ctx.Logf("workflow state and history survived the crash")

	task, err = pollWorkflowTaskUntil(c, 6*time.Second)
	if err != nil {
		return fmt.Errorf("the pending timer must still fire after a restart: %w", err)
	}
	if len(task.History) != 3 || task.History[2].Type != "TIMER_FIRED" {
		return fmt.Errorf("expected TIMER_FIRED as event 3 after restart, got history %v", eventTypes(task.History))
	}
	ctx.Logf("pending timer fired after the restart")

	// Crash again while this workflow task is claimed but not completed: the
	// claim must be forgotten and the task re-delivered.
	ctx.Logf("killing your server again while a workflow task is claimed...")
	c.Close()
	ctx.KillProgram()
	if err := ctx.StartProgram(); err != nil {
		return fmt.Errorf("restarting your program (second time): %w", err)
	}
	c, err = ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	task, err = pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return fmt.Errorf("a claimed-but-incomplete workflow task must be re-delivered after a restart: %w", err)
	}
	if err := completeWorkflowTask(c, task.TaskToken, cmdCompleteWorkflow("survived")); err != nil {
		return err
	}

	// Completed state must also survive a restart.
	c.Close()
	ctx.KillProgram()
	if err := ctx.StartProgram(); err != nil {
		return err
	}
	c, err = ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	desc, err = describeWorkflow(c, "wf-durable")
	if err != nil {
		return err
	}
	if desc.Status != "COMPLETED" || !jsonEq(desc.Result, "survived") {
		return fmt.Errorf(`after final restart: expected COMPLETED with result "survived", got status=%s result=%v`, desc.Status, desc.Result)
	}
	ctx.Logf("completed result survived a third restart")
	return nil
}

func testSignals(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := startWorkflow(c, "wf-sig", "approval", nil); err != nil {
		return err
	}
	task, err := pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return err
	}
	// The workflow is now idle, waiting for a signal.
	if err := completeWorkflowTask(c, task.TaskToken); err != nil {
		return err
	}

	signalInput := map[string]any{"by": "alice"}
	ctx.Logf("sending signal 'approve' to wf-sig")
	if err := signalWorkflow(c, "wf-sig", "approve", signalInput); err != nil {
		return fmt.Errorf("signal_workflow: %w", err)
	}
	task, err = pollWorkflowTaskUntil(c, pollTimeout)
	if err != nil {
		return fmt.Errorf("a signal must wake the workflow up with a new workflow task: %w", err)
	}
	if err := checkHistory(task.History, []wantEvent{
		{Type: "WORKFLOW_EXECUTION_STARTED"},
		{Type: "WORKFLOW_EXECUTION_SIGNALED", Attrs: map[string]any{"signal_name": "approve", "input": signalInput}},
	}); err != nil {
		return fmt.Errorf("history after signal: %w", err)
	}
	if err := completeWorkflowTask(c, task.TaskToken, cmdCompleteWorkflow("approved by alice")); err != nil {
		return err
	}
	ctx.Logf("signal delivered and workflow completed")

	err = signalWorkflow(c, "wf-sig", "approve", nil)
	if err := expectRPCError(err, "WORKFLOW_CLOSED", "signaling a completed workflow"); err != nil {
		return err
	}
	err = signalWorkflow(c, "no-such-workflow", "approve", nil)
	return expectRPCError(err, "WORKFLOW_NOT_FOUND", "signaling an unknown workflow")
}

func testConcurrency(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const n = 5
	ctx.Logf("starting %d workflows", n)
	for i := 0; i < n; i++ {
		wfID := fmt.Sprintf("wf-c-%d", i)
		if _, err := startWorkflow(c, wfID, "square", i); err != nil {
			return fmt.Errorf("starting %s: %w", wfID, err)
		}
	}

	// Claim all first workflow tasks before completing any, forcing the
	// server to track several outstanding tasks at once.
	inputs := map[string]int{}
	firstTasks := map[string]*WorkflowTask{}
	for i := 0; i < n; i++ {
		task, err := pollWorkflowTaskUntil(c, pollTimeout)
		if err != nil {
			return fmt.Errorf("claiming first workflow tasks (%d claimed so far): %w", len(firstTasks), err)
		}
		if firstTasks[task.WorkflowID] != nil {
			return fmt.Errorf("workflow task for %s was delivered twice", task.WorkflowID)
		}
		if len(task.History) != 1 {
			return fmt.Errorf("first workflow task for %s: expected 1 history event, got %d", task.WorkflowID, len(task.History))
		}
		var wfInput int
		var idx int
		if _, err := fmt.Sscanf(task.WorkflowID, "wf-c-%d", &idx); err != nil {
			return fmt.Errorf("unexpected workflow_id in task: %q", task.WorkflowID)
		}
		if !jsonEq(task.History[0].Attributes["input"], idx) {
			return fmt.Errorf("%s: history input %v does not match the input it was started with (%d) — workflow state must be isolated",
				task.WorkflowID, task.History[0].Attributes["input"], idx)
		}
		wfInput = idx
		inputs[task.WorkflowID] = wfInput
		firstTasks[task.WorkflowID] = task
	}
	for wfID, task := range firstTasks {
		if err := completeWorkflowTask(c, task.TaskToken,
			cmdScheduleActivity("a1", "square", inputs[wfID], nil)); err != nil {
			return fmt.Errorf("scheduling activity for %s: %w", wfID, err)
		}
	}

	// Same for activity tasks: claim all, then complete all.
	activityTasks := map[string]*ActivityTask{}
	for i := 0; i < n; i++ {
		at, err := pollActivityTaskUntil(c, pollTimeout)
		if err != nil {
			return fmt.Errorf("claiming activity tasks (%d claimed so far): %w", len(activityTasks), err)
		}
		if activityTasks[at.WorkflowID] != nil {
			return fmt.Errorf("activity task for %s was delivered twice", at.WorkflowID)
		}
		if !jsonEq(at.Input, inputs[at.WorkflowID]) {
			return fmt.Errorf("activity task for %s: expected input %d, got %v (activity inputs crossed between workflows?)",
				at.WorkflowID, inputs[at.WorkflowID], at.Input)
		}
		activityTasks[at.WorkflowID] = at
	}
	for wfID, at := range activityTasks {
		x := inputs[wfID]
		if err := completeActivityTask(c, at.TaskToken, x*x); err != nil {
			return fmt.Errorf("completing activity for %s: %w", wfID, err)
		}
	}

	for i := 0; i < n; i++ {
		task, err := pollWorkflowTaskUntil(c, pollTimeout)
		if err != nil {
			return fmt.Errorf("waiting for post-activity workflow tasks: %w", err)
		}
		x := inputs[task.WorkflowID]
		if err := checkHistory(task.History, []wantEvent{
			{Type: "WORKFLOW_EXECUTION_STARTED", Attrs: map[string]any{"input": x}},
			{Type: "ACTIVITY_TASK_SCHEDULED", Attrs: map[string]any{"input": x}},
			{Type: "ACTIVITY_TASK_COMPLETED", Attrs: map[string]any{"result": x * x}},
		}); err != nil {
			return fmt.Errorf("history for %s: %w", task.WorkflowID, err)
		}
		if err := completeWorkflowTask(c, task.TaskToken, cmdCompleteWorkflow(x*x)); err != nil {
			return err
		}
	}

	for i := 0; i < n; i++ {
		wfID := fmt.Sprintf("wf-c-%d", i)
		desc, err := describeWorkflow(c, wfID)
		if err != nil {
			return err
		}
		if desc.Status != "COMPLETED" || !jsonEq(desc.Result, i*i) {
			return fmt.Errorf("%s: expected COMPLETED with result %d, got status=%s result=%v", wfID, i*i, desc.Status, desc.Result)
		}
	}
	ctx.Logf("all %d workflows completed with isolated, correct results", n)
	return nil
}

func eventTypes(events []Event) []string {
	var types []string
	for _, e := range events {
		types = append(types, e.Type)
	}
	return types
}
