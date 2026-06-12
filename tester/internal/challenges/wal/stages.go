package wal

import (
	"fmt"
	"math/rand"
	"os"

	"github.com/open-crafters/open-crafters/tester/internal/harness"
)

func Challenge() harness.Challenge {
	return harness.Challenge{
		Slug: "build-your-own-wal",
		Name: "Build your own WAL",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Test: testBind},
			{Slug: "kv", Name: "An in-memory key-value store", Test: testKV},
			{Slug: "persist", Name: "Survive a crash", Test: testPersist},
			{Slug: "format", Name: "Write the log format", Test: testFormat},
			{Slug: "replay", Name: "Recover from any log", Test: testReplay},
			{Slug: "torn-writes", Name: "Torn writes", Test: testTornWrites},
			{Slug: "checksums", Name: "Detect corruption", Test: testChecksums},
			{Slug: "checkpoint", Name: "Snapshots and log truncation", Test: testCheckpoint},
			{Slug: "gauntlet", Name: "The gauntlet", Test: testGauntlet},
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

func set(c *harness.Client, key, value string) error {
	if err := c.Call("set", map[string]any{"key": key, "value": value}, nil); err != nil {
		return fmt.Errorf("set %q: %w", key, err)
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

func checkpoint(c *harness.Client) error {
	return c.Call("checkpoint", nil, nil)
}

// expectGet asserts a key's presence and value.
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

// verifyModel checks every key in the model, plus that absent keys are absent.
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

// restart kills the program (SIGKILL) and starts it again, returning a fresh
// connection.
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

// checkRecords compares parsed records against expected {op, key, value}
// triples (value ignored for del).
func checkRecords(records []Record, want [][3]string) error {
	if len(records) != len(want) {
		return fmt.Errorf("expected %d record(s) in wal.log, found %d", len(want), len(records))
	}
	for i, w := range want {
		r := records[i]
		if r.Op != w[0] || r.Key != w[1] || (w[0] == "set" && r.Value != w[2]) {
			return fmt.Errorf("record %d: expected {op:%s key:%s value:%s}, got {op:%s key:%s value:%s}",
				i+1, w[0], w[1], w[2], r.Op, r.Key, r.Value)
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

func testKV(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := expectGet(c, "fruit", "", false); err != nil {
		return fmt.Errorf("before any writes: %w", err)
	}
	if err := set(c, "fruit", "apple"); err != nil {
		return err
	}
	if err := expectGet(c, "fruit", "apple", true); err != nil {
		return err
	}
	if err := set(c, "fruit", "avocado"); err != nil {
		return err
	}
	if err := expectGet(c, "fruit", "avocado", true); err != nil {
		return fmt.Errorf("after overwrite: %w", err)
	}
	ctx.Logf("set/get/overwrite work")

	if err := set(c, "color", "blue"); err != nil {
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

func testPersist(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	for _, kv := range [][2]string{{"x", "1"}, {"y", "2"}, {"z", "3"}, {"y", "22"}} {
		if err := set(c, kv[0], kv[1]); err != nil {
			return err
		}
	}
	if _, err := del(c, "z"); err != nil {
		return err
	}

	ctx.Logf("killing your server (SIGKILL)...")
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	if err := verifyModel(c, map[string]string{"x": "1", "y": "22"}, []string{"z"}); err != nil {
		return fmt.Errorf("after restart: %w", err)
	}
	ctx.Logf("acknowledged writes (including an overwrite and a delete) survived the crash")

	// Writes made after a recovery must be just as durable.
	if err := set(c, "w", "4"); err != nil {
		return err
	}
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := verifyModel(c, map[string]string{"x": "1", "y": "22", "w": "4"}, nil); err != nil {
		return fmt.Errorf("after second restart (writes made post-recovery must also be durable): %w", err)
	}
	ctx.Logf("post-recovery writes survived a second crash")
	return nil
}

func testFormat(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := set(c, "fruit", "apple"); err != nil {
		return err
	}
	if err := set(c, "color", "blue"); err != nil {
		return err
	}
	if _, err := del(c, "fruit"); err != nil {
		return err
	}
	if err := set(c, "color", "green"); err != nil {
		return err
	}

	// The records must already be on disk: durability means write-then-ack.
	ctx.Logf("parsing %s/wal.log", ctx.DataDir())
	if _, err := os.Stat(walPath(ctx.DataDir())); err != nil {
		return fmt.Errorf("wal.log not found in --data-dir after acknowledged writes: %v", err)
	}
	records, err := parseWAL(ctx.DataDir(), true)
	if err != nil {
		return fmt.Errorf("wal.log does not conform to the record format: %w", err)
	}
	if err := checkRecords(records, [][3]string{
		{"set", "fruit", "apple"},
		{"set", "color", "blue"},
		{"del", "fruit", ""},
		{"set", "color", "green"},
	}); err != nil {
		return err
	}
	ctx.Logf("wal.log contains exactly the 4 acknowledged operations, CRCs valid, in order")
	return nil
}

func testReplay(ctx *harness.Context) error {
	// Replace the (possibly empty) log with one the tester crafted: recovery
	// must work from any spec-conformant log, not just ones your code wrote.
	ctx.KillProgram()
	ops := [][3]string{
		{"set", "name", "Ada Lovelace"},
		{"set", "motto", "première programmeuse 🚀"},
		{"set", "tmp", "scratch"},
		{"set", "empty", ""},
		{"del", "tmp", ""},
		{"set", "motto", "notes on the analytical engine"},
	}
	if err := writeWAL(ctx.DataDir(), ops); err != nil {
		return err
	}
	ctx.Logf("wrote a crafted wal.log with 6 records (overwrites, a delete, unicode, an empty value)")
	if err := ctx.StartProgram(); err != nil {
		return fmt.Errorf("starting your program against the crafted log: %w", err)
	}
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if err := verifyModel(c, map[string]string{
		"name":  "Ada Lovelace",
		"motto": "notes on the analytical engine",
		"empty": "",
	}, []string{"tmp"}); err != nil {
		return fmt.Errorf("state recovered from the crafted log: %w", err)
	}
	ctx.Logf("recovered state matches the crafted log exactly")
	return nil
}

func testTornWrites(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	for i := 1; i <= 5; i++ {
		if err := set(c, fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i)); err != nil {
			return err
		}
	}
	c.Close()
	ctx.KillProgram()

	ctx.Logf("truncating the last 3 bytes of wal.log (simulating a torn write)")
	if err := truncateWALBy(ctx.DataDir(), 3); err != nil {
		return err
	}
	if err := ctx.StartProgram(); err != nil {
		return fmt.Errorf("your program must start even with a torn log tail: %w", err)
	}
	c, err = ctx.Dial()
	if err != nil {
		return err
	}
	if err := verifyModel(c,
		map[string]string{"k1": "v1", "k2": "v2", "k3": "v3", "k4": "v4"},
		[]string{"k5"}); err != nil {
		return fmt.Errorf("after recovering a torn log (the torn final record must be discarded, everything before it kept): %w", err)
	}
	ctx.Logf("recovered the 4 complete records, discarded the torn one")

	// The torn tail must have been cleaned up: after a new write the log must
	// parse cleanly from byte 0.
	if err := set(c, "k6", "v6"); err != nil {
		return err
	}
	records, err := parseWAL(ctx.DataDir(), true)
	if err != nil {
		return fmt.Errorf("wal.log must parse cleanly after recovery + a new append (did you truncate the torn tail before appending?): %w", err)
	}
	if err := checkRecords(records, [][3]string{
		{"set", "k1", "v1"}, {"set", "k2", "v2"}, {"set", "k3", "v3"}, {"set", "k4", "v4"}, {"set", "k6", "v6"},
	}); err != nil {
		return fmt.Errorf("wal.log after recovery + append: %w", err)
	}
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := verifyModel(c, map[string]string{"k4": "v4", "k6": "v6"}, []string{"k5"}); err != nil {
		return fmt.Errorf("after another crash: %w", err)
	}
	ctx.Logf("log is clean and post-recovery appends are durable")
	return nil
}

func testChecksums(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	for i := 1; i <= 5; i++ {
		if err := set(c, fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i)); err != nil {
			return err
		}
	}
	c.Close()
	ctx.KillProgram()

	records, err := parseWAL(ctx.DataDir(), true)
	if err != nil {
		return fmt.Errorf("parsing wal.log before corrupting it: %w", err)
	}
	if len(records) != 5 {
		return fmt.Errorf("expected 5 records in wal.log before corruption, found %d", len(records))
	}
	target := records[2] // record 3 of 5: corruption in the *middle* of the log
	mid := target.PayloadOffset + (target.End-target.PayloadOffset)/2
	ctx.Logf("flipping a byte inside record 3 of 5 (offset %d)", mid)
	if err := flipByte(ctx.DataDir(), mid); err != nil {
		return err
	}

	if err := ctx.StartProgram(); err != nil {
		return fmt.Errorf("your program must start even with a corrupt record in the log: %w", err)
	}
	c, err = ctx.Dial()
	if err != nil {
		return err
	}
	// Records 4 and 5 are intact, but recovery must stop at the first invalid
	// record: the log's meaning after a corruption is undefined.
	if err := verifyModel(c,
		map[string]string{"k1": "v1", "k2": "v2"},
		[]string{"k3", "k4", "k5"}); err != nil {
		return fmt.Errorf("after recovering a corrupted log (recovery must stop at the FIRST invalid record, even though later records are intact): %w", err)
	}
	ctx.Logf("recovery stopped at the corrupt record; k3–k5 discarded, k1–k2 served")

	if err := set(c, "fresh", "new"); err != nil {
		return err
	}
	records, err = parseWAL(ctx.DataDir(), true)
	if err != nil {
		return fmt.Errorf("wal.log must parse cleanly after recovery + a new append (the corrupt tail must be truncated): %w", err)
	}
	if err := checkRecords(records, [][3]string{
		{"set", "k1", "v1"}, {"set", "k2", "v2"}, {"set", "fresh", "new"},
	}); err != nil {
		return fmt.Errorf("wal.log after corruption recovery + append: %w", err)
	}
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := verifyModel(c, map[string]string{"k2": "v2", "fresh": "new"}, []string{"k5"}); err != nil {
		return fmt.Errorf("after another crash: %w", err)
	}
	ctx.Logf("corrupt tail truncated; new writes durable")
	return nil
}

func testCheckpoint(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	model := map[string]string{}
	for i := 0; i < 12; i++ {
		key := fmt.Sprintf("key%d", i%6) // 6 keys, each written twice
		value := fmt.Sprintf("v%d", i)
		if err := set(c, key, value); err != nil {
			return err
		}
		model[key] = value
	}
	if _, err := del(c, "key5"); err != nil {
		return err
	}
	delete(model, "key5")

	ctx.Logf("calling checkpoint")
	if err := checkpoint(c); err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	snap, err := readSnapshot(ctx.DataDir())
	if err != nil {
		return fmt.Errorf("after checkpoint: %w", err)
	}
	if snap == nil {
		return fmt.Errorf("after checkpoint: snapshot.json does not exist in --data-dir")
	}
	for k, v := range model {
		if snap[k] != v {
			return fmt.Errorf("snapshot.json: expected %q=%q, got %q", k, v, snap[k])
		}
	}
	if len(snap) != len(model) {
		return fmt.Errorf("snapshot.json: expected exactly %d key(s), found %d", len(model), len(snap))
	}
	records, err := parseWAL(ctx.DataDir(), true)
	if err != nil {
		return err
	}
	if len(records) != 0 {
		return fmt.Errorf("after checkpoint: wal.log must be empty (or absent), found %d record(s)", len(records))
	}
	ctx.Logf("snapshot.json matches state, wal.log reset")

	// Post-checkpoint writes go to the (now small) log.
	if err := set(c, "after", "checkpoint"); err != nil {
		return err
	}
	model["after"] = "checkpoint"
	records, err = parseWAL(ctx.DataDir(), true)
	if err != nil {
		return err
	}
	if err := checkRecords(records, [][3]string{{"set", "after", "checkpoint"}}); err != nil {
		return fmt.Errorf("wal.log after checkpoint + one write: %w", err)
	}
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	if err := verifyModel(c, model, []string{"key5"}); err != nil {
		return fmt.Errorf("after crash: recovery must load snapshot.json then replay wal.log on top: %w", err)
	}
	ctx.Logf("snapshot + log recovery correct after a crash")
	c.Close()

	// Recovery from tester-crafted snapshot + log: the log must win for
	// overlapping keys.
	ctx.KillProgram()
	if err := writeSnapshot(ctx.DataDir(), map[string]string{"s1": "old", "s2": "keep"}); err != nil {
		return err
	}
	if err := writeWAL(ctx.DataDir(), [][3]string{{"set", "s1", "new"}, {"set", "s3", "extra"}}); err != nil {
		return err
	}
	ctx.Logf("wrote a crafted snapshot.json + wal.log with an overlapping key")
	if err := ctx.StartProgram(); err != nil {
		return err
	}
	c, err = ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	if err := verifyModel(c, map[string]string{"s1": "new", "s2": "keep", "s3": "extra"}, nil); err != nil {
		return fmt.Errorf("recovery from crafted snapshot + log (the log must override the snapshot): %w", err)
	}
	ctx.Logf("crafted snapshot + log recovered with correct precedence")
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
			round, map[bool]string{true: " + a checkpoint", false: ""}[round >= 2])
		for i := 0; i < 25; i++ {
			key := keys[rng.Intn(len(keys))]
			if rng.Float64() < 0.7 {
				counter++
				value := fmt.Sprintf("v%d", counter)
				if err := set(c, key, value); err != nil {
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
				if err := checkpoint(c); err != nil {
					return fmt.Errorf("round %d: checkpoint: %w", round, err)
				}
			}
		}
		if err := verifyModelGauntlet(c, model, keys, round, "before the crash"); err != nil {
			return err
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

	// Offline check: the durable files themselves must reconstruct to the
	// same state, per spec.
	ctx.KillProgram()
	state, err := reconstructState(ctx.DataDir())
	if err != nil {
		return fmt.Errorf("offline parse of snapshot.json + wal.log: %w", err)
	}
	for _, k := range keys {
		want, wantOK := model[k]
		got, gotOK := state[k]
		if wantOK != gotOK || want != got {
			return fmt.Errorf("offline reconstruction of your files disagrees with served state for %q: files say (%q, present=%v), expected (%q, present=%v)",
				k, got, gotOK, want, wantOK)
		}
	}
	ctx.Logf("4 rounds, 100 ops, 4 crashes, 3 checkpoints — served state and on-disk files agree throughout")
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
