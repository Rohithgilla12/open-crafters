// Package distlock holds the stage tests for the "Build your own distributed
// lock" challenge. Everything is graded over the wire (NDJSON/TCP).
package distlock

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

type acquireResult struct {
	Token       string `json:"token"`
	ExpiresAtMS int64  `json:"expires_at_ms"`
}

type tryAcquireResult struct {
	Acquired    bool   `json:"acquired"`
	Token       string `json:"token,omitempty"`
	ExpiresAtMS int64  `json:"expires_at_ms,omitempty"`
}

type releaseResult struct {
	Released bool `json:"released"`
}

type renewResult struct {
	ExpiresAtMS int64 `json:"expires_at_ms"`
}

type statusResult struct {
	Held        bool   `json:"held"`
	HolderID    string `json:"holder_id,omitempty"`
	ExpiresAtMS int64  `json:"expires_at_ms,omitempty"`
	Token       string `json:"token,omitempty"`
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-distributed-lock/stages/"
	return harness.Challenge{
		Slug: "build-your-own-distributed-lock",
		Name: "Build your own distributed lock",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "acquire", Name: "Acquire a lock", Difficulty: "easy", Instructions: docs + "02-acquire.md", Test: testAcquire},
			{Slug: "release", Name: "Release a lock", Difficulty: "easy", Instructions: docs + "03-release.md", Test: testRelease},
			{Slug: "conflict", Name: "Lock contention", Difficulty: "easy", Instructions: docs + "04-conflict.md", Test: testConflict},
			{Slug: "try-acquire", Name: "Try without blocking", Difficulty: "easy", Instructions: docs + "05-try-acquire.md", Test: testTryAcquire},
			{Slug: "expiry", Name: "Lease expiry", Difficulty: "medium", Instructions: docs + "06-expiry.md", Test: testExpiry},
			{Slug: "renew", Name: "Renew a lease", Difficulty: "medium", Instructions: docs + "07-renew.md", Test: testRenew},
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

func acquire(c *harness.Client, name, holderID string, leaseMS int64) (*acquireResult, error) {
	var res acquireResult
	if err := c.Call("acquire", map[string]any{
		"name": name, "holder_id": holderID, "lease_ms": leaseMS,
	}, &res); err != nil {
		return nil, err
	}
	if res.Token == "" {
		return nil, errors.New("acquire returned empty token")
	}
	if res.ExpiresAtMS <= time.Now().UnixMilli() {
		return nil, fmt.Errorf("acquire: expires_at_ms %d is not in the future", res.ExpiresAtMS)
	}
	return &res, nil
}

func tryAcquire(c *harness.Client, name, holderID string, leaseMS int64) (*tryAcquireResult, error) {
	var res tryAcquireResult
	if err := c.Call("try_acquire", map[string]any{
		"name": name, "holder_id": holderID, "lease_ms": leaseMS,
	}, &res); err != nil {
		return nil, err
	}
	if res.Acquired && (res.Token == "" || res.ExpiresAtMS <= time.Now().UnixMilli()) {
		return nil, fmt.Errorf("try_acquire acquired=true but token/expires_at_ms missing or stale")
	}
	return &res, nil
}

func release(c *harness.Client, name, token string) (bool, error) {
	var res releaseResult
	if err := c.Call("release", map[string]any{"name": name, "token": token}, &res); err != nil {
		return false, err
	}
	return res.Released, nil
}

func renew(c *harness.Client, name, token string, leaseMS int64) (int64, error) {
	var res renewResult
	if err := c.Call("renew", map[string]any{
		"name": name, "token": token, "lease_ms": leaseMS,
	}, &res); err != nil {
		return 0, err
	}
	return res.ExpiresAtMS, nil
}

func status(c *harness.Client, name string) (*statusResult, error) {
	var res statusResult
	if err := c.Call("status", map[string]any{"name": name}, &res); err != nil {
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

func testAcquire(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	res, err := acquire(c, "jobs", "worker-1", 5000)
	if err != nil {
		return err
	}
	st, err := status(c, "jobs")
	if err != nil {
		return err
	}
	if !st.Held {
		return fmt.Errorf("status after acquire: expected held=true")
	}
	if st.HolderID != "worker-1" {
		return fmt.Errorf("status: expected holder_id worker-1, got %q", st.HolderID)
	}
	if st.Token != res.Token {
		return fmt.Errorf("status token %q != acquire token %q", st.Token, res.Token)
	}
	if st.ExpiresAtMS != res.ExpiresAtMS {
		return fmt.Errorf("status expires_at_ms %d != acquire %d", st.ExpiresAtMS, res.ExpiresAtMS)
	}
	if _, err := acquire(c, "", "x", 100); expectRPCError(err, "INVALID_PARAMS", "acquire missing name") == nil {
		return expectRPCError(err, "INVALID_PARAMS", "acquire missing name")
	}
	ctx.Logf("acquire granted token; status reports held")
	return nil
}

func testRelease(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	res, err := acquire(c, "rel", "h1", 5000)
	if err != nil {
		return err
	}
	ok, err := release(c, "rel", res.Token)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("release with valid token: expected released=true")
	}
	st, err := status(c, "rel")
	if err != nil {
		return err
	}
	if st.Held {
		return fmt.Errorf("status after release: expected held=false")
	}
	ok, err = release(c, "rel", res.Token)
	if err != nil {
		return err
	}
	if ok {
		return fmt.Errorf("release after already released: expected released=false")
	}
	ctx.Logf("release cleared the lock; stale token returns released=false")
	return nil
}

func testConflict(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := acquire(c, "shared", "alpha", 5000); err != nil {
		return err
	}
	if _, err := acquire(c, "shared", "beta", 5000); expectRPCError(err, "LOCK_HELD", "second acquire") != nil {
		return expectRPCError(err, "LOCK_HELD", "second acquire")
	}
	st, err := status(c, "shared")
	if err != nil {
		return err
	}
	if !st.Held || st.HolderID != "alpha" {
		return fmt.Errorf("lock should still be held by alpha, got held=%v holder=%q", st.Held, st.HolderID)
	}
	ctx.Logf("contended acquire returned LOCK_HELD; original holder unchanged")
	return nil
}

func testTryAcquire(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := acquire(c, "try", "owner", 5000); err != nil {
		return err
	}
	r, err := tryAcquire(c, "try", "other", 5000)
	if err != nil {
		return err
	}
	if r.Acquired {
		return fmt.Errorf("try_acquire while held: expected acquired=false")
	}
	r, err = tryAcquire(c, "try", "owner", 5000)
	if err != nil {
		return err
	}
	if r.Acquired {
		return fmt.Errorf("try_acquire by current holder while held: expected acquired=false")
	}
	ctx.Logf("try_acquire returns acquired=false under contention without error")
	return nil
}

func testExpiry(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := acquire(c, "ttl", "first", 300); err != nil {
		return err
	}
	time.Sleep(400 * time.Millisecond)
	res, err := acquire(c, "ttl", "second", 5000)
	if err != nil {
		return fmt.Errorf("acquire after lease expiry: %w", err)
	}
	st, err := status(c, "ttl")
	if err != nil {
		return err
	}
	if !st.Held || st.HolderID != "second" {
		return fmt.Errorf("after expiry re-acquire: expected holder second, got held=%v holder=%q", st.Held, st.HolderID)
	}
	if st.Token != res.Token {
		return fmt.Errorf("status token mismatch after re-acquire")
	}
	ctx.Logf("expired lease freed the lock for a new holder")
	return nil
}

func testRenew(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	res, err := acquire(c, "renew-me", "holder", 2000)
	if err != nil {
		return err
	}
	before := res.ExpiresAtMS
	newExp, err := renew(c, "renew-me", res.Token, 3000)
	if err != nil {
		return err
	}
	if newExp <= before {
		return fmt.Errorf("renew: expected expires_at_ms > %d, got %d", before, newExp)
	}
	if newExp < time.Now().UnixMilli()+2500 {
		return fmt.Errorf("renew should extend lease by ~3000ms from max(now, current_expires), got expires_at_ms=%d", newExp)
	}
	if _, err := renew(c, "renew-me", "wrong-token", 1000); expectRPCError(err, "NOT_HOLDER", "renew wrong token") != nil {
		return expectRPCError(err, "NOT_HOLDER", "renew wrong token")
	}
	if _, err := renew(c, "renew-me", res.Token, 0); expectRPCError(err, "INVALID_PARAMS", "renew lease_ms=0") != nil {
		return expectRPCError(err, "INVALID_PARAMS", "renew lease_ms=0")
	}
	ctx.Logf("renew extended lease; wrong token returned NOT_HOLDER")
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	res, err := acquire(c, "durable", "survivor", 60000)
	if err != nil {
		return err
	}
	c.Close()

	ctx.Logf("lock acquired; SIGKILL after brief pause")
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

	st, err := status(c, "durable")
	if err != nil {
		return err
	}
	if !st.Held {
		return fmt.Errorf("after restart: lock should still be held (unexpired lease)")
	}
	if st.HolderID != "survivor" || st.Token != res.Token {
		return fmt.Errorf("after restart: holder/token mismatch (got holder=%q token=%q)", st.HolderID, st.Token)
	}
	ok, err := release(c, "durable", res.Token)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("release after restart: expected released=true")
	}
	st, err = status(c, "durable")
	if err != nil {
		return err
	}
	if st.Held {
		return fmt.Errorf("status after release post-restart: expected held=false")
	}
	ctx.Logf("unexpired lock survived SIGKILL; release worked after restart")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	const (
		locks   = 5
		workers = 25
		rounds  = 3
	)

	for round := 1; round <= rounds; round++ {
		var wg sync.WaitGroup
		errCh := make(chan error, workers)
		start := make(chan struct{})
		type winner struct {
			holder string
			token  string
		}
		winners := make([]*winner, locks)
		var mu sync.Mutex

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				c, err := ctx.Dial()
				if err != nil {
					errCh <- err
					return
				}
				defer c.Close()
				<-start
				lockIdx := id % locks
				name := fmt.Sprintf("g-%d", lockIdx)
				holder := fmt.Sprintf("w-%d-r%d", id, round)
				res, err := acquire(c, name, holder, 15000)
				if err != nil {
					var rpcErr *harness.RPCError
					if errors.As(err, &rpcErr) && rpcErr.Code == "LOCK_HELD" {
						return
					}
					errCh <- fmt.Errorf("worker %d acquire %s: %w", id, name, err)
					return
				}
				mu.Lock()
				if winners[lockIdx] != nil {
					mu.Unlock()
					errCh <- fmt.Errorf("two workers acquired lock %s in round %d", name, round)
					return
				}
				winners[lockIdx] = &winner{holder: holder, token: res.Token}
				mu.Unlock()
			}(w)
		}
		close(start)
		wg.Wait()
		select {
		case err := <-errCh:
			return err
		default:
		}

		held := 0
		for i := 0; i < locks; i++ {
			if winners[i] != nil {
				held++
			}
		}
		if held != locks {
			return fmt.Errorf("round %d: expected exactly one holder per lock (%d/%d locks held)", round, held, locks)
		}

		var relWg sync.WaitGroup
		for i := 0; i < locks; i++ {
			w := winners[i]
			relWg.Add(1)
			go func(lockIdx int, win *winner) {
				defer relWg.Done()
				c, err := ctx.Dial()
				if err != nil {
					errCh <- err
					return
				}
				defer c.Close()
				name := fmt.Sprintf("g-%d", lockIdx)
				ok, err := release(c, name, win.token)
				if err != nil {
					errCh <- err
					return
				}
				if !ok {
					errCh <- fmt.Errorf("round %d: release %s failed", round, name)
				}
			}(i, w)
		}
		relWg.Wait()
		select {
		case err := <-errCh:
			return err
		default:
		}
		ctx.Logf("round %d: %d locks acquired and released under contention", round, locks)
	}

	// Hot path: many try_acquire / release cycles must stay responsive.
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	const ops = 2000
	const budget = 8 * time.Second
	start := time.Now()
	var acquired atomic.Int64
	for i := 0; i < ops; i++ {
		name := fmt.Sprintf("hot-%d", i%20)
		r, err := tryAcquire(c, name, "bench", 60000)
		if err != nil {
			return fmt.Errorf("throughput try_acquire %d: %w", i, err)
		}
		if r.Acquired {
			acquired.Add(1)
			if _, err := release(c, name, r.Token); err != nil {
				return fmt.Errorf("throughput release %d: %w", i, err)
			}
		}
	}
	elapsed := time.Since(start)
	if elapsed > budget {
		return fmt.Errorf("throughput floor: %d try_acquire/release cycles took %s, over %s budget", ops, elapsed.Round(time.Millisecond), budget)
	}
	ctx.Logf("throughput: %d ops in %s (%d acquisitions)", ops, elapsed.Round(time.Millisecond), acquired.Load())
	return nil
}
