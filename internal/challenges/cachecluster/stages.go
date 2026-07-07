// Package cachecluster implements the meta-compose "Build your own cache cluster"
// challenge. The harness spawns reference hash-ring, bloom-filter, rate-limiter,
// and two cache nodes; the user implements the client gateway.
package cachecluster

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

const (
	ringID   = "cache"
	node1    = "node1"
	node2    = "node2"
	filterID = "keys"
)

var composeServices = []harness.ServiceSpec{
	{Name: "ring", ReferenceChallenge: "build-your-own-hash-ring", EnvAddrKey: "HASHRING_ADDR"},
	{Name: "bloom", ReferenceChallenge: "build-your-own-bloom-filter", EnvAddrKey: "BLOOM_ADDR"},
	{Name: "limiter", ReferenceChallenge: "build-your-own-rate-limiter", EnvAddrKey: "LIMITER_ADDR"},
	{Name: "cache1", ReferenceChallenge: "build-your-own-distributed-cache", EnvAddrKey: "CACHE_NODE1_ADDR"},
	{Name: "cache2", ReferenceChallenge: "build-your-own-distributed-cache", EnvAddrKey: "CACHE_NODE2_ADDR"},
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-cache-cluster/stages/"
	return harness.Challenge{
		Slug:     "build-your-own-cache-cluster",
		Name:     "Build your own cache cluster",
		Services: composeServices,
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the stack", Difficulty: "easy", Instructions: docs + "01-bind.md", TestCompose: testBind},
			{Slug: "set-get", Name: "Set and get", Difficulty: "easy", Instructions: docs + "02-set-get.md", TestCompose: testSetGet},
			{Slug: "delete", Name: "Delete a key", Difficulty: "easy", Instructions: docs + "03-delete.md", TestCompose: testDelete},
			{Slug: "routing", Name: "Shard routing", Difficulty: "medium", Instructions: docs + "04-routing.md", TestCompose: testRouting},
			{Slug: "bloom", Name: "Bloom membership", Difficulty: "medium", Instructions: docs + "05-bloom.md", TestCompose: testBloom},
			{Slug: "bloom-miss", Name: "Fast negative lookup", Difficulty: "medium", Instructions: docs + "06-bloom-miss.md", TestCompose: testBloomMiss},
			{Slug: "rate-limit", Name: "Stampede guard", Difficulty: "medium", Instructions: docs + "07-rate-limit.md", TestCompose: testRateLimit},
			{Slug: "mget", Name: "Multi-get", Difficulty: "easy", Instructions: docs + "08-mget.md", TestCompose: testMget},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", TestCompose: testGauntlet},
		},
	}
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

func gwSet(c *harness.Client, key, value string, ttlMS int) (int, error) {
	p := map[string]any{"key": key, "value": value}
	if ttlMS > 0 {
		p["ttl_ms"] = ttlMS
	}
	var res struct {
		Version int `json:"version"`
	}
	if err := c.Call("set", p, &res); err != nil {
		return 0, err
	}
	return res.Version, nil
}

func gwGet(c *harness.Client, key string) (bool, string, int, error) {
	var res struct {
		Hit     bool   `json:"hit"`
		Value   string `json:"value"`
		Version int    `json:"version"`
	}
	if err := c.Call("get", map[string]any{"key": key}, &res); err != nil {
		return false, "", 0, err
	}
	return res.Hit, res.Value, res.Version, nil
}

func gwDelete(c *harness.Client, key string) (bool, error) {
	var res struct {
		Deleted bool `json:"deleted"`
	}
	if err := c.Call("delete", map[string]any{"key": key}, &res); err != nil {
		return false, err
	}
	return res.Deleted, nil
}

func ringLookup(ctx *harness.ComposeContext, key string) (string, error) {
	rc, err := ctx.DialService("ring")
	if err != nil {
		return "", err
	}
	defer rc.Close()
	var res struct {
		NodeID string `json:"node_id"`
	}
	if err := rc.Call("lookup", map[string]any{"ring_id": ringID, "key": key}, &res); err != nil {
		return "", err
	}
	return res.NodeID, nil
}

func bloomContains(ctx *harness.ComposeContext, item string) (bool, error) {
	bc, err := ctx.DialService("bloom")
	if err != nil {
		return false, err
	}
	defer bc.Close()
	var res struct {
		MaybePresent bool `json:"maybe_present"`
	}
	if err := bc.Call("contains", map[string]any{"filter_id": filterID, "item": item}, &res); err != nil {
		return false, err
	}
	return res.MaybePresent, nil
}

func cacheGet(ctx *harness.ComposeContext, nodeName, key string) (bool, string, error) {
	cc, err := ctx.DialService(nodeName)
	if err != nil {
		return false, "", err
	}
	defer cc.Close()
	var res struct {
		Hit   bool   `json:"hit"`
		Value string `json:"value"`
	}
	if err := cc.Call("get", map[string]any{"key": key}, &res); err != nil {
		return false, "", err
	}
	return res.Hit, res.Value, nil
}

func configureLimiter(ctx *harness.ComposeContext, key string, capacity int) error {
	lc, err := ctx.DialService("limiter")
	if err != nil {
		return err
	}
	defer lc.Close()
	return lc.Call("configure", map[string]any{
		"key":                key,
		"algorithm":          "token_bucket",
		"capacity":           capacity,
		"refill_tokens":      capacity,
		"refill_interval_ms": 1000,
	}, nil)
}

func findKeyForNode(ctx *harness.ComposeContext, wantNode string) (string, error) {
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("route-key-%d", i)
		node, err := ringLookup(ctx, key)
		if err != nil {
			return "", err
		}
		if node == wantNode {
			return key, nil
		}
	}
	return "", fmt.Errorf("could not find key routing to %s", wantNode)
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
	for _, name := range []string{"ring", "bloom", "limiter", "cache1", "cache2"} {
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
	ctx.Logf("gateway and five reference services answered ping")
	return nil
}

func testSetGet(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	const key, val = "hello", "world"
	if _, err := gwSet(gw, key, val, 0); err != nil {
		return err
	}
	hit, got, _, err := gwGet(gw, key)
	if err != nil || !hit || got != val {
		return fmt.Errorf("get %q: hit=%v got=%q want %q", key, hit, got, val)
	}
	ctx.Logf("set/get round-trip OK")
	return nil
}

func testDelete(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	const key = "to-delete"
	if _, err := gwSet(gw, key, "x", 0); err != nil {
		return err
	}
	deleted, err := gwDelete(gw, key)
	if err != nil || !deleted {
		return fmt.Errorf("delete: deleted=%v err=%v", deleted, err)
	}
	hit, _, _, err := gwGet(gw, key)
	if err != nil || hit {
		return fmt.Errorf("key should be gone after delete")
	}
	ctx.Logf("delete OK")
	return nil
}

func testRouting(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if _, err := gwSet(gw, "__bootstrap__", "1", 0); err != nil {
		return err
	}
	k1, err := findKeyForNode(ctx, node1)
	if err != nil {
		return err
	}
	k2, err := findKeyForNode(ctx, node2)
	if err != nil {
		return err
	}
	if _, err := gwSet(gw, k1, "on-node1", 0); err != nil {
		return err
	}
	if _, err := gwSet(gw, k2, "on-node2", 0); err != nil {
		return err
	}
	hit1, val1, err := cacheGet(ctx, "cache1", k1)
	if err != nil || !hit1 || val1 != "on-node1" {
		return fmt.Errorf("key %q should live on cache1", k1)
	}
	hit2, val2, err := cacheGet(ctx, "cache2", k2)
	if err != nil || !hit2 || val2 != "on-node2" {
		return fmt.Errorf("key %q should live on cache2", k2)
	}
	ctx.Logf("keys routed to distinct cache nodes")
	return nil
}

func testBloom(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	const key = "bloom-key"
	if _, err := gwSet(gw, key, "v", 0); err != nil {
		return err
	}
	present, err := bloomContains(ctx, key)
	if err != nil || !present {
		return fmt.Errorf("bloom should contain %q after set", key)
	}
	ctx.Logf("bloom filter contains stored key")
	return nil
}

func testBloomMiss(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	const key = "never-stored-key-xyz"
	hit, _, _, err := gwGet(gw, key)
	if err != nil || hit {
		return fmt.Errorf("get unseen key: hit=%v err=%v", hit, err)
	}
	present, err := bloomContains(ctx, key)
	if err != nil {
		return err
	}
	if present {
		return fmt.Errorf("bloom should not contain unseen key")
	}
	ctx.Logf("fast negative miss via bloom")
	return nil
}

func testRateLimit(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	const key = "hot-key"
	if _, err := gwSet(gw, key, "hot", 0); err != nil {
		return err
	}
	if err := configureLimiter(ctx, "rl:"+key, 2); err != nil {
		return err
	}
	const workers = 6
	var allowed, limited atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			gw, err := ctx.DialGateway()
			if err != nil {
				return
			}
			defer gw.Close()
			<-start
			_, _, _, err = gwGet(gw, key)
			if err != nil {
				var rpc *harness.RPCError
				if errors.As(err, &rpc) && rpc.Code == "RATE_LIMITED" {
					limited.Add(1)
					return
				}
			} else {
				allowed.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if allowed.Load() < 1 {
		return fmt.Errorf("expected some gets allowed, got %d", allowed.Load())
	}
	if limited.Load() < 1 {
		return fmt.Errorf("expected some RATE_LIMITED, got %d", limited.Load())
	}
	ctx.Logf("rate limiter blocked %d of %d parallel gets", limited.Load(), workers)
	return nil
}

func testMget(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	keys := []string{"m1", "m2", "m3"}
	for i, k := range keys {
		if _, err := gwSet(gw, k, fmt.Sprintf("v%d", i), 0); err != nil {
			return err
		}
	}
	var res struct {
		Entries []struct {
			Key   string `json:"key"`
			Hit   bool   `json:"hit"`
			Value string `json:"value"`
		} `json:"entries"`
	}
	if err := gw.Call("mget", map[string]any{"keys": keys}, &res); err != nil {
		return err
	}
	if len(res.Entries) != len(keys) {
		return fmt.Errorf("mget: %d entries, want %d", len(res.Entries), len(keys))
	}
	for i, e := range res.Entries {
		if e.Key != keys[i] || !e.Hit || e.Value != fmt.Sprintf("v%d", i) {
			return fmt.Errorf("mget entry %d: %+v", i, e)
		}
	}
	ctx.Logf("mget returned %d hits", len(keys))
	return nil
}

func testGauntlet(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if _, err := gwSet(gw, "g1", "one", 0); err != nil {
		return err
	}
	if _, err := gwSet(gw, "g2", "two", 300); err != nil {
		return err
	}
	hit, val, _, err := gwGet(gw, "g1")
	if err != nil || !hit || val != "one" {
		return fmt.Errorf("gauntlet g1")
	}
	if _, err := gwDelete(gw, "g1"); err != nil {
		return err
	}
	hit, _, _, err = gwGet(gw, "g1")
	if err != nil || hit {
		return fmt.Errorf("g1 should be deleted")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hit, val, _, err = gwGet(gw, "g2")
		if err == nil && hit && val == "two" {
			ctx.Logf("gauntlet: TTL key, delete, mget paths OK")
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for TTL key g2")
}
