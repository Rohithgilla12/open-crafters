package scheduler

import (
	"errors"
	"fmt"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

const (
	pollInterval = 40 * time.Millisecond
	pollWait     = 5 * time.Second
)

type Job struct {
	JobID    string `json:"job_id"`
	Payload  any    `json:"payload"`
	Attempt  int    `json:"attempt"`
	LeaseTok string `json:"lease_token"`
}

type JobInfo struct {
	JobID    string `json:"job_id"`
	Status   string `json:"status"`
	Payload  any    `json:"payload"`
	RunAtMS  int64  `json:"run_at_ms"`
	Attempt  int    `json:"attempt"`
	Result   any    `json:"result"`
	Error    any    `json:"error"`
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-scheduler/stages/"
	return harness.Challenge{
		Slug: "build-your-own-scheduler",
		Name: "Build your own scheduler",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "schedule", Name: "Schedule a delayed job", Difficulty: "easy", Instructions: docs + "02-schedule.md", Test: testSchedule},
			{Slug: "complete", Name: "Complete a job", Difficulty: "easy", Instructions: docs + "03-complete.md", Test: testComplete},
			{Slug: "lease", Name: "Job leases", Difficulty: "medium", Instructions: docs + "04-lease.md", Test: testLease},
			{Slug: "retry", Name: "Retries", Difficulty: "medium", Instructions: docs + "05-retry.md", Test: testRetry},
			{Slug: "cancel", Name: "Cancel a job", Difficulty: "easy", Instructions: docs + "06-cancel.md", Test: testCancel},
			{Slug: "durability", Name: "Survive a crash", Difficulty: "hard", Instructions: docs + "07-durability.md", Test: testDurability},
			{Slug: "recurring", Name: "Recurring jobs", Difficulty: "medium", Instructions: docs + "08-recurring.md", Test: testRecurring},
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
		return fmt.Errorf(`ping: expected "pong", got %q`, res.Message)
	}
	return nil
}

func schedule(c *harness.Client, params map[string]any) (string, error) {
	var res struct {
		JobID string `json:"job_id"`
	}
	if err := c.Call("schedule", params, &res); err != nil {
		return "", err
	}
	if res.JobID == "" {
		return "", errors.New("schedule returned empty job_id")
	}
	return res.JobID, nil
}

func poll(c *harness.Client) (*Job, error) {
	var res struct {
		Job *Job `json:"job"`
	}
	if err := c.Call("poll", nil, &res); err != nil {
		return nil, err
	}
	if res.Job != nil && res.Job.LeaseTok == "" {
		return nil, errors.New("poll returned job with empty lease_token")
	}
	return res.Job, nil
}

func pollWithin(c *harness.Client, within time.Duration) (*Job, error) {
	deadline := time.Now().Add(within)
	for {
		j, err := poll(c)
		if err != nil {
			return nil, err
		}
		if j != nil {
			return j, nil
		}
		if !time.Now().Before(deadline) {
			return nil, nil
		}
		time.Sleep(pollInterval)
	}
}

func expectNoJob(c *harness.Client, d time.Duration) error {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		j, err := poll(c)
		if err != nil {
			return err
		}
		if j != nil {
			return fmt.Errorf("expected no job for %s, but got job_id=%q", d, j.JobID)
		}
		time.Sleep(pollInterval)
	}
	return nil
}

func complete(c *harness.Client, token string, result any) error {
	params := map[string]any{"lease_token": token}
	if result != nil {
		params["result"] = result
	}
	return c.Call("complete", params, nil)
}

func failJob(c *harness.Client, token, errMsg string) error {
	return c.Call("fail", map[string]any{"lease_token": token, "error": errMsg}, nil)
}

func cancel(c *harness.Client, jobID string) (bool, error) {
	var res struct {
		Cancelled bool `json:"cancelled"`
	}
	if err := c.Call("cancel", map[string]any{"job_id": jobID}, &res); err != nil {
		return false, err
	}
	return res.Cancelled, nil
}

func getJob(c *harness.Client, jobID string) (*JobInfo, error) {
	var res JobInfo
	if err := c.Call("get_job", map[string]any{"job_id": jobID}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func expectRPCError(err error, code, context string) error {
	if err == nil {
		return fmt.Errorf("%s: expected error %q, call succeeded", context, code)
	}
	var rpcErr *harness.RPCError
	if !errors.As(err, &rpcErr) {
		return fmt.Errorf("%s: expected %q, got %v", context, code, err)
	}
	if rpcErr.Code != code {
		return fmt.Errorf("%s: expected %q, got %q (%s)", context, code, rpcErr.Code, rpcErr.Message)
	}
	return nil
}

func testBind(ctx *harness.Context) error {
	c1, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c1.Close()
	c2, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c2.Close()
	for i := 0; i < 3; i++ {
		if err := ping(c1); err != nil {
			return err
		}
		if err := ping(c2); err != nil {
			return err
		}
	}
	ctx.Logf("both connections answered ping")
	return nil
}

func testSchedule(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	payload := map[string]any{"task": "hello"}
	ctx.Logf("scheduling job with delay_ms=300")
	jobID, err := schedule(c, map[string]any{"payload": payload, "delay_ms": 300})
	if err != nil {
		return err
	}

	j, err := poll(c)
	if err != nil {
		return err
	}
	if j != nil {
		return fmt.Errorf("job should not be pollable immediately after schedule (delay_ms=300)")
	}

	start := time.Now()
	j, err = pollWithin(c, pollWait)
	if err != nil {
		return err
	}
	if j == nil {
		return fmt.Errorf("job did not become pollable within %s", pollWait)
	}
	elapsed := time.Since(start)
	if elapsed < 250*time.Millisecond {
		return fmt.Errorf("job polled too early after %s (expected ≥ 250ms for delay_ms=300)", elapsed)
	}
	if j.JobID != jobID {
		return fmt.Errorf("expected job_id %q, got %q", jobID, j.JobID)
	}
	if j.Attempt != 1 {
		return fmt.Errorf("expected attempt 1, got %d", j.Attempt)
	}
	ctx.Logf("job %q became pollable after %s with correct payload", jobID, elapsed)
	return nil
}

func testComplete(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	jobID, err := schedule(c, map[string]any{"payload": "work", "delay_ms": 0})
	if err != nil {
		return err
	}
	j, err := pollWithin(c, pollWait)
	if err != nil || j == nil {
		return fmt.Errorf("expected due job, got err=%v job=%v", err, j)
	}
	if err := complete(c, j.LeaseTok, "done"); err != nil {
		return err
	}
	j, err = poll(c)
	if err != nil {
		return err
	}
	if j != nil {
		return fmt.Errorf("completed job should not be pollable again, got job_id=%q", j.JobID)
	}
	info, err := getJob(c, jobID)
	if err != nil {
		return err
	}
	if info.Status != "completed" {
		return fmt.Errorf("get_job: expected status completed, got %q", info.Status)
	}
	ctx.Logf("job completed and removed from poll queue")
	return nil
}

func testLease(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	_, err = schedule(c, map[string]any{"payload": "lease-test", "delay_ms": 0, "lease_ms": 400})
	if err != nil {
		return err
	}
	j1, err := pollWithin(c, pollWait)
	if err != nil || j1 == nil {
		return fmt.Errorf("expected due job: %v", err)
	}
	j2, err := poll(c)
	if err != nil {
		return err
	}
	if j2 != nil {
		return fmt.Errorf("leased job must not be pollable again immediately (got job_id=%q)", j2.JobID)
	}
	ctx.Logf("leased job hidden from poll")
	time.Sleep(500 * time.Millisecond)
	j3, err := pollWithin(c, pollWait)
	if err != nil || j3 == nil {
		return fmt.Errorf("job should be pollable after lease expiry: %v", err)
	}
	if j3.JobID != j1.JobID {
		return fmt.Errorf("expected same job_id %q after lease expiry, got %q", j1.JobID, j3.JobID)
	}
	if j3.Attempt != j1.Attempt {
		return fmt.Errorf("lease expiry should not increment attempt (got %d, want %d)", j3.Attempt, j1.Attempt)
	}
	if j3.LeaseTok == j1.LeaseTok {
		return fmt.Errorf("lease expiry must issue a new lease_token")
	}
	ctx.Logf("job redelivered after lease expiry with new token")
	return nil
}

func testRetry(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	_, err = schedule(c, map[string]any{
		"payload": map[string]any{"n": 1}, "delay_ms": 0,
		"retry_policy": map[string]any{"maximum_attempts": 3, "retry_delay_ms": 200},
	})
	if err != nil {
		return err
	}
	for attempt := 1; attempt <= 3; attempt++ {
		j, err := pollWithin(c, pollWait)
		if err != nil || j == nil {
			return fmt.Errorf("attempt %d: expected pollable job, got err=%v", attempt, err)
		}
		if j.Attempt != attempt {
			return fmt.Errorf("attempt %d: expected poll attempt=%d, got %d", attempt, attempt, j.Attempt)
		}
		if err := failJob(c, j.LeaseTok, "boom"); err != nil {
			return fmt.Errorf("fail attempt %d: %w", attempt, err)
		}
		if attempt < 3 {
			time.Sleep(250 * time.Millisecond)
		}
	}
	if err := expectNoJob(c, 500*time.Millisecond); err != nil {
		return err
	}
	ctx.Logf("three failures exhausted retries")
	return nil
}

func testCancel(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	jobID, err := schedule(c, map[string]any{"payload": "later", "delay_ms": 10000})
	if err != nil {
		return err
	}
	ok, err := cancel(c, jobID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("cancel pending job: expected cancelled=true")
	}
	if err := expectNoJob(c, 400*time.Millisecond); err != nil {
		return err
	}
	info, err := getJob(c, jobID)
	if err != nil {
		return err
	}
	if info.Status != "cancelled" {
		return fmt.Errorf("get_job: expected status cancelled, got %q", info.Status)
	}
	ok, err = cancel(c, jobID)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("cancel already-cancelled job: expected cancelled=false")
	}
	ctx.Logf("pending job cancelled and never polled")
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	ctx.Logf("scheduling job with delay_ms=2000")
	jobID, err := schedule(c, map[string]any{"payload": "durable", "delay_ms": 2000})
	if err != nil {
		return err
	}
	scheduledAt := time.Now()
	c.Close()

	ctx.Logf("SIGKILL after brief pause")
	time.Sleep(300 * time.Millisecond)
	ctx.KillProgram()
	if err := ctx.StartProgram(); err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	restartedAt := time.Now()

	c, err = ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	j, err := pollWithin(c, 3*time.Second)
	if err != nil {
		return err
	}
	if j == nil {
		return fmt.Errorf("job %q did not fire after restart", jobID)
	}
	if j.JobID != jobID {
		return fmt.Errorf("expected job_id %q, got %q", jobID, j.JobID)
	}
	total := time.Since(scheduledAt)
	restartWait := time.Since(restartedAt)
	if total < 1800*time.Millisecond {
		return fmt.Errorf("job fired too early (%s after schedule) — run_at_ms must be absolute, not delay-from-reboot", total)
	}
	if restartWait > 1800*time.Millisecond {
		return fmt.Errorf("job fired too late after restart (%s) — should fire near original run_at_ms", restartWait)
	}
	ctx.Logf("job fired at original scheduled time after crash (total %s, after restart %s)", total, restartWait)
	return nil
}

func testRecurring(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	_, err = schedule(c, map[string]any{
		"payload": map[string]any{"tick": 1}, "delay_ms": 0, "interval_ms": 300,
	})
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for run := 1; run <= 2; run++ {
		start := time.Now()
		j, err := pollWithin(c, pollWait)
		if err != nil || j == nil {
			return fmt.Errorf("recurring run %d: no job polled", run)
		}
		if seen[j.JobID] {
			return fmt.Errorf("recurring run %d: duplicate job_id %q", run, j.JobID)
		}
		seen[j.JobID] = true
		if run > 1 && time.Since(start) < 250*time.Millisecond {
			return fmt.Errorf("recurring run %d polled too early (%s)", run, time.Since(start))
		}
		if err := complete(c, j.LeaseTok, run); err != nil {
			return err
		}
		ctx.Logf("completed recurring run %d (job_id=%s)", run, j.JobID)
	}
	ctx.Logf("two recurring runs completed with distinct job_ids")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	j1, err := schedule(c, map[string]any{"payload": "ga-1", "delay_ms": 100})
	if err != nil {
		return err
	}
	j2, err := schedule(c, map[string]any{"payload": "ga-2", "delay_ms": 200})
	if err != nil {
		return err
	}
	j3, err := schedule(c, map[string]any{"payload": "ga-cancel", "delay_ms": 5000})
	if err != nil {
		return err
	}
	if ok, _ := cancel(c, j3); !ok {
		return fmt.Errorf("gauntlet: failed to cancel long job")
	}

	ctx.KillProgram()
	if err := ctx.StartProgram(); err != nil {
		return err
	}
	c.Close()
	c, err = ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	done := map[string]bool{j1: false, j2: false}
	deadline := time.Now().Add(3 * time.Second)
	for len(done) > 0 && time.Now().Before(deadline) {
		j, err := poll(c)
		if err != nil {
			return err
		}
		if j == nil {
			time.Sleep(pollInterval)
			continue
		}
		if j.JobID == j3 {
			return fmt.Errorf("cancelled job %q was polled", j3)
		}
		if _, ok := done[j.JobID]; !ok {
			return fmt.Errorf("unexpected job_id %q", j.JobID)
		}
		if err := complete(c, j.LeaseTok, nil); err != nil {
			return err
		}
		delete(done, j.JobID)
	}
	if len(done) > 0 {
		return fmt.Errorf("gauntlet: jobs not completed after restart: %v", done)
	}
	ctx.Logf("gauntlet passed")
	return nil
}
