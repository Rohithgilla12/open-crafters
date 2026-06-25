// Package ratelimiter holds the stage tests for the "Build your own rate
// limiter" challenge. Everything is graded over the wire (NDJSON/TCP); the
// tester never reads the submission's code.
package ratelimiter

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

const (
	pollInterval = 20 * time.Millisecond
)

type takeResult struct {
	Allowed      bool  `json:"allowed"`
	Remaining    int   `json:"remaining"`
	Limit        int   `json:"limit"`
	RetryAfterMS int64 `json:"retry_after_ms"`
}

type peekResult struct {
	Remaining    int   `json:"remaining"`
	Limit        int   `json:"limit"`
	RetryAfterMS int64 `json:"retry_after_ms"`
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-rate-limiter/stages/"
	return harness.Challenge{
		Slug: "build-your-own-rate-limiter",
		Name: "Build your own rate limiter",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "fixed-window", Name: "Fixed-window counter", Difficulty: "easy", Instructions: docs + "02-fixed-window.md", Test: testFixedWindow},
			{Slug: "token-bucket", Name: "Token bucket", Difficulty: "medium", Instructions: docs + "03-token-bucket.md", Test: testTokenBucket},
			{Slug: "sliding-window", Name: "Sliding window", Difficulty: "medium", Instructions: docs + "04-sliding-window.md", Test: testSlidingWindow},
			{Slug: "multi-key", Name: "Independent keys", Difficulty: "easy", Instructions: docs + "05-multi-key.md", Test: testMultiKey},
			{Slug: "peek", Name: "Peek without consuming", Difficulty: "medium", Instructions: docs + "06-peek.md", Test: testPeek},
			{Slug: "concurrency", Name: "Atomic admission", Difficulty: "hard", Instructions: docs + "07-concurrency.md", Test: testConcurrency},
			{Slug: "durability", Name: "Survive a crash", Difficulty: "hard", Instructions: docs + "08-durability.md", Test: testDurability},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", Test: testGauntlet},
		},
	}
}

// --- protocol helpers ---------------------------------------------------

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

func configure(c *harness.Client, params map[string]any) error {
	return c.Call("configure", params, nil)
}

// take calls take for key with the given cost; cost <= 0 omits the field so
// the server applies its default of 1.
func take(c *harness.Client, key string, cost int) (*takeResult, error) {
	p := map[string]any{"key": key}
	if cost > 0 {
		p["cost"] = cost
	}
	var r takeResult
	if err := c.Call("take", p, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func peek(c *harness.Client, key string, cost int) (*peekResult, error) {
	p := map[string]any{"key": key}
	if cost > 0 {
		p["cost"] = cost
	}
	var r peekResult
	if err := c.Call("peek", p, &r); err != nil {
		return nil, err
	}
	return &r, nil
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

// alignToWindowStart sleeps until just past a fixed-window boundary so a burst
// fired immediately after cannot straddle two windows. Fixed windows are
// aligned to the epoch (floor(now/window_ms)).
func alignToWindowStart(windowMS int64) {
	now := time.Now().UnixMilli()
	rem := windowMS - (now % windowMS)
	time.Sleep(time.Duration(rem+15) * time.Millisecond)
}

// --- stage tests --------------------------------------------------------

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

func testFixedWindow(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const limit, windowMS = 3, 1000
	if err := configure(c, map[string]any{
		"key": "api", "algorithm": "fixed_window", "limit": limit, "window_ms": windowMS,
	}); err != nil {
		return err
	}

	// Start at a fresh window boundary so the burst can't straddle two windows.
	alignToWindowStart(windowMS)
	for i := 1; i <= limit; i++ {
		r, err := take(c, "api", 0)
		if err != nil {
			return err
		}
		if !r.Allowed {
			return fmt.Errorf("take %d/%d should be allowed within the window limit, got allowed=false", i, limit)
		}
		if r.Limit != limit {
			return fmt.Errorf("take %d: expected limit=%d, got %d", i, limit, r.Limit)
		}
		if want := limit - i; r.Remaining != want {
			return fmt.Errorf("take %d: expected remaining=%d, got %d", i, want, r.Remaining)
		}
	}
	r, err := take(c, "api", 0)
	if err != nil {
		return err
	}
	if r.Allowed {
		return fmt.Errorf("take %d should be denied: only %d allowed per %dms window", limit+1, limit, windowMS)
	}
	if r.RetryAfterMS <= 0 || r.RetryAfterMS > windowMS+200 {
		return fmt.Errorf("denied take: expected 0 < retry_after_ms ≤ %d (until window ends), got %d", windowMS+200, r.RetryAfterMS)
	}
	ctx.Logf("%d takes admitted, overflow denied with retry_after_ms=%d", limit, r.RetryAfterMS)

	// New window → budget resets.
	alignToWindowStart(windowMS)
	r, err = take(c, "api", 0)
	if err != nil {
		return err
	}
	if !r.Allowed {
		return fmt.Errorf("take in a fresh window should be allowed (counter must reset at the window boundary)")
	}
	ctx.Logf("counter reset in the next window")
	return nil
}

func testTokenBucket(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const capacity, refillTokens, refillMS = 4, 1, 200
	if err := configure(c, map[string]any{
		"key": "tb", "algorithm": "token_bucket",
		"capacity": capacity, "refill_tokens": refillTokens, "refill_interval_ms": refillMS,
	}); err != nil {
		return err
	}

	// Bucket starts full: a single cost=capacity take drains it.
	r, err := take(c, "tb", capacity)
	if err != nil {
		return err
	}
	if !r.Allowed {
		return fmt.Errorf("cost=%d take on a full bucket of capacity %d should be allowed", capacity, capacity)
	}
	if r.Remaining != 0 {
		return fmt.Errorf("after draining: expected remaining=0, got %d", r.Remaining)
	}
	if r.Limit != capacity {
		return fmt.Errorf("expected limit=%d (capacity), got %d", capacity, r.Limit)
	}

	// Empty now (only microseconds have passed): next take denied.
	r, err = take(c, "tb", 1)
	if err != nil {
		return err
	}
	if r.Allowed {
		return fmt.Errorf("take on a freshly drained bucket should be denied")
	}
	if r.RetryAfterMS <= 0 || r.RetryAfterMS > 2*refillMS {
		return fmt.Errorf("denied take: expected retry_after_ms for ~1 token (≈%dms), got %d", refillMS, r.RetryAfterMS)
	}
	ctx.Logf("bucket drained, take denied with retry_after_ms=%d", r.RetryAfterMS)

	// Wait one refill interval + slack; ~1 token should have accrued.
	time.Sleep(time.Duration(refillMS+80) * time.Millisecond)
	r, err = take(c, "tb", 1)
	if err != nil {
		return err
	}
	if !r.Allowed {
		return fmt.Errorf("after ~%dms a refilled token should be available, but take was denied", refillMS)
	}
	ctx.Logf("token refilled after %dms and take admitted", refillMS)
	return nil
}

func testSlidingWindow(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const limit, windowMS = 3, 1200
	if err := configure(c, map[string]any{
		"key": "sw", "algorithm": "sliding_window", "limit": limit, "window_ms": windowMS,
	}); err != nil {
		return err
	}

	start := time.Now()
	for i := 1; i <= limit; i++ {
		r, err := take(c, "sw", 0)
		if err != nil {
			return err
		}
		if !r.Allowed {
			return fmt.Errorf("take %d/%d should be allowed, got allowed=false", i, limit)
		}
	}
	r, err := take(c, "sw", 0)
	if err != nil {
		return err
	}
	if r.Allowed {
		return fmt.Errorf("take %d should be denied: %d already admitted in the trailing %dms", limit+1, limit, windowMS)
	}
	ctx.Logf("%d admitted, overflow denied (retry_after_ms=%d)", limit, r.RetryAfterMS)

	// Half a window later the earliest admissions are still inside the trailing
	// window, so a fresh take must still be denied — unlike a fixed window,
	// there is no boundary at which the count resets.
	time.Sleep(500 * time.Millisecond)
	r, err = take(c, "sw", 0)
	if err != nil {
		return err
	}
	if r.Allowed {
		return fmt.Errorf("take ~500ms later should still be denied: the %d admissions are still within the trailing %dms (no boundary reset)", limit, windowMS)
	}
	ctx.Logf("still denied mid-window (no boundary burst)")

	// Once the original admissions age past window_ms, capacity returns.
	time.Sleep(time.Until(start.Add(time.Duration(windowMS+250) * time.Millisecond)))
	r, err = take(c, "sw", 0)
	if err != nil {
		return err
	}
	if !r.Allowed {
		return fmt.Errorf("after the trailing window cleared (~%dms) a take should be allowed again", windowMS)
	}
	ctx.Logf("capacity returned after the window slid past the old admissions")
	return nil
}

func testMultiKey(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// A never-refilling token bucket and an effectively non-resetting window.
	if err := configure(c, map[string]any{
		"key": "a", "algorithm": "token_bucket", "capacity": 2, "refill_tokens": 1, "refill_interval_ms": 600000,
	}); err != nil {
		return err
	}
	if err := configure(c, map[string]any{
		"key": "b", "algorithm": "fixed_window", "limit": 2, "window_ms": 600000,
	}); err != nil {
		return err
	}

	// Drain key a.
	for i := 1; i <= 2; i++ {
		r, err := take(c, "a", 0)
		if err != nil {
			return err
		}
		if !r.Allowed {
			return fmt.Errorf("key a take %d should be allowed", i)
		}
	}
	r, err := take(c, "a", 0)
	if err != nil {
		return err
	}
	if r.Allowed {
		return fmt.Errorf("key a is exhausted; take should be denied")
	}

	// Key b must be untouched by key a's exhaustion.
	r, err = take(c, "b", 0)
	if err != nil {
		return err
	}
	if !r.Allowed || r.Remaining != 1 {
		return fmt.Errorf("key b should be independent of key a: expected allowed with remaining=1, got allowed=%v remaining=%d", r.Allowed, r.Remaining)
	}
	ctx.Logf("keys are independent")

	// Reconfigure resets consumption.
	if err := configure(c, map[string]any{
		"key": "a", "algorithm": "token_bucket", "capacity": 2, "refill_tokens": 1, "refill_interval_ms": 600000,
	}); err != nil {
		return err
	}
	r, err = take(c, "a", 0)
	if err != nil {
		return err
	}
	if !r.Allowed || r.Remaining != 1 {
		return fmt.Errorf("reconfigure should reset key a to full: expected allowed with remaining=1, got allowed=%v remaining=%d", r.Allowed, r.Remaining)
	}
	ctx.Logf("reconfigure reset the limiter")

	// Unknown keys error.
	if _, err := take(c, "ghost", 0); expectRPCError(err, "KEY_NOT_FOUND", "take on unconfigured key") != nil {
		return expectRPCError(err, "KEY_NOT_FOUND", "take on unconfigured key")
	}
	if _, err := peek(c, "ghost", 0); expectRPCError(err, "KEY_NOT_FOUND", "peek on unconfigured key") != nil {
		return expectRPCError(err, "KEY_NOT_FOUND", "peek on unconfigured key")
	}
	ctx.Logf("unknown keys return KEY_NOT_FOUND")
	return nil
}

func testPeek(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const capacity, refillMS = 3, 200
	if err := configure(c, map[string]any{
		"key": "p", "algorithm": "token_bucket", "capacity": capacity, "refill_tokens": 1, "refill_interval_ms": refillMS,
	}); err != nil {
		return err
	}

	// Peek does not consume.
	p1, err := peek(c, "p", 1)
	if err != nil {
		return err
	}
	if p1.Remaining != capacity || p1.RetryAfterMS != 0 {
		return fmt.Errorf("peek on full bucket: expected remaining=%d retry_after_ms=0, got remaining=%d retry_after_ms=%d", capacity, p1.Remaining, p1.RetryAfterMS)
	}
	p2, err := peek(c, "p", 1)
	if err != nil {
		return err
	}
	if p2.Remaining != capacity {
		return fmt.Errorf("two peeks with no take between them must agree: got %d then %d", p1.Remaining, p2.Remaining)
	}
	ctx.Logf("peek does not consume (remaining stayed at %d)", capacity)

	// Drain, then peek must report empty with a usable retry hint.
	r, err := take(c, "p", capacity)
	if err != nil {
		return err
	}
	if !r.Allowed {
		return fmt.Errorf("cost=%d take on full bucket should be allowed", capacity)
	}
	pk, err := peek(c, "p", 1)
	if err != nil {
		return err
	}
	if pk.Remaining != 0 {
		return fmt.Errorf("peek after drain: expected remaining=0, got %d", pk.Remaining)
	}
	if pk.RetryAfterMS <= 0 || pk.RetryAfterMS > 2*refillMS {
		return fmt.Errorf("peek after drain: expected retry_after_ms ≈ %dms for one token, got %d", refillMS, pk.RetryAfterMS)
	}

	// The retry hint must be a usable lower bound: sleeping it (plus slack) and
	// retrying succeeds.
	time.Sleep(time.Duration(pk.RetryAfterMS+80) * time.Millisecond)
	r, err = take(c, "p", 1)
	if err != nil {
		return err
	}
	if !r.Allowed {
		return fmt.Errorf("after waiting peek.retry_after_ms (%dms)+slack, take should be allowed", pk.RetryAfterMS)
	}
	ctx.Logf("peek.retry_after_ms was a usable lower bound")
	return nil
}

func testConcurrency(ctx *harness.Context) error {
	const capacity = 50
	const conns = 10
	const perConn = 20 // 200 attempts total, far above capacity

	// A bucket that does not meaningfully refill over the test's lifetime, so
	// the admitted count must equal capacity exactly.
	setup, err := ctx.Dial()
	if err != nil {
		return err
	}
	if err := configure(setup, map[string]any{
		"key": "race", "algorithm": "token_bucket", "capacity": capacity, "refill_tokens": 1, "refill_interval_ms": 600000,
	}); err != nil {
		setup.Close()
		return err
	}
	setup.Close()

	var allowed atomic.Int64
	var wg sync.WaitGroup
	errCh := make(chan error, conns)
	start := make(chan struct{})
	for i := 0; i < conns; i++ {
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
			for j := 0; j < perConn; j++ {
				r, err := take(c, "race", 1)
				if err != nil {
					errCh <- err
					return
				}
				if r.Allowed {
					allowed.Add(1)
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

	if got := allowed.Load(); got != capacity {
		return fmt.Errorf("expected exactly %d admissions under %d concurrent takers, got %d — check-and-consume must be atomic (no over- or under-admission)", capacity, conns, got)
	}
	ctx.Logf("exactly %d admitted across %d connections firing %d takes each", capacity, conns, perConn)
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const capacity = 5
	if err := configure(c, map[string]any{
		"key": "d", "algorithm": "token_bucket", "capacity": capacity, "refill_tokens": 1, "refill_interval_ms": 60000,
	}); err != nil {
		return err
	}
	r, err := take(c, "d", capacity)
	if err != nil {
		return err
	}
	if !r.Allowed || r.Remaining != 0 {
		return fmt.Errorf("draining the bucket: expected allowed with remaining=0, got allowed=%v remaining=%d", r.Allowed, r.Remaining)
	}
	c.Close()

	ctx.Logf("bucket drained; SIGKILL after a brief pause")
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

	// Config must have survived.
	pk, err := peek(c, "d", 1)
	if err != nil {
		return fmt.Errorf("peek after restart (config should have survived): %w", err)
	}
	if pk.Limit != capacity {
		return fmt.Errorf("after restart: expected limit=%d (config persisted), got %d", capacity, pk.Limit)
	}

	// Consumption must have survived: the bucket was empty and barely any time
	// has passed, so a take must be denied — a restart must not refill it.
	r, err = take(c, "d", 1)
	if err != nil {
		return err
	}
	if r.Allowed {
		return fmt.Errorf("after restart the drained bucket allowed a take — consumption was not persisted (a crash must not hand out a free burst)")
	}
	ctx.Logf("drained bucket stayed drained across the crash; config intact")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// A mix of algorithms, plus a slow bucket we'll drain and expect to survive.
	configs := []map[string]any{
		{"key": "g-fw", "algorithm": "fixed_window", "limit": 5, "window_ms": 60000},
		{"key": "g-sw", "algorithm": "sliding_window", "limit": 5, "window_ms": 60000},
		{"key": "g-tb", "algorithm": "token_bucket", "capacity": 5, "refill_tokens": 1, "refill_interval_ms": 60000},
	}
	for _, cfg := range configs {
		if err := configure(c, cfg); err != nil {
			return err
		}
	}
	// Drain the persistent bucket.
	r, err := take(c, "g-tb", 5)
	if err != nil {
		return err
	}
	if !r.Allowed {
		return fmt.Errorf("draining g-tb should be allowed")
	}
	// Spend some of g-fw.
	for i := 0; i < 3; i++ {
		if _, err := take(c, "g-fw", 0); err != nil {
			return err
		}
	}

	c.Close()
	ctx.KillProgram()
	if err := ctx.StartProgram(); err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	c, err = ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// Drained bucket stays drained.
	r, err = take(c, "g-tb", 1)
	if err != nil {
		return err
	}
	if r.Allowed {
		return fmt.Errorf("g-tb allowed a take after restart — drained state was not persisted")
	}
	// Window configs survived.
	for _, key := range []string{"g-fw", "g-sw"} {
		pk, err := peek(c, key, 1)
		if err != nil {
			return fmt.Errorf("peek %s after restart: %w", key, err)
		}
		if pk.Limit != 5 {
			return fmt.Errorf("%s after restart: expected limit=5, got %d", key, pk.Limit)
		}
	}
	ctx.Logf("state survived the crash across all three algorithms")

	// Performance tier: a generous throughput floor on the hot path. A
	// pathological design (fsync per take, O(n) rescans, one global lock)
	// will not clear it; any reasonable one clears it comfortably.
	if err := configure(c, map[string]any{
		"key": "hot", "algorithm": "token_bucket", "capacity": 100000000, "refill_tokens": 100000000, "refill_interval_ms": 1,
	}); err != nil {
		return err
	}
	const ops = 3000
	const budget = 10 * time.Second
	start := time.Now()
	for i := 0; i < ops; i++ {
		r, err := take(c, "hot", 1)
		if err != nil {
			return fmt.Errorf("throughput take %d: %w", i, err)
		}
		if !r.Allowed {
			return fmt.Errorf("throughput take %d unexpectedly denied (bucket is effectively unlimited)", i)
		}
	}
	elapsed := time.Since(start)
	if elapsed > budget {
		return fmt.Errorf("throughput floor: %d takes took %s, over the %s budget (≈%.0f ops/s) — avoid fsync-per-take, O(n) rescans, and global locks", ops, elapsed.Round(time.Millisecond), budget, float64(ops)/elapsed.Seconds())
	}
	ctx.Logf("throughput: %d takes in %s (≈%.0f ops/s)", ops, elapsed.Round(time.Millisecond), float64(ops)/elapsed.Seconds())
	return nil
}
