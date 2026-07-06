// Package distcache implements stage tests for the "Build your own distributed
// cache" challenge. See challenges/build-your-own-distributed-cache/PROTOCOL.md.
package distcache

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-distributed-cache/stages/"
	return harness.Challenge{
		Slug: "build-your-own-distributed-cache",
		Name: "Build your own distributed cache",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "set-get", Name: "Set and get", Difficulty: "easy", Instructions: docs + "02-set-get.md", Test: testSetGet},
			{Slug: "delete", Name: "Delete a key", Difficulty: "easy", Instructions: docs + "03-delete.md", Test: testDelete},
			{Slug: "ttl", Name: "TTL expiration", Difficulty: "medium", Instructions: docs + "04-ttl.md", Test: testTTL},
			{Slug: "setnx", Name: "Set if not exists", Difficulty: "medium", Instructions: docs + "05-setnx.md", Test: testSetnx},
			{Slug: "cas", Name: "Compare and swap", Difficulty: "medium", Instructions: docs + "06-cas.md", Test: testCas},
			{Slug: "mget", Name: "Multi-get", Difficulty: "easy", Instructions: docs + "07-mget.md", Test: testMget},
			{Slug: "lru", Name: "LRU eviction", Difficulty: "hard", Instructions: docs + "08-lru.md", Test: testLRU},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", Test: testGauntlet},
		},
	}
}

type getResult struct {
	Hit     bool   `json:"hit"`
	Value   string `json:"value"`
	Version int    `json:"version"`
}

type setResult struct {
	Version int `json:"version"`
}

type deleteResult struct {
	Deleted bool `json:"deleted"`
}

type setnxResult struct {
	Stored  bool `json:"stored"`
	Version int  `json:"version"`
}

type casResult struct {
	Swapped bool `json:"swapped"`
	Version int  `json:"version"`
}

type mgetEntry struct {
	Key     string `json:"key"`
	Hit     bool   `json:"hit"`
	Value   string `json:"value"`
	Version int    `json:"version"`
}

type mgetResult struct {
	Entries []mgetEntry `json:"entries"`
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

func configure(c *harness.Client, maxKeys int) error {
	return c.Call("configure", map[string]any{"max_keys": maxKeys}, nil)
}

func cacheSet(c *harness.Client, key, value string, ttlMS int) (*setResult, error) {
	p := map[string]any{"key": key, "value": value}
	if ttlMS > 0 {
		p["ttl_ms"] = ttlMS
	}
	var r setResult
	if err := c.Call("set", p, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func cacheGet(c *harness.Client, key string) (*getResult, error) {
	var r getResult
	if err := c.Call("get", map[string]any{"key": key}, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func cacheDelete(c *harness.Client, key string) (*deleteResult, error) {
	var r deleteResult
	if err := c.Call("delete", map[string]any{"key": key}, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func cacheSetnx(c *harness.Client, key, value string, ttlMS int) (*setnxResult, error) {
	p := map[string]any{"key": key, "value": value}
	if ttlMS > 0 {
		p["ttl_ms"] = ttlMS
	}
	var r setnxResult
	if err := c.Call("setnx", p, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func cacheCas(c *harness.Client, key string, expectedVersion int, value string, ttlMS int) (*casResult, error) {
	p := map[string]any{"key": key, "expected_version": expectedVersion, "value": value}
	if ttlMS > 0 {
		p["ttl_ms"] = ttlMS
	}
	var r casResult
	if err := c.Call("cas", p, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func cacheMget(c *harness.Client, keys []string) (*mgetResult, error) {
	var r mgetResult
	if err := c.Call("mget", map[string]any{"keys": keys}, &r); err != nil {
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

func testSetGet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	miss, err := cacheGet(c, "missing")
	if err != nil {
		return err
	}
	if miss.Hit {
		return fmt.Errorf("get missing key: expected hit=false")
	}

	sr, err := cacheSet(c, "user:1", "alice", 0)
	if err != nil {
		return err
	}
	if sr.Version != 1 {
		return fmt.Errorf("first set: expected version=1, got %d", sr.Version)
	}
	gr, err := cacheGet(c, "user:1")
	if err != nil {
		return err
	}
	if !gr.Hit || gr.Value != "alice" || gr.Version != 1 {
		return fmt.Errorf("get after set: hit=%v value=%q version=%d", gr.Hit, gr.Value, gr.Version)
	}

	sr2, err := cacheSet(c, "user:1", "bob", 0)
	if err != nil {
		return err
	}
	if sr2.Version != 2 {
		return fmt.Errorf("overwrite set: expected version=2, got %d", sr2.Version)
	}
	gr2, err := cacheGet(c, "user:1")
	if err != nil || !gr2.Hit || gr2.Value != "bob" || gr2.Version != 2 {
		return fmt.Errorf("get after overwrite: hit=%v value=%q version=%d", gr2.Hit, gr2.Value, gr2.Version)
	}

	if err := c.Call("set", map[string]any{"key": "bad"}, nil); expectRPCError(err, "INVALID_PARAMS", "set missing value") != nil {
		return expectRPCError(err, "INVALID_PARAMS", "set missing value")
	}
	ctx.Logf("set/get and version bumps verified")
	return nil
}

func testDelete(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	d0, err := cacheDelete(c, "ghost")
	if err != nil || d0.Deleted {
		return fmt.Errorf("delete missing key: deleted=%v err=%v", d0.Deleted, err)
	}
	if _, err := cacheSet(c, "temp", "x", 0); err != nil {
		return err
	}
	d1, err := cacheDelete(c, "temp")
	if err != nil || !d1.Deleted {
		return fmt.Errorf("delete existing key: deleted=%v err=%v", d1.Deleted, err)
	}
	gr, err := cacheGet(c, "temp")
	if err != nil || gr.Hit {
		return fmt.Errorf("get after delete: expected miss, hit=%v", gr.Hit)
	}
	d2, err := cacheDelete(c, "temp")
	if err != nil || d2.Deleted {
		return fmt.Errorf("delete again: expected deleted=false")
	}
	ctx.Logf("delete removes keys and is idempotent on misses")
	return nil
}

func testTTL(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const ttlMS = 200
	if _, err := cacheSet(c, "session", "token", ttlMS); err != nil {
		return err
	}
	gr, err := cacheGet(c, "session")
	if err != nil || !gr.Hit || gr.Value != "token" {
		return fmt.Errorf("immediate get: expected hit with token")
	}
	time.Sleep(time.Duration(ttlMS+80) * time.Millisecond)
	gr2, err := cacheGet(c, "session")
	if err != nil {
		return err
	}
	if gr2.Hit {
		return fmt.Errorf("get after ttl_ms=%d + sleep: expected hit=false (expired)", ttlMS)
	}
	ctx.Logf("key expired after ttl_ms=%d", ttlMS)
	return nil
}

func testSetnx(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	s1, err := cacheSetnx(c, "lock:job", "owner-a", 0)
	if err != nil || !s1.Stored || s1.Version != 1 {
		return fmt.Errorf("first setnx: stored=%v version=%d", s1.Stored, s1.Version)
	}
	s2, err := cacheSetnx(c, "lock:job", "owner-b", 0)
	if err != nil || s2.Stored {
		return fmt.Errorf("second setnx on same key: expected stored=false")
	}
	gr, err := cacheGet(c, "lock:job")
	if err != nil || !gr.Hit || gr.Value != "owner-a" {
		return fmt.Errorf("value must remain owner-a after failed setnx")
	}
	if _, err := cacheDelete(c, "lock:job"); err != nil {
		return err
	}
	s3, err := cacheSetnx(c, "lock:job", "owner-c", 0)
	if err != nil || !s3.Stored {
		return fmt.Errorf("setnx after delete: expected stored=true")
	}
	ctx.Logf("setnx is atomic create-only")
	return nil
}

func testCas(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	sr, err := cacheSet(c, "counter", "v1", 0)
	if err != nil || sr.Version != 1 {
		return err
	}
	ok, err := cacheCas(c, "counter", 1, "v2", 0)
	if err != nil || !ok.Swapped || ok.Version != 2 {
		return fmt.Errorf("cas with matching version: swapped=%v version=%d", ok.Swapped, ok.Version)
	}
	bad, err := cacheCas(c, "counter", 1, "v3", 0)
	if err != nil || bad.Swapped {
		return fmt.Errorf("stale cas: expected swapped=false")
	}
	gr, err := cacheGet(c, "counter")
	if err != nil || !gr.Hit || gr.Value != "v2" || gr.Version != 2 {
		return fmt.Errorf("value after stale cas attempt: got %q v%d", gr.Value, gr.Version)
	}
	miss, err := cacheCas(c, "absent", 1, "x", 0)
	if err != nil || miss.Swapped {
		return fmt.Errorf("cas on missing key: expected swapped=false")
	}
	ctx.Logf("cas swaps only on matching version")
	return nil
}

func testMget(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := cacheSet(c, "a", "1", 0); err != nil {
		return err
	}
	if _, err := cacheSet(c, "c", "3", 0); err != nil {
		return err
	}
	mg, err := cacheMget(c, []string{"a", "b", "c"})
	if err != nil {
		return err
	}
	if len(mg.Entries) != 3 {
		return fmt.Errorf("mget: expected 3 entries, got %d", len(mg.Entries))
	}
	want := []struct {
		key string
		hit bool
		val string
	}{
		{"a", true, "1"},
		{"b", false, ""},
		{"c", true, "3"},
	}
	for i, w := range want {
		e := mg.Entries[i]
		if e.Key != w.key || e.Hit != w.hit || (w.hit && e.Value != w.val) {
			return fmt.Errorf("entries[%d]: got key=%q hit=%v value=%q, want key=%q hit=%v value=%q",
				i, e.Key, e.Hit, e.Value, w.key, w.hit, w.val)
		}
	}
	if err := c.Call("mget", map[string]any{"keys": []string{}}, nil); expectRPCError(err, "INVALID_PARAMS", "empty keys") != nil {
		return expectRPCError(err, "INVALID_PARAMS", "empty keys")
	}
	ctx.Logf("mget preserves key order and reports misses")
	return nil
}

func testLRU(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := configure(c, 3); err != nil {
		return err
	}
	for _, kv := range []struct{ k, v string }{
		{"k1", "v1"}, {"k2", "v2"}, {"k3", "v3"},
	} {
		if _, err := cacheSet(c, kv.k, kv.v, 0); err != nil {
			return err
		}
	}
	// Touch k1 so LRU order among full cache is k2, k3, k1.
	if gr, err := cacheGet(c, "k1"); err != nil || !gr.Hit {
		return fmt.Errorf("get k1 before eviction: %v", err)
	}
	if _, err := cacheSet(c, "k4", "v4", 0); err != nil {
		return err
	}
	for _, key := range []string{"k2", "k1", "k3", "k4"} {
		gr, err := cacheGet(c, key)
		if err != nil {
			return err
		}
		wantHit := key != "k2"
		if gr.Hit != wantHit {
			return fmt.Errorf("after inserting k4 with max_keys=3, get %q: hit=%v want %v (k2 should be evicted)", key, gr.Hit, wantHit)
		}
	}
	ctx.Logf("LRU evicted k2 after k1 was touched and k4 was inserted")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	const conns = 8
	const opsPerConn = 30

	setup, err := ctx.Dial()
	if err != nil {
		return err
	}
	if err := configure(setup, 64); err != nil {
		setup.Close()
		return err
	}
	setup.Close()

	type snap struct {
		value   string
		version int
	}
	state := sync.Map{}
	var errs atomic.Value
	var wg sync.WaitGroup
	start := make(chan struct{})

	worker := func(id int) {
		defer wg.Done()
		client, err := ctx.Dial()
		if err != nil {
			errs.Store(err)
			return
		}
		defer client.Close()
		<-start
		for i := 0; i < opsPerConn; i++ {
			key := fmt.Sprintf("g:%d:%d", id, i%5)
			val := fmt.Sprintf("v-%d-%d", id, i)
			sr, err := cacheSet(client, key, val, 0)
			if err != nil {
				errs.Store(err)
				return
			}
			state.Store(key, snap{val, sr.Version})
			gr, err := cacheGet(client, key)
			if err != nil || !gr.Hit || gr.Value != val {
				errs.Store(fmt.Errorf("gauntlet get %q after set: hit=%v value=%q", key, gr.Hit, gr.Value))
				return
			}
		}
	}

	for i := 0; i < conns; i++ {
		wg.Add(1)
		go worker(i)
	}
	close(start)
	wg.Wait()
	if v := errs.Load(); v != nil {
		return v.(error)
	}

	verify, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer verify.Close()
	count := 0
	state.Range(func(k, v any) bool {
		key := k.(string)
		want := v.(snap)
		gr, err := cacheGet(verify, key)
		if err != nil {
			errs.Store(err)
			return false
		}
		if !gr.Hit || gr.Value != want.value || gr.Version != want.version {
			errs.Store(fmt.Errorf("verify %q: hit=%v value=%q version=%d want value=%q version=%d",
				key, gr.Hit, gr.Value, gr.Version, want.value, want.version))
			return false
		}
		count++
		return true
	})
	if v := errs.Load(); v != nil {
		return v.(error)
	}

	// TTL + setnx sanity under a single connection after churn.
	if _, err := cacheSet(verify, "ttl-key", "alive", 250); err != nil {
		return err
	}
	snx, err := cacheSetnx(verify, "ttl-key", "nope", 0)
	if err != nil || snx.Stored {
		return fmt.Errorf("setnx on live key after churn: stored=%v", snx.Stored)
	}
	time.Sleep(320 * time.Millisecond)
	if gr, err := cacheGet(verify, "ttl-key"); err != nil || gr.Hit {
		return fmt.Errorf("ttl-key should expire after concurrent churn")
	}
	ctx.Logf("concurrent set/get across %d connections (%d keys) + ttl/setnx checks", conns, count)
	return nil
}
