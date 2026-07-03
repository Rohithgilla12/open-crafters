// Package idgen holds stage tests for "Build your own ID generator".
package idgen

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

const (
	snowflakeEpochMS = 1577836800000 // 2020-01-01T00:00:00Z
	maxWorkerID      = 1023
	maxSequence      = 4095
	maxBatch         = 1024
)

type parseResult struct {
	TimestampMS int64 `json:"timestamp_ms"`
	WorkerID    int64 `json:"worker_id"`
	Sequence    int64 `json:"sequence"`
}

type nextIDResult struct {
	ID string `json:"id"`
}

type batchResult struct {
	IDs []string `json:"ids"`
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-id-generator/stages/"
	return harness.Challenge{
		Slug: "build-your-own-id-generator",
		Name: "Build your own ID generator",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "next-id", Name: "Generate an ID", Difficulty: "easy", Instructions: docs + "02-next-id.md", Test: testNextID},
			{Slug: "sortable", Name: "Time-ordered IDs", Difficulty: "easy", Instructions: docs + "03-sortable.md", Test: testSortable},
			{Slug: "worker-id", Name: "Worker partitioning", Difficulty: "easy", Instructions: docs + "04-worker-id.md", Test: testWorkerID},
			{Slug: "sequence", Name: "Per-millisecond sequence", Difficulty: "medium", Instructions: docs + "05-sequence.md", Test: testSequence},
			{Slug: "clock-skew", Name: "Dense millisecond allocation", Difficulty: "medium", Instructions: docs + "06-clock-skew.md", Test: testClockSkew},
			{Slug: "batch", Name: "Batch allocate", Difficulty: "medium", Instructions: docs + "07-batch.md", Test: testBatch},
			{Slug: "durability", Name: "Survive a crash", Difficulty: "hard", Instructions: docs + "08-durability.md", Test: testDurability},
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

func configure(c *harness.Client, workerID int64) error {
	return c.Call("configure", map[string]any{"worker_id": workerID}, &struct{}{})
}

func nextID(c *harness.Client) (string, error) {
	var res nextIDResult
	if err := c.Call("next_id", nil, &res); err != nil {
		return "", err
	}
	if res.ID == "" {
		return "", errors.New("next_id returned empty id")
	}
	return res.ID, nil
}

func parseID(c *harness.Client, id string) (*parseResult, error) {
	var res parseResult
	if err := c.Call("parse", map[string]any{"id": id}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func batchIDs(c *harness.Client, count int64) ([]string, error) {
	var res batchResult
	if err := c.Call("batch", map[string]any{"count": count}, &res); err != nil {
		return nil, err
	}
	if int64(len(res.IDs)) != count {
		return nil, fmt.Errorf("batch: expected %d ids, got %d", count, len(res.IDs))
	}
	return res.IDs, nil
}

func idBig(id string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(id, 10)
	if !ok {
		return nil, fmt.Errorf("id %q is not a decimal integer", id)
	}
	if n.Sign() <= 0 {
		return nil, fmt.Errorf("id %q must be positive", id)
	}
	return n, nil
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

func testNextID(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	id, err := nextID(c)
	if err != nil {
		return err
	}
	p, err := parseID(c, id)
	if err != nil {
		return err
	}
	if p.TimestampMS < snowflakeEpochMS {
		return fmt.Errorf("parse timestamp_ms %d before epoch %d", p.TimestampMS, snowflakeEpochMS)
	}
	if p.WorkerID < 0 || p.WorkerID > maxWorkerID {
		return fmt.Errorf("worker_id %d out of range", p.WorkerID)
	}
	if p.Sequence < 0 || p.Sequence > maxSequence {
		return fmt.Errorf("sequence %d out of range", p.Sequence)
	}
	if err := c.Call("configure", map[string]any{}, &struct{}{}); expectRPCError(err, "INVALID_PARAMS", "configure missing worker_id") == nil {
		return expectRPCError(err, "INVALID_PARAMS", "configure missing worker_id")
	}
	ctx.Logf("next_id returned %s (ts=%d worker=%d seq=%d)", id, p.TimestampMS, p.WorkerID, p.Sequence)
	return nil
}

func testSortable(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	var prev *big.Int
	for i := 0; i < 50; i++ {
		id, err := nextID(c)
		if err != nil {
			return err
		}
		n, err := idBig(id)
		if err != nil {
			return err
		}
		if prev != nil && n.Cmp(prev) <= 0 {
			return fmt.Errorf("ids not strictly increasing: %s after %s", id, prev.String())
		}
		prev = n
		time.Sleep(2 * time.Millisecond)
	}
	ctx.Logf("50 sequential next_id calls produced strictly increasing ids")
	return nil
}

func testWorkerID(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := configure(c, 42); err != nil {
		return err
	}
	id, err := nextID(c)
	if err != nil {
		return err
	}
	p, err := parseID(c, id)
	if err != nil {
		return err
	}
	if p.WorkerID != 42 {
		return fmt.Errorf("expected worker_id 42, got %d", p.WorkerID)
	}
	if err := configure(c, 1000); err != nil {
		return err
	}
	id2, err := nextID(c)
	if err != nil {
		return err
	}
	p2, err := parseID(c, id2)
	if err != nil {
		return err
	}
	if p2.WorkerID != 1000 {
		return fmt.Errorf("after reconfigure expected worker_id 1000, got %d", p2.WorkerID)
	}
	if err := configure(c, -1); expectRPCError(err, "INVALID_PARAMS", "worker_id -1") == nil {
		return expectRPCError(err, "INVALID_PARAMS", "worker_id -1")
	}
	if err := configure(c, 2000); expectRPCError(err, "INVALID_PARAMS", "worker_id 2000") == nil {
		return expectRPCError(err, "INVALID_PARAMS", "worker_id 2000")
	}
	ctx.Logf("configure sets worker bits; parse reflects worker_id")
	return nil
}

func testSequence(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := configure(c, 7); err != nil {
		return err
	}
	seen := map[string]bool{}
	var ts int64 = -1
	for i := 0; i < 5000; i++ {
		id, err := nextID(c)
		if err != nil {
			return fmt.Errorf("next_id %d: %w", i, err)
		}
		if seen[id] {
			return fmt.Errorf("duplicate id %s at iteration %d", id, i)
		}
		seen[id] = true
		p, err := parseID(c, id)
		if err != nil {
			return err
		}
		if ts == -1 {
			ts = p.TimestampMS
		}
		if p.Sequence < 0 || p.Sequence > maxSequence {
			return fmt.Errorf("sequence %d out of range", p.Sequence)
		}
	}
	ctx.Logf("5000 rapid next_id calls — all unique, sequences in range")
	return nil
}

func testClockSkew(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const n = 512
	ids, err := batchIDs(c, n)
	if err != nil {
		return err
	}
	var ts int64 = -1
	for i, id := range ids {
		p, err := parseID(c, id)
		if err != nil {
			return err
		}
		if ts == -1 {
			ts = p.TimestampMS
		} else if p.TimestampMS != ts {
			return fmt.Errorf("batch id[%d]: timestamp %d != first %d (expected single-ms batch)", i, p.TimestampMS, ts)
		}
		if p.Sequence != int64(i) {
			return fmt.Errorf("batch id[%d]: sequence %d, want %d", i, p.Sequence, i)
		}
	}
	id, err := nextID(c)
	if err != nil {
		return err
	}
	p, err := parseID(c, id)
	if err != nil {
		return err
	}
	if p.TimestampMS < ts {
		return fmt.Errorf("clock went backwards after batch")
	}
	if p.TimestampMS == ts && p.Sequence != n {
		return fmt.Errorf("after batch of %d in one ms, next sequence %d want %d", n, p.Sequence, n)
	}
	ctx.Logf("batch of %d shared timestamp %d with sequences 0..%d", n, ts, n-1)
	return nil
}

func testBatch(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	ids, err := batchIDs(c, 100)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	var prev *big.Int
	for i, id := range ids {
		if seen[id] {
			return fmt.Errorf("duplicate id in batch at %d", i)
		}
		seen[id] = true
		n, err := idBig(id)
		if err != nil {
			return err
		}
		if prev != nil && n.Cmp(prev) <= 0 {
			return fmt.Errorf("batch ids not strictly increasing at %d", i)
		}
		prev = n
	}
	if err := c.Call("batch", map[string]any{"count": 0}, &struct{}{}); expectRPCError(err, "INVALID_PARAMS", "count 0") == nil {
		return expectRPCError(err, "INVALID_PARAMS", "count 0")
	}
	if err := c.Call("batch", map[string]any{"count": maxBatch + 1}, &struct{}{}); expectRPCError(err, "BATCH_TOO_LARGE", "count too large") == nil {
		return expectRPCError(err, "BATCH_TOO_LARGE", "count too large")
	}
	ctx.Logf("batch returned 100 unique ascending ids")
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	var issued []string
	for i := 0; i < 200; i++ {
		id, err := nextID(c)
		if err != nil {
			return err
		}
		issued = append(issued, id)
	}
	last, _ := idBig(issued[len(issued)-1])
	c.Close()

	ctx.Logf("issued %d ids; SIGKILL after brief pause", len(issued))
	time.Sleep(200 * time.Millisecond)
	ctx.KillProgram()
	if err := ctx.StartProgram(); err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	c, err = ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	for i := 0; i < 50; i++ {
		id, err := nextID(c)
		if err != nil {
			return err
		}
		n, err := idBig(id)
		if err != nil {
			return err
		}
		if n.Cmp(last) <= 0 {
			return fmt.Errorf("after restart id %s is not greater than pre-crash max %s", id, last.String())
		}
		for _, old := range issued {
			if id == old {
				return fmt.Errorf("reused id %s after restart", id)
			}
		}
	}
	ctx.Logf("post-restart ids strictly exceed pre-crash max — no reuse")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	const workers = 30
	const perWorker = 150

	seen := sync.Map{}
	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	start := make(chan struct{})

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := ctx.Dial()
			if err != nil {
				errCh <- err
				return
			}
			defer c.Close()
			<-start
			for i := 0; i < perWorker; i++ {
				id, err := nextID(c)
				if err != nil {
					errCh <- fmt.Errorf("worker next_id: %w", err)
					return
				}
				if _, loaded := seen.LoadOrStore(id, true); loaded {
					errCh <- fmt.Errorf("duplicate id %s under concurrency", id)
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
	}

	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	const ops = 3000
	const budget = 8 * time.Second
	startAt := time.Now()
	var issued atomic.Int64
	for i := 0; i < ops; i++ {
		if i%3 == 0 {
			ids, err := batchIDs(c, 10)
			if err != nil {
				return fmt.Errorf("throughput batch %d: %w", i, err)
			}
			issued.Add(int64(len(ids)))
		} else {
			if _, err := nextID(c); err != nil {
				return fmt.Errorf("throughput next_id %d: %w", i, err)
			}
			issued.Add(1)
		}
	}
	elapsed := time.Since(startAt)
	if elapsed > budget {
		return fmt.Errorf("throughput floor: %d ops took %s, over %s budget", ops, elapsed.Round(time.Millisecond), budget)
	}
	total := workers * perWorker
	ctx.Logf("concurrency: %d unique ids from %d workers; throughput %d ops in %s",
		total, workers, ops, elapsed.Round(time.Millisecond))
	return nil
}
