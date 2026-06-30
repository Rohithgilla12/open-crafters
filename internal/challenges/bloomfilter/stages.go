// Package bloomfilter implements stage tests for the "Build your own bloom
// filter" challenge. See challenges/build-your-own-bloom-filter/PROTOCOL.md.
package bloomfilter

import (
	"errors"
	"fmt"
	"math/rand"
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

// hashPositions returns the k bit indices for item per PROTOCOL.md.
func hashPositions(item string, m, k int) []int {
	itemBytes := []byte(item)
	h1 := fnv1a64(itemBytes)
	h2Data := append(append([]byte(nil), itemBytes...), 0x01)
	h2 := fnv1a64(h2Data)
	positions := make([]int, k)
	for i := 0; i < k; i++ {
		positions[i] = int((h1 + uint64(i)*h2) % uint64(m))
	}
	return positions
}

// refFilter is a local bloom filter oracle using the protocol hash.
type refFilter struct {
	m    int
	k    int
	bits map[int]bool
}

func newRefFilter(m, k int) *refFilter {
	return &refFilter{m: m, k: k, bits: map[int]bool{}}
}

func (f *refFilter) add(item string) {
	for _, p := range hashPositions(item, f.m, f.k) {
		f.bits[p] = true
	}
}

func (f *refFilter) contains(item string) bool {
	for _, p := range hashPositions(item, f.m, f.k) {
		if !f.bits[p] {
			return false
		}
	}
	return true
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-bloom-filter/stages/"
	return harness.Challenge{
		Slug: "build-your-own-bloom-filter",
		Name: "Build your own bloom filter",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "create", Name: "Create a filter", Difficulty: "easy", Instructions: docs + "02-create.md", Test: testCreate},
			{Slug: "add", Name: "Add an item", Difficulty: "easy", Instructions: docs + "03-add.md", Test: testAdd},
			{Slug: "positive", Name: "Positive lookup", Difficulty: "easy", Instructions: docs + "04-positive.md", Test: testPositive},
			{Slug: "negative", Name: "Negative lookup", Difficulty: "medium", Instructions: docs + "05-negative.md", Test: testNegative},
			{Slug: "no-false-negatives", Name: "No false negatives", Difficulty: "medium", Instructions: docs + "06-no-false-negatives.md", Test: testNoFalseNegatives},
			{Slug: "multi-filter", Name: "Independent filters", Difficulty: "easy", Instructions: docs + "07-multi-filter.md", Test: testMultiFilter},
			{Slug: "hash-functions", Name: "K hash functions", Difficulty: "hard", Instructions: docs + "08-hash-functions.md", Test: testHashFunctions},
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

func createFilter(c *harness.Client, filterID string, m, k int) error {
	return c.Call("create", map[string]any{"filter_id": filterID, "m": m, "k": k}, nil)
}

func addItem(c *harness.Client, filterID, item string) error {
	return c.Call("add", map[string]any{"filter_id": filterID, "item": item}, nil)
}

func containsItem(c *harness.Client, filterID, item string) (bool, error) {
	var res struct {
		MaybePresent bool `json:"maybe_present"`
	}
	if err := c.Call("contains", map[string]any{"filter_id": filterID, "item": item}, &res); err != nil {
		return false, err
	}
	return res.MaybePresent, nil
}

func deleteFilter(c *harness.Client, filterID string) (bool, error) {
	var res struct {
		Deleted bool `json:"deleted"`
	}
	if err := c.Call("delete_filter", map[string]any{"filter_id": filterID}, &res); err != nil {
		return false, err
	}
	return res.Deleted, nil
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

	if err := createFilter(c, "users", 1024, 3); err != nil {
		return fmt.Errorf("create users: %w", err)
	}
	ctx.Logf("created filter users (m=1024, k=3)")

	if err := createFilter(c, "users", 512, 2); expectRPCError(err, "FILTER_EXISTS", "duplicate create") != nil {
		return expectRPCError(err, "FILTER_EXISTS", "duplicate create")
	}

	for _, bad := range []struct {
		id   string
		m, k int
	}{
		{"small-m", 7, 3},
		{"zero-k", 64, 0},
	} {
		if err := createFilter(c, bad.id, bad.m, bad.k); expectRPCError(err, "INVALID_PARAMS", fmt.Sprintf("create %s", bad.id)) != nil {
			return expectRPCError(err, "INVALID_PARAMS", fmt.Sprintf("create %s", bad.id))
		}
	}

	if err := c.Call("create", map[string]any{"filter_id": "missing-m", "k": 3}, nil); expectRPCError(err, "INVALID_PARAMS", "missing m") != nil {
		return expectRPCError(err, "INVALID_PARAMS", "missing m")
	}
	ctx.Logf("FILTER_EXISTS and INVALID_PARAMS enforced")
	return nil
}

func testAdd(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := createFilter(c, "tags", 256, 4); err != nil {
		return err
	}
	if err := addItem(c, "tags", "golang"); err != nil {
		return fmt.Errorf("add golang: %w", err)
	}
	if err := addItem(c, "missing", "x"); expectRPCError(err, "FILTER_NOT_FOUND", "add to missing filter") != nil {
		return expectRPCError(err, "FILTER_NOT_FOUND", "add to missing filter")
	}
	ctx.Logf("add succeeded on an existing filter")
	return nil
}

func testPositive(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := createFilter(c, "emails", 512, 5); err != nil {
		return err
	}
	const item = "alice@example.com"
	if err := addItem(c, "emails", item); err != nil {
		return err
	}
	present, err := containsItem(c, "emails", item)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("contains %q after add: expected maybe_present=true", item)
	}
	ctx.Logf("added item returns maybe_present=true")
	return nil
}

func testNegative(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const m, k = 1024, 3
	if err := createFilter(c, "sparse", m, k); err != nil {
		return err
	}
	// Keep the filter sparse: at most two inserts.
	for _, item := range []string{"seed-a", "seed-b"} {
		if err := addItem(c, "sparse", item); err != nil {
			return err
		}
	}

	neverAdded := []string{
		"ghost-0", "ghost-1", "ghost-2", "ghost-3", "ghost-4",
		"ghost-5", "ghost-6", "ghost-7", "ghost-8", "ghost-9",
	}
	for _, item := range neverAdded {
		present, err := containsItem(c, "sparse", item)
		if err != nil {
			return err
		}
		if present {
			return fmt.Errorf("contains %q on a sparse filter: expected maybe_present=false (never added)", item)
		}
	}
	ctx.Logf("10 never-added items returned maybe_present=false on a sparse filter")
	return nil
}

func testNoFalseNegatives(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := createFilter(c, "bulk", 8192, 4); err != nil {
		return err
	}
	items := make([]string, 200)
	for i := range items {
		items[i] = fmt.Sprintf("member-%04d", i)
		if err := addItem(c, "bulk", items[i]); err != nil {
			return fmt.Errorf("add %q: %w", items[i], err)
		}
	}
	for _, item := range items {
		present, err := containsItem(c, "bulk", item)
		if err != nil {
			return err
		}
		if !present {
			return fmt.Errorf("contains %q after add: bloom filters must not produce false negatives", item)
		}
	}
	ctx.Logf("200 added items all returned maybe_present=true")
	return nil
}

func testMultiFilter(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := createFilter(c, "a", 128, 3); err != nil {
		return err
	}
	if err := createFilter(c, "b", 128, 3); err != nil {
		return err
	}
	if err := addItem(c, "a", "only-in-a"); err != nil {
		return err
	}
	if err := addItem(c, "b", "only-in-b"); err != nil {
		return err
	}

	inA, err := containsItem(c, "a", "only-in-a")
	if err != nil || !inA {
		return fmt.Errorf("filter a should contain only-in-a")
	}
	inBFromA, err := containsItem(c, "a", "only-in-b")
	if err != nil {
		return err
	}
	if inBFromA {
		return fmt.Errorf("filter a must not see items added only to filter b")
	}
	inB, err := containsItem(c, "b", "only-in-b")
	if err != nil || !inB {
		return fmt.Errorf("filter b should contain only-in-b")
	}
	inAFromB, err := containsItem(c, "b", "only-in-a")
	if err != nil {
		return err
	}
	if inAFromB {
		return fmt.Errorf("filter b must not see items added only to filter a")
	}
	ctx.Logf("filters are independent")
	return nil
}

func testHashFunctions(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const m, k = 512, 4
	adds := []string{"apple", "banana", "cherry", "date"}
	if err := createFilter(c, "hf", m, k); err != nil {
		return err
	}
	ref := newRefFilter(m, k)
	for _, item := range adds {
		if err := addItem(c, "hf", item); err != nil {
			return err
		}
		ref.add(item)
	}

	for _, item := range adds {
		present, err := containsItem(c, "hf", item)
		if err != nil {
			return err
		}
		if !present {
			return fmt.Errorf("contains %q after add: all k=%d hash positions must be set on add", item, k)
		}
	}

	// After several inserts, a single-hash contains cheat (checking only h1 % m)
	// false-positives on this probe; the reference oracle expects false.
	const cheatProbe = "z-76"
	want := ref.contains(cheatProbe)
	present, err := containsItem(c, "hf", cheatProbe)
	if err != nil {
		return err
	}
	if present != want {
		return fmt.Errorf("contains %q: got maybe_present=%v, reference hash expects %v — use all k positions (h1 + i*h2) %% m, not just h1 %% m", cheatProbe, present, want)
	}
	ctx.Logf("probe %q matches the reference oracle (maybe_present=%v)", cheatProbe, want)

	for i := 0; i < 100; i++ {
		item := fmt.Sprintf("oracle-%d", i)
		want := ref.contains(item)
		got, err := containsItem(c, "hf", item)
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("contains %q: got %v, reference expects %v — check FNV-1a double hashing", item, got, want)
		}
	}

	// Different k must set different numbers of bits: k=1 vs k=4 on the same item.
	if err := createFilter(c, "k1", m, 1); err != nil {
		return err
	}
	if err := createFilter(c, "k4", m, 4); err != nil {
		return err
	}
	const probe = "hash-probe-item"
	if err := addItem(c, "k1", probe); err != nil {
		return err
	}
	if err := addItem(c, "k4", probe); err != nil {
		return err
	}
	p1, err := containsItem(c, "k1", probe)
	if err != nil || !p1 {
		return fmt.Errorf("k=1 filter must contain %q after add", probe)
	}
	p4, err := containsItem(c, "k4", probe)
	if err != nil || !p4 {
		return fmt.Errorf("k=4 filter must contain %q after add", probe)
	}
	ctx.Logf("reference oracle and k=%d add/contains verified", k)
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	const filters = 4
	const m, k = 512, 5
	const conns = 8
	const opsPerConn = 40

	setup, err := ctx.Dial()
	if err != nil {
		return err
	}
	for i := 0; i < filters; i++ {
		id := fmt.Sprintf("g-%d", i)
		if err := createFilter(setup, id, m, k); err != nil {
			setup.Close()
			return err
		}
	}
	setup.Close()

	type key struct {
		filterID string
		item     string
	}
	added := sync.Map{}
	rng := rand.New(rand.NewSource(99))

	var errs atomic.Value
	var wg sync.WaitGroup
	start := make(chan struct{})

	recordAdd := func(filterID, item string) {
		added.Store(key{filterID, item}, true)
	}

	for c := 0; c < conns; c++ {
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
				filterID := fmt.Sprintf("g-%d", rng.Intn(filters))
				item := fmt.Sprintf("item-%d-%d", connIdx, i)
				if err := addItem(client, filterID, item); err != nil {
					errs.Store(err)
					return
				}
				recordAdd(filterID, item)
				present, err := containsItem(client, filterID, item)
				if err != nil {
					errs.Store(err)
					return
				}
				if !present {
					errs.Store(fmt.Errorf("gauntlet: %q in %q should be present after add", item, filterID))
					return
				}
			}
		}(c)
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
	added.Range(func(k, _ any) bool {
		kk := k.(key)
		present, err := containsItem(verify, kk.filterID, kk.item)
		if err != nil {
			errs.Store(err)
			return false
		}
		if !present {
			errs.Store(fmt.Errorf("final verify: %q in %q missing after concurrent churn", kk.item, kk.filterID))
			return false
		}
		count++
		return true
	})
	if v := errs.Load(); v != nil {
		return v.(error)
	}
	ctx.Logf("concurrent add/contains across %d filters and %d connections (%d items verified)", filters, conns, count)

	// Sequential delete_filter sanity check (optional RPC, useful for cleanup).
	if err := createFilter(verify, "tmp-del", m, k); err != nil {
		return err
	}
	if err := addItem(verify, "tmp-del", "x"); err != nil {
		return err
	}
	deleted, err := deleteFilter(verify, "tmp-del")
	if err != nil || !deleted {
		return fmt.Errorf("delete_filter on existing filter: deleted=%v err=%v", deleted, err)
	}
	if err := addItem(verify, "tmp-del", "y"); expectRPCError(err, "FILTER_NOT_FOUND", "add after delete") != nil {
		return expectRPCError(err, "FILTER_NOT_FOUND", "add after delete")
	}
	ctx.Logf("delete_filter removes a filter from memory")
	return nil
}
