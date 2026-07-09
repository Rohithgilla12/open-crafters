// Package distkv implements the meta-compose "Build your own distributed KV"
// challenge. The harness spawns reference hash-ring, a 3-node Raft cluster, and
// an LSM shard; the user implements the client gateway.
package distkv

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

const (
	ringID       = "kv"
	raftShard    = "raft-shard"
	lsmShard     = "lsm-shard"
	ringReplicas = 64
)

var composeServices = []harness.ServiceSpec{
	{Name: "ring", ReferenceChallenge: "build-your-own-hash-ring", EnvAddrKey: "HASHRING_ADDR"},
	{Name: "raft", ReferenceChallenge: "build-your-own-raft", EnvAddrKey: "RAFT", ClusterSize: 3},
	{Name: "lsm", ReferenceChallenge: "build-your-own-lsm", EnvAddrKey: "LSM_ADDR"},
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-distributed-kv/stages/"
	return harness.Challenge{
		Slug:     "build-your-own-distributed-kv",
		Name:     "Build your own distributed KV",
		Services: composeServices,
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the stack", Difficulty: "easy", Instructions: docs + "01-bind.md", TestCompose: testBind},
			{Slug: "put-get", Name: "Put and get", Difficulty: "easy", Instructions: docs + "02-put-get.md", TestCompose: testPutGet},
			{Slug: "routing", Name: "Shard routing", Difficulty: "medium", Instructions: docs + "03-routing.md", TestCompose: testRouting},
			{Slug: "raft-write", Name: "Replicated shard", Difficulty: "medium", Instructions: docs + "04-raft-write.md", TestCompose: testRaftWrite},
			{Slug: "raft-read", Name: "Read from follower", Difficulty: "medium", Instructions: docs + "05-raft-read.md", TestCompose: testRaftRead},
			{Slug: "delete", Name: "Delete on LSM shard", Difficulty: "easy", Instructions: docs + "06-delete.md", TestCompose: testDelete},
			{Slug: "lsm-durable", Name: "LSM flush", Difficulty: "medium", Instructions: docs + "07-lsm-durable.md", TestCompose: testLSMDurable},
			{Slug: "concurrent", Name: "Parallel writes", Difficulty: "hard", Instructions: docs + "08-concurrent.md", TestCompose: testConcurrent},
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

func gwPut(c *harness.Client, key, value string) error {
	return c.Call("put", map[string]any{"key": key, "value": value}, nil)
}

func gwGet(c *harness.Client, key string) (bool, string, error) {
	var res struct {
		Found bool   `json:"found"`
		Value string `json:"value"`
	}
	if err := c.Call("get", map[string]any{"key": key}, &res); err != nil {
		return false, "", err
	}
	return res.Found, res.Value, nil
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

func bootstrapRing(ctx *harness.ComposeContext) error {
	rc, err := ctx.DialService("ring")
	if err != nil {
		return err
	}
	defer rc.Close()
	if err := rc.Call("create_ring", map[string]any{"ring_id": ringID, "replicas": ringReplicas}, nil); err != nil {
		var rpcErr *harness.RPCError
		if !errors.As(err, &rpcErr) || rpcErr.Code != "RING_EXISTS" {
			return err
		}
	}
	for _, node := range []string{raftShard, lsmShard} {
		if err := rc.Call("add_node", map[string]any{"ring_id": ringID, "node_id": node}, nil); err != nil {
			var rpcErr *harness.RPCError
			if !errors.As(err, &rpcErr) || rpcErr.Code != "NODE_EXISTS" {
				return err
			}
		}
	}
	return nil
}

func ringLookup(ctx *harness.ComposeContext, key string) (string, error) {
	if err := bootstrapRing(ctx); err != nil {
		return "", err
	}
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

func raftGet(ctx *harness.ComposeContext, nodeID int, key string) (bool, string, error) {
	rc, err := ctx.DialClusterNode("raft", nodeID)
	if err != nil {
		return false, "", err
	}
	defer rc.Close()
	var res struct {
		Found bool `json:"found"`
		Value any  `json:"value"`
	}
	if err := rc.Call("get", map[string]any{"key": key}, &res); err != nil {
		return false, "", err
	}
	if !res.Found {
		return false, "", nil
	}
	s, _ := res.Value.(string)
	return true, s, nil
}

func lsmFlush(ctx *harness.ComposeContext) error {
	lc, err := ctx.DialService("lsm")
	if err != nil {
		return err
	}
	defer lc.Close()
	return lc.Call("flush", nil, nil)
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
	for _, name := range []string{"ring", "lsm"} {
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
	}
	for i := 1; i <= 3; i++ {
		rc, err := ctx.DialClusterNode("raft", i)
		if err != nil {
			return err
		}
		var res struct {
			Message string `json:"message"`
		}
		if err := rc.Call("ping", nil, &res); err != nil {
			rc.Close()
			return fmt.Errorf("raft node %d ping: %w", i, err)
		}
		rc.Close()
	}
	ctx.Logf("gateway, hash ring, LSM, and 3-node Raft cluster answered ping")
	return nil
}

func testPutGet(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	key, err := findKeyForNode(ctx, lsmShard)
	if err != nil {
		return err
	}
	if err := gwPut(gw, key, "alpha"); err != nil {
		return fmt.Errorf("put: %w", err)
	}
	found, val, err := gwGet(gw, key)
	if err != nil {
		return err
	}
	if !found || val != "alpha" {
		return fmt.Errorf("get: expected alpha, got found=%v value=%q", found, val)
	}
	ctx.Logf("put/get on LSM shard succeeded")
	return nil
}

func testRouting(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	raftKey, err := findKeyForNode(ctx, raftShard)
	if err != nil {
		return err
	}
	lsmKey, err := findKeyForNode(ctx, lsmShard)
	if err != nil {
		return err
	}
	if err := gwPut(gw, raftKey, "replicated"); err != nil {
		return fmt.Errorf("put raft key: %w", err)
	}
	if err := gwPut(gw, lsmKey, "local"); err != nil {
		return fmt.Errorf("put lsm key: %w", err)
	}
	_, rv, err := gwGet(gw, raftKey)
	if err != nil || rv != "replicated" {
		return fmt.Errorf("get raft key: %v %q", err, rv)
	}
	_, lv, err := gwGet(gw, lsmKey)
	if err != nil || lv != "local" {
		return fmt.Errorf("get lsm key: %v %q", err, lv)
	}
	ctx.Logf("keys routed to raft-shard and lsm-shard independently")
	return nil
}

func testRaftWrite(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	key, err := findKeyForNode(ctx, raftShard)
	if err != nil {
		return err
	}
	if err := gwPut(gw, key, "quorum"); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	for i := 1; i <= 3; i++ {
		found, val, err := raftGet(ctx, i, key)
		if err != nil {
			return fmt.Errorf("raft node %d get: %w", i, err)
		}
		if !found || val != "quorum" {
			return fmt.Errorf("raft node %d: expected quorum, got found=%v val=%q", i, found, val)
		}
	}
	ctx.Logf("replicated write visible on all Raft nodes")
	return nil
}

func testRaftRead(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	key, err := findKeyForNode(ctx, raftShard)
	if err != nil {
		return err
	}
	if err := gwPut(gw, key, "follower-read"); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	found, val, err := raftGet(ctx, 2, key)
	if err != nil {
		return err
	}
	if !found || val != "follower-read" {
		return fmt.Errorf("follower read: got found=%v val=%q", found, val)
	}
	ctx.Logf("follower serves committed read after gateway write")
	return nil
}

func testDelete(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	key, err := findKeyForNode(ctx, lsmShard)
	if err != nil {
		return err
	}
	if err := gwPut(gw, key, "gone"); err != nil {
		return err
	}
	deleted, err := gwDelete(gw, key)
	if err != nil || !deleted {
		return fmt.Errorf("delete: deleted=%v err=%v", deleted, err)
	}
	found, _, err := gwGet(gw, key)
	if err != nil {
		return err
	}
	if found {
		return fmt.Errorf("key should be absent after delete")
	}
	ctx.Logf("delete on LSM shard works")
	return nil
}

func testLSMDurable(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	key, err := findKeyForNode(ctx, lsmShard)
	if err != nil {
		return err
	}
	if err := gwPut(gw, key, "flushed"); err != nil {
		return err
	}
	if err := lsmFlush(ctx); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	found, val, err := gwGet(gw, key)
	if err != nil {
		return err
	}
	if !found || val != "flushed" {
		return fmt.Errorf("after flush: expected flushed, got found=%v val=%q", found, val)
	}
	ctx.Logf("value survives LSM flush")
	return nil
}

func testConcurrent(ctx *harness.ComposeContext) error {
	var keys []string
	for i := 0; len(keys) < 4; i++ {
		key := fmt.Sprintf("conc-%d", i)
		node, err := ringLookup(ctx, key)
		if err != nil {
			return err
		}
		if node == lsmShard {
			keys = append(keys, key)
		}
	}
	if len(keys) < 4 {
		return fmt.Errorf("could not find 4 keys routing to %s", lsmShard)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i, key := range keys {
		wg.Add(1)
		go func(n int, key string) {
			defer wg.Done()
			gw, err := ctx.DialGateway()
			if err != nil {
				errs <- err
				return
			}
			defer gw.Close()
			val := fmt.Sprintf("v%d", n)
			if err := gwPut(gw, key, val); err != nil {
				errs <- err
				return
			}
			found, got, err := gwGet(gw, key)
			if err != nil || !found || got != val {
				errs <- fmt.Errorf("goroutine %d: got %q found=%v", n, got, found)
			}
		}(i, key)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	ctx.Logf("concurrent LSM-shard writes succeeded")
	return nil
}

func testGauntlet(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	raftKey, _ := findKeyForNode(ctx, raftShard)
	lsmKey, _ := findKeyForNode(ctx, lsmShard)
	if err := gwPut(gw, raftKey, "g-raft"); err != nil {
		return err
	}
	if err := gwPut(gw, lsmKey, "g-lsm"); err != nil {
		return err
	}
	if err := lsmFlush(ctx); err != nil {
		return err
	}
	_, rv, err := gwGet(gw, raftKey)
	if err != nil || rv != "g-raft" {
		return fmt.Errorf("raft key in gauntlet: %v %q", err, rv)
	}
	_, lv, err := gwGet(gw, lsmKey)
	if err != nil || lv != "g-lsm" {
		return fmt.Errorf("lsm key in gauntlet: %v %q", err, lv)
	}
	deleted, err := gwDelete(gw, lsmKey)
	if err != nil || !deleted {
		return fmt.Errorf("gauntlet delete: %v", err)
	}
	ctx.Logf("gauntlet: raft + lsm routing, flush, and delete")
	return nil
}
