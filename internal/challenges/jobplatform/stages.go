// Package jobplatform implements the meta-compose "Build your own job platform"
// challenge. The harness spawns reference scheduler, queue, and distributed-lock
// binaries; the user implements the orchestrator gateway.
package jobplatform

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

var composeServices = []harness.ServiceSpec{
	{Name: "scheduler", ReferenceChallenge: "build-your-own-scheduler", EnvAddrKey: "SCHEDULER_ADDR"},
	{Name: "queue", ReferenceChallenge: "build-your-own-queue", EnvAddrKey: "QUEUE_ADDR"},
	{Name: "lock", ReferenceChallenge: "build-your-own-distributed-lock", EnvAddrKey: "LOCK_ADDR"},
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-job-platform/stages/"
	return harness.Challenge{
		Slug:     "build-your-own-job-platform",
		Name:     "Build your own job platform",
		Services: composeServices,
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the stack", Difficulty: "easy", Instructions: docs + "01-bind.md", TestCompose: testBind},
			{Slug: "submit", Name: "Schedule a job", Difficulty: "easy", Instructions: docs + "02-submit.md", TestCompose: testSubmit},
			{Slug: "delayed", Name: "Delayed delivery", Difficulty: "easy", Instructions: docs + "03-delayed.md", TestCompose: testDelayed},
			{Slug: "complete", Name: "Complete work", Difficulty: "medium", Instructions: docs + "04-complete.md", TestCompose: testComplete},
			{Slug: "empty", Name: "Nothing to do", Difficulty: "easy", Instructions: docs + "05-empty.md", TestCompose: testEmpty},
			{Slug: "cancel", Name: "Cancel pending", Difficulty: "medium", Instructions: docs + "06-cancel.md", TestCompose: testCancel},
			{Slug: "multi", Name: "Several jobs", Difficulty: "medium", Instructions: docs + "07-multi.md", TestCompose: testMulti},
			{Slug: "concurrent", Name: "Parallel submits", Difficulty: "hard", Instructions: docs + "08-concurrent.md", TestCompose: testConcurrent},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", TestCompose: testGauntlet},
		},
	}
}

const pollInterval = 50 * time.Millisecond

type workItem struct {
	JobID      string          `json:"job_id"`
	Payload    json.RawMessage `json:"payload"`
	LeaseToken string          `json:"lease_token"`
	Receipt    string          `json:"receipt"`
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

func submitJob(c *harness.Client, payload any, delayMS int) (string, error) {
	var res struct {
		JobID string `json:"job_id"`
	}
	params := map[string]any{"payload": payload, "delay_ms": delayMS}
	if err := c.Call("submit_job", params, &res); err != nil {
		return "", err
	}
	if res.JobID == "" {
		return "", fmt.Errorf("submit_job: empty job_id")
	}
	return res.JobID, nil
}

func receiveWork(c *harness.Client) (*workItem, error) {
	var res struct {
		Work *workItem `json:"work"`
	}
	if err := c.Call("receive_work", nil, &res); err != nil {
		return nil, err
	}
	return res.Work, nil
}

func completeWork(c *harness.Client, leaseToken, receipt string) error {
	return c.Call("complete_work", map[string]any{
		"lease_token": leaseToken,
		"receipt":     receipt,
	}, nil)
}

func cancelJob(c *harness.Client, jobID string) (bool, error) {
	var res struct {
		Cancelled bool `json:"cancelled"`
	}
	if err := c.Call("cancel_job", map[string]any{"job_id": jobID}, &res); err != nil {
		return false, err
	}
	return res.Cancelled, nil
}

func getJobStatus(c *harness.Client, jobID string) (string, error) {
	var res struct {
		Status string `json:"status"`
	}
	if err := c.Call("get_job", map[string]any{"job_id": jobID}, &res); err != nil {
		return "", err
	}
	return res.Status, nil
}

func waitForWork(c *harness.Client, timeout time.Duration) (*workItem, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		w, err := receiveWork(c)
		if err != nil {
			return nil, err
		}
		if w != nil {
			return w, nil
		}
		time.Sleep(pollInterval)
	}
	return nil, fmt.Errorf("no work received within %s", timeout)
}

func testBind(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := gwPing(gw); err != nil {
		return err
	}
	for _, name := range []string{"scheduler", "queue", "lock"} {
		sc, err := ctx.DialService(name)
		if err != nil {
			return err
		}
		var res struct {
			Message string `json:"message"`
		}
		if err := sc.Call("ping", nil, &res); err != nil {
			sc.Close()
			return fmt.Errorf("%s ping: %w", name, err)
		}
		sc.Close()
		if res.Message != "pong" {
			return fmt.Errorf("%s ping: got %q", name, res.Message)
		}
	}
	ctx.Logf("gateway and three reference services answered ping")
	return nil
}

func testSubmit(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	jobID, err := submitJob(gw, map[string]any{"task": "email"}, 5000)
	if err != nil {
		return err
	}
	ctx.Logf("submitted job %q", jobID)
	return nil
}

func testDelayed(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	jobID, err := submitJob(gw, "delayed-payload", 300)
	if err != nil {
		return err
	}
	w, err := receiveWork(gw)
	if err != nil {
		return err
	}
	if w != nil {
		return fmt.Errorf("job %q delivered immediately (delay_ms=300)", jobID)
	}
	w, err = waitForWork(gw, 2*time.Second)
	if err != nil {
		return err
	}
	if w.JobID != jobID {
		return fmt.Errorf("expected job %q, got %q", jobID, w.JobID)
	}
	ctx.Logf("delayed job %q delivered after fire time", jobID)
	return nil
}

func testComplete(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	jobID, err := submitJob(gw, map[string]any{"n": 1}, 0)
	if err != nil {
		return err
	}
	w, err := waitForWork(gw, 2*time.Second)
	if err != nil {
		return err
	}
	if w.JobID != jobID || w.LeaseToken == "" || w.Receipt == "" {
		return fmt.Errorf("receive_work: missing fields for job %q", jobID)
	}
	if err := completeWork(gw, w.LeaseToken, w.Receipt); err != nil {
		return err
	}
	status, err := getJobStatus(gw, jobID)
	if err != nil {
		return err
	}
	if status != "completed" {
		return fmt.Errorf("job %q status %q, want completed", jobID, status)
	}
	ctx.Logf("job %q completed end-to-end", jobID)
	return nil
}

func testEmpty(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	w, err := receiveWork(gw)
	if err != nil {
		return err
	}
	if w != nil {
		return fmt.Errorf("expected work=null with no pending jobs, got job %q", w.JobID)
	}
	ctx.Logf("receive_work returns null when idle")
	return nil
}

func testCancel(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	jobID, err := submitJob(gw, "cancel-me", 10000)
	if err != nil {
		return err
	}
	cancelled, err := cancelJob(gw, jobID)
	if err != nil {
		return err
	}
	if !cancelled {
		return fmt.Errorf("cancel_job: expected cancelled=true")
	}
	status, err := getJobStatus(gw, jobID)
	if err != nil {
		return err
	}
	if status != "cancelled" {
		return fmt.Errorf("job %q status %q, want cancelled", jobID, status)
	}
	deadline := time.Now().Add(400 * time.Millisecond)
	for time.Now().Before(deadline) {
		w, err := receiveWork(gw)
		if err != nil {
			return err
		}
		if w != nil && w.JobID == jobID {
			return fmt.Errorf("cancelled job %q was delivered", jobID)
		}
		time.Sleep(pollInterval)
	}
	ctx.Logf("cancelled job never delivered")
	return nil
}

func testMulti(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	want := []string{"alpha", "beta", "gamma"}
	ids := make([]string, len(want))
	for i, p := range want {
		ids[i], err = submitJob(gw, p, 0)
		if err != nil {
			return err
		}
	}
	got := map[string]bool{}
	for range want {
		w, err := waitForWork(gw, 3*time.Second)
		if err != nil {
			return err
		}
		var payload string
		if err := json.Unmarshal(w.Payload, &payload); err != nil {
			return fmt.Errorf("payload decode: %w", err)
		}
		got[payload] = true
		if err := completeWork(gw, w.LeaseToken, w.Receipt); err != nil {
			return err
		}
	}
	for _, p := range want {
		if !got[p] {
			return fmt.Errorf("never received job with payload %q", p)
		}
	}
	ctx.Logf("three distinct jobs delivered and completed")
	return nil
}

func testConcurrent(ctx *harness.ComposeContext) error {
	const workers = 10
	ids := make([]string, workers)
	var errs atomic.Value
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			gw, err := ctx.DialGateway()
			if err != nil {
				errs.Store(err)
				return
			}
			defer gw.Close()
			<-start
			id, err := submitJob(gw, fmt.Sprintf("work-%d", n), 0)
			if err != nil {
				errs.Store(err)
				return
			}
			ids[n] = id
		}(i)
	}
	close(start)
	wg.Wait()
	if v := errs.Load(); v != nil {
		return v.(error)
	}
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	received := map[string]bool{}
	for range workers {
		w, err := waitForWork(gw, 5*time.Second)
		if err != nil {
			return err
		}
		received[w.JobID] = true
		if err := completeWork(gw, w.LeaseToken, w.Receipt); err != nil {
			return err
		}
	}
	for _, id := range ids {
		if id == "" || !received[id] {
			return fmt.Errorf("job %q not received", id)
		}
	}
	ctx.Logf("%d concurrent submits all delivered", workers)
	return nil
}

func testGauntlet(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	j1, err := submitJob(gw, "ga-1", 100)
	if err != nil {
		return err
	}
	j2, err := submitJob(gw, "ga-2", 0)
	if err != nil {
		return err
	}
	j3, err := submitJob(gw, "ga-cancel", 8000)
	if err != nil {
		return err
	}
	if _, err := cancelJob(gw, j3); err != nil {
		return err
	}
	w2, err := waitForWork(gw, 2*time.Second)
	if err != nil {
		return err
	}
	if w2.JobID != j2 {
		return fmt.Errorf("expected immediate job %q first, got %q", j2, w2.JobID)
	}
	if err := completeWork(gw, w2.LeaseToken, w2.Receipt); err != nil {
		return err
	}
	w1, err := waitForWork(gw, 2*time.Second)
	if err != nil {
		return err
	}
	if w1.JobID != j1 {
		return fmt.Errorf("expected delayed job %q, got %q", j1, w1.JobID)
	}
	if err := completeWork(gw, w1.LeaseToken, w1.Receipt); err != nil {
		return err
	}
	st, _ := getJobStatus(gw, j3)
	if st != "cancelled" {
		return fmt.Errorf("cancelled job status %q", st)
	}
	ctx.Logf("gauntlet: immediate + delayed delivery, complete, cancel")
	return nil
}
