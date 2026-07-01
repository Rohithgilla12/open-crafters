// Package hashring implements stage tests for the "Build your own hash ring"
// challenge. See challenges/build-your-own-hash-ring/PROTOCOL.md.
package hashring

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

const (
	fnvOffset64 = uint64(14695981039346656037)
	fnvPrime64  = uint64(1099511628211)
)

func fnv1a64(data []byte) uint64 {
	hash := fnvOffset64
	for _, b := range data {
		hash ^= uint64(b)
		hash *= fnvPrime64
	}
	return hash
}

func hashKey(key string) uint64 {
	return fnv1a64([]byte(key))
}

func vnodePosition(nodeID string, replica int) uint64 {
	return fnv1a64([]byte(nodeID + "#" + strconv.Itoa(replica)))
}

type vnode struct {
	position uint64
	nodeID   string
}

// refLookup returns the owning physical node per PROTOCOL.md.
func refLookup(nodeIDs []string, replicas int, key string) string {
	if len(nodeIDs) == 0 {
		return ""
	}
	vnodes := make([]vnode, 0, len(nodeIDs)*replicas)
	for _, nodeID := range nodeIDs {
		for i := 0; i < replicas; i++ {
			vnodes = append(vnodes, vnode{
				position: vnodePosition(nodeID, i),
				nodeID:   nodeID,
			})
		}
	}
	sort.Slice(vnodes, func(i, j int) bool {
		if vnodes[i].position != vnodes[j].position {
			return vnodes[i].position < vnodes[j].position
		}
		return vnodes[i].nodeID < vnodes[j].nodeID
	})
	h := hashKey(key)
	for _, v := range vnodes {
		if v.position >= h {
			return v.nodeID
		}
	}
	return vnodes[0].nodeID
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-hash-ring/stages/"
	return harness.Challenge{
		Slug: "build-your-own-hash-ring",
		Name: "Build your own hash ring",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "create", Name: "Create a ring", Difficulty: "easy", Instructions: docs + "02-create.md", Test: testCreate},
			{Slug: "add-node", Name: "Add a node", Difficulty: "easy", Instructions: docs + "03-add-node.md", Test: testAddNode},
			{Slug: "deterministic", Name: "Deterministic lookup", Difficulty: "easy", Instructions: docs + "04-deterministic.md", Test: testDeterministic},
			{Slug: "spread", Name: "Key spread", Difficulty: "medium", Instructions: docs + "05-spread.md", Test: testSpread},
			{Slug: "minimal-move", Name: "Add node (minimal move)", Difficulty: "medium", Instructions: docs + "06-minimal-move.md", Test: testMinimalMove},
			{Slug: "remove-node", Name: "Remove a node", Difficulty: "medium", Instructions: docs + "07-remove-node.md", Test: testRemoveNode},
			{Slug: "virtual-nodes", Name: "Virtual nodes", Difficulty: "hard", Instructions: docs + "08-virtual-nodes.md", Test: testVirtualNodes},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", Test: testGauntlet},
		},
	}
}

// --- RPC helpers ---

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

func createRing(c *harness.Client, ringID string, replicas int) error {
	return c.Call("create_ring", map[string]any{"ring_id": ringID, "replicas": replicas}, nil)
}

func addNode(c *harness.Client, ringID, nodeID string) error {
	return c.Call("add_node", map[string]any{"ring_id": ringID, "node_id": nodeID}, nil)
}

func removeNode(c *harness.Client, ringID, nodeID string) (bool, error) {
	var res struct {
		Removed bool `json:"removed"`
	}
	if err := c.Call("remove_node", map[string]any{"ring_id": ringID, "node_id": nodeID}, &res); err != nil {
		return false, err
	}
	return res.Removed, nil
}

func lookup(c *harness.Client, ringID, key string) (string, error) {
	var res struct {
		NodeID string `json:"node_id"`
	}
	if err := c.Call("lookup", map[string]any{"ring_id": ringID, "key": key}, &res); err != nil {
		return "", err
	}
	return res.NodeID, nil
}

func listNodes(c *harness.Client, ringID string) ([]string, error) {
	var res struct {
		Nodes []string `json:"nodes"`
	}
	if err := c.Call("list_nodes", map[string]any{"ring_id": ringID}, &res); err != nil {
		return nil, err
	}
	if res.Nodes == nil {
		res.Nodes = []string{}
	}
	return res.Nodes, nil
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

// --- stages ---

func testBind(ctx *harness.Context) error {
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

func testCreate(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := createRing(c, "cache", 3); err != nil {
		return fmt.Errorf("create_ring cache: %w", err)
	}
	ctx.Logf("created ring cache (replicas=3)")

	if err := createRing(c, "cache", 1); expectRPCError(err, "RING_EXISTS", "duplicate create_ring") != nil {
		return expectRPCError(err, "RING_EXISTS", "duplicate create_ring")
	}

	for _, bad := range []struct {
		id       string
		replicas int
	}{
		{"zero-replicas", 0},
		{"negative", -1},
	} {
		if err := createRing(c, bad.id, bad.replicas); expectRPCError(err, "INVALID_PARAMS", fmt.Sprintf("create %s", bad.id)) != nil {
			return expectRPCError(err, "INVALID_PARAMS", fmt.Sprintf("create %s", bad.id))
		}
	}

	if err := c.Call("create_ring", map[string]any{"ring_id": "missing-replicas"}, nil); expectRPCError(err, "INVALID_PARAMS", "missing replicas") != nil {
		return expectRPCError(err, "INVALID_PARAMS", "missing replicas")
	}
	ctx.Logf("RING_EXISTS and INVALID_PARAMS enforced")
	return nil
}

func testAddNode(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := createRing(c, "solo", 1); err != nil {
		return err
	}
	if err := addNode(c, "solo", "only-one"); err != nil {
		return fmt.Errorf("add_node: %w", err)
	}
	if err := addNode(c, "solo", "only-one"); expectRPCError(err, "NODE_EXISTS", "duplicate add_node") != nil {
		return expectRPCError(err, "NODE_EXISTS", "duplicate add_node")
	}
	if err := addNode(c, "missing", "x"); expectRPCError(err, "RING_NOT_FOUND", "add to missing ring") != nil {
		return expectRPCError(err, "RING_NOT_FOUND", "add to missing ring")
	}

	for _, key := range []string{"alpha", "beta", "gamma", "key-42"} {
		node, err := lookup(c, "solo", key)
		if err != nil {
			return err
		}
		if node != "only-one" {
			return fmt.Errorf("lookup %q on single-node ring: got %q, want only-one", key, node)
		}
	}
	ctx.Logf("single node owns every key")
	return nil
}

func testDeterministic(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := createRing(c, "stable", 4); err != nil {
		return err
	}
	for _, id := range []string{"node-a", "node-b", "node-c"} {
		if err := addNode(c, "stable", id); err != nil {
			return err
		}
	}

	const key = "session-7f3a"
	want := refLookup([]string{"node-a", "node-b", "node-c"}, 4, key)
	var first string
	for i := 0; i < 50; i++ {
		got, err := lookup(c, "stable", key)
		if err != nil {
			return err
		}
		if i == 0 {
			first = got
		} else if got != first {
			return fmt.Errorf("lookup %q: iteration %d returned %q, first was %q — lookups must be deterministic", key, i, got, first)
		}
	}
	if first != want {
		return fmt.Errorf("lookup %q: got %q, reference hash expects %q", key, first, want)
	}
	ctx.Logf("50 lookups of %q all returned %q (matches reference)", key, first)
	return nil
}

func testSpread(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const ringID = "spread"
	if err := createRing(c, ringID, 1); err != nil {
		return err
	}
	nodes := []string{"alpha", "beta", "gamma"}
	for _, id := range nodes {
		if err := addNode(c, ringID, id); err != nil {
			return err
		}
	}

	counts := map[string]int{}
	const keys = 2000 // item-0000..1999 spans enough hash space for 3 vnodes
	const minShare = 0.15
	for i := 0; i < keys; i++ {
		key := fmt.Sprintf("item-%04d", i)
		node, err := lookup(c, ringID, key)
		if err != nil {
			return err
		}
		want := refLookup(nodes, 1, key)
		if node != want {
			return fmt.Errorf("lookup %q: got %q, reference expects %q", key, node, want)
		}
		counts[node]++
	}

	minKeys := int(float64(keys) * minShare)
	for _, id := range nodes {
		if counts[id] < minKeys {
			return fmt.Errorf("node %q owns %d/%d keys (%.1f%%), need at least %.0f%% — check vnode hashing and clockwise lookup", id, counts[id], keys, 100*float64(counts[id])/float64(keys), 100*minShare)
		}
	}
	ctx.Logf("each of 3 nodes owns at least 15%% of 300 keys")
	return nil
}

func testMinimalMove(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const ringID = "grow"
	if err := createRing(c, ringID, 1); err != nil {
		return err
	}
	nodes := []string{"n1", "n2", "n3"}
	for _, id := range nodes {
		if err := addNode(c, ringID, id); err != nil {
			return err
		}
	}

	const keys = 200
	before := make([]string, keys)
	for i := 0; i < keys; i++ {
		key := fmt.Sprintf("record-%04d", i)
		node, err := lookup(c, ringID, key)
		if err != nil {
			return err
		}
		before[i] = node
	}

	if err := addNode(c, ringID, "n4"); err != nil {
		return err
	}
	nodes = append(nodes, "n4")

	changed := 0
	for i := 0; i < keys; i++ {
		key := fmt.Sprintf("record-%04d", i)
		after, err := lookup(c, ringID, key)
		if err != nil {
			return err
		}
		want := refLookup(nodes, 1, key)
		if after != want {
			return fmt.Errorf("lookup %q after add: got %q, reference expects %q", key, after, want)
		}
		if after != before[i] {
			changed++
		}
	}

	maxChange := int(float64(keys) * 0.45)
	if changed >= maxChange {
		return fmt.Errorf("adding a 4th node moved %d/%d keys (%.0f%%), consistent hashing should move fewer than 45%% (%d)", changed, keys, 100*float64(changed)/float64(keys), maxChange)
	}
	ctx.Logf("adding n4 moved %d/%d keys (%.0f%%) — under the 45%% threshold", changed, keys, 100*float64(changed)/float64(keys))
	return nil
}

func testRemoveNode(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const ringID = "shrink"
	if err := createRing(c, ringID, 2); err != nil {
		return err
	}
	nodes := []string{"east", "west", "north"}
	for _, id := range nodes {
		if err := addNode(c, ringID, id); err != nil {
			return err
		}
	}

	const keys = 150
	for i := 0; i < keys; i++ {
		key := fmt.Sprintf("k-%04d", i)
		node, err := lookup(c, ringID, key)
		if err != nil {
			return err
		}
		want := refLookup(nodes, 2, key)
		if node != want {
			return fmt.Errorf("lookup %q: got %q, reference expects %q", key, node, want)
		}
	}

	removed, err := removeNode(c, ringID, "west")
	if err != nil || !removed {
		return fmt.Errorf("remove_node west: removed=%v err=%v", removed, err)
	}
	remaining := []string{"east", "north"}

	for i := 0; i < keys; i++ {
		key := fmt.Sprintf("k-%04d", i)
		node, err := lookup(c, ringID, key)
		if err != nil {
			return err
		}
		if node == "west" {
			return fmt.Errorf("lookup %q after remove: returned removed node west", key)
		}
		want := refLookup(remaining, 2, key)
		if node != want {
			return fmt.Errorf("lookup %q after remove: got %q, reference expects %q", key, node, want)
		}
	}

	removed, err = removeNode(c, ringID, "east")
	if err != nil || !removed {
		return fmt.Errorf("remove_node east: removed=%v err=%v", removed, err)
	}
	removed, err = removeNode(c, ringID, "north")
	if err != nil || !removed {
		return fmt.Errorf("remove_node north: removed=%v err=%v", removed, err)
	}
	if _, err := lookup(c, ringID, "any"); expectRPCError(err, "NO_NODES", "lookup on empty ring") != nil {
		return expectRPCError(err, "NO_NODES", "lookup on empty ring")
	}
	ctx.Logf("removed west; 150 keys remap to east/north; empty ring returns NO_NODES")
	return nil
}

func keySpread(c *harness.Client, ringID string, nodes []string, replicas, keys int) (map[string]int, int, error) {
	counts := map[string]int{}
	for _, id := range nodes {
		counts[id] = 0
	}
	for i := 0; i < keys; i++ {
		key := fmt.Sprintf("load-%04d", i)
		node, err := lookup(c, ringID, key)
		if err != nil {
			return nil, 0, err
		}
		want := refLookup(nodes, replicas, key)
		if node != want {
			return nil, 0, fmt.Errorf("lookup %q: got %q, reference expects %q", key, node, want)
		}
		counts[node]++
	}
	minC, maxC := keys, 0
	for _, id := range nodes {
		if counts[id] < minC {
			minC = counts[id]
		}
		if counts[id] > maxC {
			maxC = counts[id]
		}
	}
	return counts, maxC - minC, nil
}

func testVirtualNodes(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	nodes := []string{"p1", "p2", "p3"}
	const keys = 600

	if err := createRing(c, "low-rep", 1); err != nil {
		return err
	}
	if err := createRing(c, "high-rep", 50); err != nil {
		return err
	}
	for _, id := range nodes {
		if err := addNode(c, "low-rep", id); err != nil {
			return err
		}
		if err := addNode(c, "high-rep", id); err != nil {
			return err
		}
	}

	_, spreadLow, err := keySpread(c, "low-rep", nodes, 1, keys)
	if err != nil {
		return err
	}
	_, spreadHigh, err := keySpread(c, "high-rep", nodes, 50, keys)
	if err != nil {
		return err
	}
	if spreadHigh >= spreadLow {
		return fmt.Errorf("virtual nodes: replicas=50 spread (max-min)=%d should be less than replicas=1 spread=%d", spreadHigh, spreadLow)
	}
	ctx.Logf("replicas=50 spread=%d vs replicas=1 spread=%d", spreadHigh, spreadLow)
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	const conns = 8
	const opsPerConn = 30

	setup, err := ctx.Dial()
	if err != nil {
		return err
	}
	for _, ringID := range []string{"g-a", "g-b"} {
		if err := createRing(setup, ringID, 10); err != nil {
			setup.Close()
			return err
		}
		for _, id := range []string{"n1", "n2", "n3"} {
			if err := addNode(setup, ringID, id); err != nil {
				setup.Close()
				return err
			}
		}
	}
	setup.Close()

	type record struct {
		ringID string
		key    string
	}
	var records sync.Map
	rng := rand.New(rand.NewSource(42))
	var errs atomic.Value
	var wg sync.WaitGroup
	start := make(chan struct{})

	for conn := 0; conn < conns; conn++ {
		wg.Add(1)
		go func(connIdx int) {
			defer wg.Done()
			client, err := ctx.Dial()
			if err != nil {
				errs.Store(err)
				return
			}
			defer client.Close()
			<-start
			for i := 0; i < opsPerConn; i++ {
				ringID := "g-a"
				if rng.Intn(2) == 1 {
					ringID = "g-b"
				}
				switch rng.Intn(3) {
				case 0:
					nodeID := fmt.Sprintf("extra-%d-%d", connIdx, i)
					if err := addNode(client, ringID, nodeID); err != nil {
						errs.Store(err)
						return
					}
				case 1:
					_, _ = removeNode(client, ringID, fmt.Sprintf("extra-%d-%d", connIdx, i%5))
				default:
					key := fmt.Sprintf("key-%d-%d", connIdx, i)
					node, err := lookup(client, ringID, key)
					if err != nil {
						errs.Store(err)
						return
					}
					records.Store(record{ringID, key}, node)
				}
			}
		}(conn)
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

	records.Range(func(k, _ any) bool {
		rec := k.(record)
		nodes, err := listNodes(verify, rec.ringID)
		if err != nil {
			errs.Store(err)
			return false
		}
		sort.Strings(nodes)
		got, err := lookup(verify, rec.ringID, rec.key)
		if err != nil {
			errs.Store(err)
			return false
		}
		want := refLookup(nodes, 10, rec.key)
		if got != want {
			errs.Store(fmt.Errorf("gauntlet verify %q on %q: got %q, reference expects %q", rec.key, rec.ringID, got, want))
			return false
		}
		return true
	})
	if v := errs.Load(); v != nil {
		return v.(error)
	}

	listed, err := listNodes(verify, "g-a")
	if err != nil {
		return err
	}
	if !sort.StringsAreSorted(listed) {
		return fmt.Errorf("list_nodes g-a not sorted: %v", listed)
	}
	ctx.Logf("concurrent churn across 2 rings verified against reference oracle")
	return nil
}
