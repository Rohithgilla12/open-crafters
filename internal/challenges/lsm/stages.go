package lsm

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-lsm/stages/"
	return harness.Challenge{
		Slug: "build-your-own-lsm",
		Name: "Build your own LSM-tree",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "put-get", Name: "An in-memory key-value store", Difficulty: "easy", Instructions: docs + "02-put-get.md", Test: testPutGet},
			{Slug: "flush", Name: "Flush to SSTable", Difficulty: "medium", Instructions: docs + "03-flush.md", Test: testFlush},
			{Slug: "restart", Name: "Recover after restart", Difficulty: "medium", Instructions: docs + "04-restart.md", Test: testRestart},
			{Slug: "scan", Name: "Range scan", Difficulty: "medium", Instructions: docs + "05-scan.md", Test: testScan},
			{Slug: "compaction", Name: "Compact SSTables", Difficulty: "hard", Instructions: docs + "06-compaction.md", Test: testCompaction},
			{Slug: "delete", Name: "Tombstones", Difficulty: "hard", Instructions: docs + "07-delete.md", Test: testDelete},
			{Slug: "durability", Name: "Multi-file recovery", Difficulty: "hard", Instructions: docs + "08-durability.md", Test: testDurability},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", Test: testGauntlet},
		},
	}
}

// --- RPC wrappers ---

func ping(c *harness.Client) error {
	var res struct {
		Message string `json:"message"`
	}
	if err := c.Call("ping", nil, &res); err != nil {
		return err
	}
	if res.Message != "pong" {
		return fmt.Errorf(`ping result: expected {"message": "pong"}, got message %q`, res.Message)
	}
	return nil
}

func put(c *harness.Client, key, value string) error {
	if err := c.Call("put", map[string]any{"key": key, "value": value}, nil); err != nil {
		return fmt.Errorf("put %q: %w", key, err)
	}
	return nil
}

func del(c *harness.Client, key string) (bool, error) {
	var res struct {
		Deleted bool `json:"deleted"`
	}
	if err := c.Call("del", map[string]any{"key": key}, &res); err != nil {
		return false, fmt.Errorf("del %q: %w", key, err)
	}
	return res.Deleted, nil
}

func flush(c *harness.Client) error {
	return c.Call("flush", nil, nil)
}

func compact(c *harness.Client) error {
	return c.Call("compact", nil, nil)
}

func expectGet(c *harness.Client, key string, wantValue string, wantFound bool) error {
	var res struct {
		Value *string `json:"value"`
		Found bool    `json:"found"`
	}
	if err := c.Call("get", map[string]any{"key": key}, &res); err != nil {
		return fmt.Errorf("get %q: %w", key, err)
	}
	if !wantFound {
		if res.Found {
			got := "<nil>"
			if res.Value != nil {
				got = *res.Value
			}
			return fmt.Errorf("get %q: expected found=false, got found=true with value %q", key, got)
		}
		return nil
	}
	if !res.Found {
		return fmt.Errorf("get %q: expected value %q, got found=false", key, wantValue)
	}
	if res.Value == nil || *res.Value != wantValue {
		got := "<nil>"
		if res.Value != nil {
			got = *res.Value
		}
		return fmt.Errorf("get %q: expected value %q, got %q", key, wantValue, got)
	}
	return nil
}

func scan(c *harness.Client, start, end string) ([][2]string, error) {
	var res struct {
		Entries []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"entries"`
	}
	if err := c.Call("scan", map[string]any{"start": start, "end": end}, &res); err != nil {
		return nil, fmt.Errorf("scan [%q, %q): %w", start, end, err)
	}
	out := make([][2]string, len(res.Entries))
	for i, e := range res.Entries {
		out[i] = [2]string{e.Key, e.Value}
	}
	return out, nil
}

func verifyModel(c *harness.Client, model map[string]string, absent []string) error {
	for k, v := range model {
		if err := expectGet(c, k, v, true); err != nil {
			return err
		}
	}
	for _, k := range absent {
		if err := expectGet(c, k, "", false); err != nil {
			return err
		}
	}
	return nil
}

func restart(ctx *harness.Context, old *harness.Client) (*harness.Client, error) {
	if old != nil {
		old.Close()
	}
	ctx.KillProgram()
	if err := ctx.StartProgram(); err != nil {
		return nil, fmt.Errorf("restarting your program: %w", err)
	}
	return ctx.Dial()
}

func expectScan(c *harness.Client, start, end string, want [][2]string) error {
	got, err := scan(c, start, end)
	if err != nil {
		return err
	}
	if len(got) != len(want) {
		return fmt.Errorf("scan [%q, %q): expected %d entr(y/ies), got %d", start, end, len(want), len(got))
	}
	for i := range want {
		if got[i][0] != want[i][0] || got[i][1] != want[i][1] {
			return fmt.Errorf("scan [%q, %q): entry %d: expected {key:%q value:%q}, got {key:%q value:%q}",
				start, end, i+1, want[i][0], want[i][1], got[i][0], got[i][1])
		}
	}
	return nil
}

// --- stages ---

func testBind(ctx *harness.Context) error {
	ctx.Logf("connecting two concurrent clients to %s", ctx.Addr())
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

func testPutGet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := expectGet(c, "fruit", "", false); err != nil {
		return fmt.Errorf("before any writes: %w", err)
	}
	if err := put(c, "fruit", "apple"); err != nil {
		return err
	}
	if err := expectGet(c, "fruit", "apple", true); err != nil {
		return err
	}
	if err := put(c, "fruit", "avocado"); err != nil {
		return err
	}
	if err := expectGet(c, "fruit", "avocado", true); err != nil {
		return fmt.Errorf("after overwrite: %w", err)
	}
	ctx.Logf("put/get/overwrite work")

	if err := put(c, "color", "blue"); err != nil {
		return err
	}
	deleted, err := del(c, "fruit")
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("del of an existing key: expected deleted=true, got false")
	}
	if err := expectGet(c, "fruit", "", false); err != nil {
		return fmt.Errorf("after delete: %w", err)
	}
	deleted, err = del(c, "fruit")
	if err != nil {
		return err
	}
	if deleted {
		return fmt.Errorf("del of a missing key: expected deleted=false, got true")
	}
	if err := expectGet(c, "color", "blue", true); err != nil {
		return fmt.Errorf("deleting one key must not affect others: %w", err)
	}
	ctx.Logf("delete semantics correct")
	return nil
}

func testFlush(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := put(c, "alpha", "1"); err != nil {
		return err
	}
	if err := put(c, "beta", "2"); err != nil {
		return err
	}
	if err := put(c, "gamma", "3"); err != nil {
		return err
	}
	ctx.Logf("calling flush")
	if err := flush(c); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	ctx.Logf("killing your server (SIGKILL) to inspect the SST file on disk")
	ctx.KillProgram()

	sstFile := sstPath(ctx.DataDir(), 1)
	if _, err := os.Stat(sstFile); err != nil {
		return fmt.Errorf("000001.sst not found in %s/sst/ after flush: %v", ctx.DataDir(), err)
	}
	ctx.Logf("parsing %s", sstFile)
	entries, err := parseSST(sstFile)
	if err != nil {
		return fmt.Errorf("000001.sst does not conform to the SST1 format: %w", err)
	}
	sorted := make([]Entry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key < sorted[j].Key })
	if err := checkSSTEntries(sorted, [][3]any{
		{"alpha", "1", false},
		{"beta", "2", false},
		{"gamma", "3", false},
	}); err != nil {
		return err
	}
	ctx.Logf("000001.sst contains exactly the 3 flushed keys, byte-valid SST1 format, sorted by key")
	return nil
}

func testRestart(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	for _, kv := range [][2]string{{"x", "1"}, {"y", "2"}, {"z", "3"}, {"y", "22"}} {
		if err := put(c, kv[0], kv[1]); err != nil {
			return err
		}
	}
	if _, err := del(c, "z"); err != nil {
		return err
	}
	ctx.Logf("calling flush")
	if err := flush(c); err != nil {
		return fmt.Errorf("flush: %w", err)
	}

	ctx.Logf("killing your server (SIGKILL)...")
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	if err := verifyModel(c, map[string]string{"x": "1", "y": "22"}, []string{"z"}); err != nil {
		return fmt.Errorf("after restart: %w", err)
	}
	ctx.Logf("flushed state (including an overwrite and a delete) survived the crash")

	if err := put(c, "w", "4"); err != nil {
		return err
	}
	if err := flush(c); err != nil {
		return fmt.Errorf("flush after restart: %w", err)
	}
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := verifyModel(c, map[string]string{"x": "1", "y": "22", "w": "4"}, []string{"z"}); err != nil {
		return fmt.Errorf("after second restart (writes made post-recovery must also be durable): %w", err)
	}
	ctx.Logf("post-recovery writes survived a second crash")
	return nil
}

func testScan(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	for _, kv := range [][2]string{{"a", "1"}, {"c", "3"}, {"e", "5"}, {"g", "7"}} {
		if err := put(c, kv[0], kv[1]); err != nil {
			return err
		}
	}
	if err := expectScan(c, "b", "f", [][2]string{{"c", "3"}, {"e", "5"}}); err != nil {
		return fmt.Errorf("scan of memtable only: %w", err)
	}
	ctx.Logf("memtable scan correct")

	if err := flush(c); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	for _, kv := range [][2]string{{"b", "2"}, {"d", "4"}, {"f", "6"}} {
		if err := put(c, kv[0], kv[1]); err != nil {
			return err
		}
	}
	if err := expectScan(c, "a", "h", [][2]string{
		{"a", "1"}, {"b", "2"}, {"c", "3"}, {"d", "4"}, {"e", "5"}, {"f", "6"}, {"g", "7"},
	}); err != nil {
		return fmt.Errorf("scan across memtable + SST: %w", err)
	}
	ctx.Logf("merged memtable + SST scan correct")

	if err := expectScan(c, "c", "f", [][2]string{{"c", "3"}, {"d", "4"}, {"e", "5"}}); err != nil {
		return fmt.Errorf("half-open end boundary: %w", err)
	}
	ctx.Logf("range boundaries [start, end) correct")
	return nil
}

func testCompaction(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := put(c, "key", "v1"); err != nil {
		return err
	}
	if err := flush(c); err != nil {
		return fmt.Errorf("first flush: %w", err)
	}
	if err := put(c, "key", "v2"); err != nil {
		return err
	}
	if err := put(c, "other", "keep"); err != nil {
		return err
	}
	if err := flush(c); err != nil {
		return fmt.Errorf("second flush: %w", err)
	}

	n, err := countSSTFiles(ctx.DataDir())
	if err != nil {
		return err
	}
	if n != 2 {
		return fmt.Errorf("before compact: expected 2 SST file(s), found %d", n)
	}

	ctx.Logf("calling compact")
	if err := compact(c); err != nil {
		return fmt.Errorf("compact: %w", err)
	}

	n, err = countSSTFiles(ctx.DataDir())
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("after compact: expected exactly 1 SST file, found %d", n)
	}
	if err := expectGet(c, "key", "v2", true); err != nil {
		return fmt.Errorf("after compact, latest value must win: %w", err)
	}
	if err := expectGet(c, "other", "keep", true); err != nil {
		return fmt.Errorf("after compact: %w", err)
	}

	ctx.Logf("killing your server to verify the compacted SST on disk")
	ctx.KillProgram()
	paths, err := listSSTFiles(ctx.DataDir())
	if err != nil {
		return err
	}
	if len(paths) != 1 {
		return fmt.Errorf("after compact + kill: expected 1 SST file on disk, found %d", len(paths))
	}
	entries, err := parseSST(paths[0])
	if err != nil {
		return fmt.Errorf("compacted SST does not conform to SST1 format: %w", err)
	}
	if err := checkSSTEntries(entries, [][3]any{
		{"key", "v2", false},
		{"other", "keep", false},
	}); err != nil {
		return fmt.Errorf("compacted SST contents: %w", err)
	}
	ctx.Logf("compaction merged 2 SST files into 1 sorted file with the latest value for overlapping keys")
	return nil
}

func testDelete(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := put(c, "alive", "yes"); err != nil {
		return err
	}
	if err := put(c, "gone", "soon"); err != nil {
		return err
	}
	if err := flush(c); err != nil {
		return fmt.Errorf("flush: %w", err)
	}
	if _, err := del(c, "gone"); err != nil {
		return err
	}
	if err := flush(c); err != nil {
		return fmt.Errorf("flush tombstone: %w", err)
	}

	if err := expectGet(c, "gone", "", false); err != nil {
		return fmt.Errorf("tombstone in memtable: %w", err)
	}
	if err := expectGet(c, "alive", "yes", true); err != nil {
		return fmt.Errorf("non-deleted key must remain: %w", err)
	}

	ctx.Logf("killing your server to verify tombstones survive restart")
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := expectGet(c, "gone", "", false); err != nil {
		return fmt.Errorf("after restart, tombstone must hide the key: %w", err)
	}
	if err := expectGet(c, "alive", "yes", true); err != nil {
		return fmt.Errorf("after restart: %w", err)
	}

	paths, err := listSSTFiles(ctx.DataDir())
	if err != nil {
		return err
	}
	if len(paths) < 2 {
		return fmt.Errorf("expected at least 2 SST files (one with data, one with tombstone), found %d", len(paths))
	}
	foundTombstone := false
	for _, p := range paths {
		entries, err := parseSST(p)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", filepath.Base(p), err)
		}
		for _, e := range entries {
			if e.Key == "gone" && e.Deleted {
				foundTombstone = true
			}
		}
	}
	if !foundTombstone {
		return fmt.Errorf("no SST file contains a tombstone (value_len=0) for key %q", "gone")
	}
	ctx.Logf("tombstones hide keys across memtable, SST, and restart")
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}

	model := map[string]string{}
	for i := 0; i < 8; i++ {
		key := fmt.Sprintf("k%d", i)
		value := fmt.Sprintf("v%d", i)
		if err := put(c, key, value); err != nil {
			return err
		}
		model[key] = value
		if i%3 == 2 {
			ctx.Logf("flush after %d keys", i+1)
			if err := flush(c); err != nil {
				return fmt.Errorf("flush: %w", err)
			}
		}
	}
	if _, err := del(c, "k3"); err != nil {
		return err
	}
	delete(model, "k3")
	if err := put(c, "k1", "updated"); err != nil {
		return err
	}
	model["k1"] = "updated"

	ctx.Logf("flushing remaining memtable before crash")
	if err := flush(c); err != nil {
		return fmt.Errorf("final flush: %w", err)
	}

	ctx.Logf("killing your server with multiple SST files on disk...")
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	if err := verifyModel(c, model, []string{"k3"}); err != nil {
		return fmt.Errorf("after restart with multiple SST files: %w", err)
	}

	n, err := countSSTFiles(ctx.DataDir())
	if err != nil {
		return err
	}
	if n < 2 {
		return fmt.Errorf("expected multiple SST files before compact, found %d", n)
	}
	ctx.Logf("%d SST files on disk, all keys recovered correctly", n)

	ctx.Logf("calling compact then crashing again")
	if err := compact(c); err != nil {
		return fmt.Errorf("compact: %w", err)
	}
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := verifyModel(c, model, []string{"k3"}); err != nil {
		return fmt.Errorf("after compact + restart: %w", err)
	}

	ctx.KillProgram()
	offline, err := parseAllSSTs(ctx.DataDir())
	if err != nil {
		return fmt.Errorf("offline parse of SST files: %w", err)
	}
	for k, want := range model {
		got, ok := offline[k]
		if !ok || got != want {
			return fmt.Errorf("offline reconstruction disagrees for %q: files say %q (present=%v), expected %q",
				k, got, ok, want)
		}
	}
	if _, ok := offline["k3"]; ok {
		return fmt.Errorf("offline reconstruction: key %q should be absent (tombstoned)", "k3")
	}
	ctx.Logf("multi-file recovery and post-compact durability verified")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	rng := rand.New(rand.NewSource(42))
	model := map[string]string{}
	keys := make([]string, 10)
	for i := range keys {
		keys[i] = fmt.Sprintf("g%d", i)
	}
	counter := 0

	for round := 1; round <= 4; round++ {
		ctx.Logf("round %d: 25 random ops%s, then SIGKILL",
			round, map[bool]string{true: " + a flush", false: ""}[round >= 2])
		for i := 0; i < 25; i++ {
			key := keys[rng.Intn(len(keys))]
			if rng.Float64() < 0.65 {
				counter++
				value := fmt.Sprintf("v%d", counter)
				if err := put(c, key, value); err != nil {
					return err
				}
				model[key] = value
			} else {
				wantDeleted := false
				if _, ok := model[key]; ok {
					wantDeleted = true
				}
				deleted, err := del(c, key)
				if err != nil {
					return err
				}
				if deleted != wantDeleted {
					return fmt.Errorf("round %d: del %q returned deleted=%v, expected %v", round, key, deleted, wantDeleted)
				}
				delete(model, key)
			}
			if round >= 2 && i == 12 {
				if err := flush(c); err != nil {
					return fmt.Errorf("round %d: flush: %w", round, err)
				}
			}
			if round >= 3 && i == 20 {
				if err := compact(c); err != nil {
					return fmt.Errorf("round %d: compact: %w", round, err)
				}
			}
		}
		if err := verifyModelGauntlet(c, model, keys, round, "before the crash"); err != nil {
			return err
		}
		if err := flush(c); err != nil {
			return fmt.Errorf("round %d: flush before crash: %w", round, err)
		}
		c, err = restart(ctx, c)
		if err != nil {
			return err
		}
		if err := verifyModelGauntlet(c, model, keys, round, "after the crash"); err != nil {
			return err
		}
	}
	c.Close()

	ctx.KillProgram()
	offline, err := parseAllSSTs(ctx.DataDir())
	if err != nil {
		return fmt.Errorf("offline parse of SST files: %w", err)
	}
	for _, k := range keys {
		want, wantOK := model[k]
		got, gotOK := offline[k]
		if wantOK != gotOK || want != got {
			return fmt.Errorf("offline reconstruction of your SST files disagrees with served state for %q: files say (%q, present=%v), expected (%q, present=%v)",
				k, got, gotOK, want, wantOK)
		}
	}
	ctx.Logf("4 rounds, 100 ops, 4 crashes, 3 flushes, 2 compactions — served state and on-disk files agree throughout")
	return nil
}

func verifyModelGauntlet(c *harness.Client, model map[string]string, keys []string, round int, when string) error {
	for _, k := range keys {
		want, ok := model[k]
		if err := expectGet(c, k, want, ok); err != nil {
			return fmt.Errorf("round %d, %s: %w", round, when, err)
		}
	}
	return nil
}
