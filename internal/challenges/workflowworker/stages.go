// Package workflowworker implements the meta-compose "Build your own workflow worker"
// challenge. The harness spawns reference temporal and workflow-sdk binaries;
// the user implements the worker gateway.
package workflowworker

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

var composeServices = []harness.ServiceSpec{
	{Name: "temporal", ReferenceChallenge: "build-your-own-temporal", EnvAddrKey: "TEMPORAL_ADDR"},
	{Name: "sdk", ReferenceChallenge: "build-your-own-workflow-sdk", EnvAddrKey: "SDK_ADDR"},
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-workflow-worker/stages/"
	return harness.Challenge{
		Slug:     "build-your-own-workflow-worker",
		Name:     "Build your own workflow worker",
		Services: composeServices,
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the stack", Difficulty: "easy", Instructions: docs + "01-bind.md", TestCompose: testBind},
			{Slug: "greet", Name: "Run greet", Difficulty: "easy", Instructions: docs + "02-greet.md", TestCompose: testGreet},
			{Slug: "fetch", Name: "Activity workflow", Difficulty: "medium", Instructions: docs + "03-fetch.md", TestCompose: testFetch},
			{Slug: "timer", Name: "Timer workflow", Difficulty: "medium", Instructions: docs + "04-timer.md", TestCompose: testTimer},
			{Slug: "pipeline", Name: "Activity then timer", Difficulty: "medium", Instructions: docs + "05-pipeline.md", TestCompose: testPipeline},
			{Slug: "signal", Name: "Signal delivery", Difficulty: "medium", Instructions: docs + "06-signal.md", TestCompose: testSignal},
			{Slug: "duplicate", Name: "Reject duplicate start", Difficulty: "easy", Instructions: docs + "07-duplicate.md", TestCompose: testDuplicate},
			{Slug: "concurrent", Name: "Parallel workflows", Difficulty: "hard", Instructions: docs + "08-concurrent.md", TestCompose: testConcurrent},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", TestCompose: testGauntlet},
		},
	}
}

const (
	runTimeout   = 15 * time.Second
	timerTimeout = 3 * time.Second
)

type gwResult struct {
	Status string `json:"status"`
	Result any    `json:"result"`
	Error  any    `json:"error"`
}

func gwPing(c *harness.Client) error {
	var res struct {
		Message string `json:"message"`
	}
	if err := c.Call("ping", nil, &res); err != nil {
		return err
	}
	if res.Message != "pong" {
		return fmt.Errorf(`ping: expected "pong", got %q`, res.Message)
	}
	return nil
}

func gwRunWorkflow(c *harness.Client, workflowID, workflowType string, input any) (*gwResult, error) {
	var res gwResult
	params := map[string]any{
		"workflow_id":   workflowID,
		"workflow_type": workflowType,
		"input":         input,
		"task_queue":    "default",
	}
	if err := c.Call("run_workflow", params, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func gwStartWorkflow(c *harness.Client, workflowID, workflowType string, input any) (string, error) {
	var res struct {
		RunID string `json:"run_id"`
	}
	if err := c.Call("start_workflow", map[string]any{
		"workflow_id":   workflowID,
		"workflow_type": workflowType,
		"input":         input,
		"task_queue":    "default",
	}, &res); err != nil {
		return "", err
	}
	if res.RunID == "" {
		return "", errors.New("start_workflow: empty run_id")
	}
	return res.RunID, nil
}

func gwSignalWorkflow(c *harness.Client, workflowID, signalName string, input any) error {
	return c.Call("signal_workflow", map[string]any{
		"workflow_id": workflowID,
		"signal_name": signalName,
		"input":       input,
	}, nil)
}

func gwAwaitWorkflow(c *harness.Client, workflowID string) (*gwResult, error) {
	var res gwResult
	if err := c.Call("await_workflow", map[string]any{"workflow_id": workflowID}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func jsonEq(a, b any) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ja) == string(jb)
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

func testBind(ctx *harness.ComposeContext) error {
	c, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer c.Close()
	c2, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer c2.Close()
	for i := 0; i < 3; i++ {
		if err := gwPing(c); err != nil {
			return fmt.Errorf("ping conn 1: %w", err)
		}
		if err := gwPing(c2); err != nil {
			return fmt.Errorf("ping conn 2: %w", err)
		}
	}
	ctx.Logf("gateway answered ping on two connections")
	return nil
}

func testGreet(ctx *harness.ComposeContext) error {
	c, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer c.Close()

	res, err := gwRunWorkflow(c, "wf-greet", "greet", map[string]any{"name": "world"})
	if err != nil {
		return fmt.Errorf("run_workflow: %w", err)
	}
	if res.Status != "COMPLETED" {
		return fmt.Errorf("expected COMPLETED, got %q", res.Status)
	}
	want := map[string]any{"greeting": "hello world"}
	if !jsonEq(res.Result, want) {
		return fmt.Errorf("expected result %v, got %v", want, res.Result)
	}
	ctx.Logf("greet workflow completed with %v", res.Result)
	return nil
}

func testFetch(ctx *harness.ComposeContext) error {
	c, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer c.Close()

	input := map[string]any{"url": "https://example.com"}
	res, err := gwRunWorkflow(c, "wf-fetch", "fetch", input)
	if err != nil {
		return fmt.Errorf("run_workflow: %w", err)
	}
	if res.Status != "COMPLETED" {
		return fmt.Errorf("expected COMPLETED, got %q", res.Status)
	}
	want := map[string]any{"status": 200, "body": "ok"}
	if !jsonEq(res.Result, want) {
		return fmt.Errorf("expected activity result %v, got %v", want, res.Result)
	}
	ctx.Logf("fetch workflow completed via activity stub")
	return nil
}

func testTimer(ctx *harness.ComposeContext) error {
	c, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer c.Close()

	done := make(chan error, 1)
	go func() {
		res, err := gwRunWorkflow(c, "wf-timer", "timer_wait", nil)
		if err != nil {
			done <- err
			return
		}
		if res.Status != "COMPLETED" {
			done <- fmt.Errorf("expected COMPLETED, got %q", res.Status)
			return
		}
		if res.Result != "timer fired" {
			done <- fmt.Errorf(`expected result "timer fired", got %v`, res.Result)
			return
		}
		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-time.After(timerTimeout):
		return fmt.Errorf("timer_wait did not complete within %s", timerTimeout)
	}
	ctx.Logf("timer_wait workflow completed after durable timer fired")
	return nil
}

func testPipeline(ctx *harness.ComposeContext) error {
	c, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer c.Close()

	res, err := gwRunWorkflow(c, "wf-pipe", "pipeline", nil)
	if err != nil {
		return fmt.Errorf("run_workflow: %w", err)
	}
	if res.Status != "COMPLETED" {
		return fmt.Errorf("expected COMPLETED, got %q", res.Status)
	}
	if res.Result != "done" {
		return fmt.Errorf(`expected result "done", got %v`, res.Result)
	}
	ctx.Logf("pipeline workflow completed activity + timer path")
	return nil
}

func testSignal(ctx *harness.ComposeContext) error {
	c, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := gwStartWorkflow(c, "wf-sig", "signal_wait", nil); err != nil {
		return fmt.Errorf("start_workflow: %w", err)
	}

	sigInput := map[string]any{"n": 42}
	done := make(chan error, 1)
	go func() {
		awaitClient, err := ctx.DialGateway()
		if err != nil {
			done <- err
			return
		}
		defer awaitClient.Close()
		res, err := gwAwaitWorkflow(awaitClient, "wf-sig")
		if err != nil {
			done <- err
			return
		}
		if res.Status != "COMPLETED" {
			done <- fmt.Errorf("expected COMPLETED, got %q", res.Status)
			return
		}
		if !jsonEq(res.Result, sigInput) {
			done <- fmt.Errorf("expected signal input as result %v, got %v", sigInput, res.Result)
			return
		}
		done <- nil
	}()

	time.Sleep(150 * time.Millisecond)
	if err := gwSignalWorkflow(c, "wf-sig", "ready", sigInput); err != nil {
		return fmt.Errorf("signal_workflow: %w", err)
	}

	select {
	case err := <-done:
		if err != nil {
			return err
		}
	case <-time.After(runTimeout):
		return fmt.Errorf("await_workflow did not complete within %s", runTimeout)
	}
	ctx.Logf("signal_wait workflow completed after signal delivery")
	return nil
}

func testDuplicate(ctx *harness.ComposeContext) error {
	c, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := gwRunWorkflow(c, "wf-dup", "greet", map[string]any{"name": "a"}); err != nil {
		return fmt.Errorf("first run_workflow: %w", err)
	}
	_, err = gwRunWorkflow(c, "wf-dup", "greet", map[string]any{"name": "b"})
	if err := expectRPCError(err, "WORKFLOW_ALREADY_EXISTS", "duplicate run_workflow"); err != nil {
		return err
	}
	ctx.Logf("duplicate workflow_id rejected")
	return nil
}

func testConcurrent(ctx *harness.ComposeContext) error {
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i, id := range []string{"wf-c1", "wf-c2"} {
		wg.Add(1)
		go func(wfID string, n int) {
			defer wg.Done()
			c, err := ctx.DialGateway()
			if err != nil {
				errs <- err
				return
			}
			defer c.Close()
			res, err := gwRunWorkflow(c, wfID, "greet", map[string]any{"name": fmt.Sprintf("u%d", n)})
			if err != nil {
				errs <- err
				return
			}
			if res.Status != "COMPLETED" {
				errs <- fmt.Errorf("%s: expected COMPLETED, got %q", wfID, res.Status)
			}
		}(id, i+1)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	ctx.Logf("two greet workflows completed concurrently")
	return nil
}

func testGauntlet(ctx *harness.ComposeContext) error {
	c, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := gwRunWorkflow(c, "wf-g1", "greet", map[string]any{"name": "gauntlet"}); err != nil {
		return fmt.Errorf("greet: %w", err)
	}
	if _, err := gwRunWorkflow(c, "wf-g2", "fetch", map[string]any{"url": "https://x.test"}); err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	res, err := gwRunWorkflow(c, "wf-g3", "timer_wait", nil)
	if err != nil {
		return fmt.Errorf("timer: %w", err)
	}
	if res.Status != "COMPLETED" {
		return fmt.Errorf("timer: expected COMPLETED, got %q", res.Status)
	}
	ctx.Logf("gauntlet: greet, fetch, and timer_wait all completed")
	return nil
}
