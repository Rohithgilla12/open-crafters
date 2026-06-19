// Package logstore implements the stage tests for the "Build your own log"
// challenge: an append-only, replayable log with absolute offsets, consumer
// groups, and retention. See challenges/build-your-own-log/PROTOCOL.md.
package logstore

import (
	"fmt"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-log/stages/"
	return harness.Challenge{
		Slug: "build-your-own-log",
		Name: "Build your own log",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "append-read", Name: "Append and read by offset", Difficulty: "easy", Instructions: docs + "02-append-read.md", Test: testAppendRead},
			{Slug: "durability", Name: "Survive a crash", Difficulty: "medium", Instructions: docs + "03-durability.md", Test: testDurability},
			{Slug: "topics", Name: "Independent topics", Difficulty: "easy", Instructions: docs + "04-topics.md", Test: testTopics},
			{Slug: "consumer-groups", Name: "Consumer group offsets", Difficulty: "medium", Instructions: docs + "05-consumer-groups.md", Test: testConsumerGroups},
			{Slug: "replay", Name: "Replay and batching", Difficulty: "medium", Instructions: docs + "06-replay.md", Test: testReplay},
			{Slug: "retention", Name: "Retention keeps offsets absolute", Difficulty: "hard", Instructions: docs + "07-retention.md", Test: testRetention},
			{Slug: "offset-durability", Name: "Durable offsets and resume", Difficulty: "medium", Instructions: docs + "08-offset-durability.md", Test: testOffsetDurability},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", Test: testGauntlet},
		},
	}
}

// --- RPC wrappers ---

type record struct {
	Offset int    `json:"offset"`
	Value  string `json:"value"`
}

func ping(c *harness.Client) error {
	var res struct {
		Message string `json:"message"`
	}
	if err := c.Call("ping", nil, &res); err != nil {
		return err
	}
	if res.Message != "pong" {
		return fmt.Errorf(`ping result: expected {"message": "pong"}, got %q`, res.Message)
	}
	return nil
}

func appendRec(c *harness.Client, topic, value string) (int, error) {
	var res struct {
		Offset int `json:"offset"`
	}
	if err := c.Call("append", map[string]any{"topic": topic, "value": value}, &res); err != nil {
		return 0, fmt.Errorf("append to %q: %w", topic, err)
	}
	return res.Offset, nil
}

type readResult struct {
	Records    []record `json:"records"`
	NextOffset int      `json:"next_offset"`
}

func readLog(c *harness.Client, topic string, offset, max int) (readResult, bool, error) {
	params := map[string]any{"topic": topic, "offset": offset}
	if max > 0 {
		params["max"] = max
	}
	var res readResult
	err := c.Call("read", params, &res)
	if err != nil {
		if re, ok := err.(*harness.RPCError); ok && re.Code == "OUT_OF_RANGE" {
			return readResult{}, true, nil
		}
		return readResult{}, false, fmt.Errorf("read %q@%d: %w", topic, offset, err)
	}
	return res, false, nil
}

func commitOffset(c *harness.Client, group, topic string, offset int) error {
	return c.Call("commit_offset", map[string]any{"group": group, "topic": topic, "offset": offset}, nil)
}

func committedOffset(c *harness.Client, group, topic string) (int, error) {
	var res struct {
		Offset int `json:"offset"`
	}
	if err := c.Call("committed_offset", map[string]any{"group": group, "topic": topic}, &res); err != nil {
		return 0, err
	}
	return res.Offset, nil
}

func truncate(c *harness.Client, topic string, before int) error {
	return c.Call("truncate", map[string]any{"topic": topic, "before": before}, nil)
}

func stats(c *harness.Client, topic string) (start, end int, err error) {
	var res struct {
		Start int `json:"start_offset"`
		End   int `json:"end_offset"`
	}
	if err := c.Call("stats", map[string]any{"topic": topic}, &res); err != nil {
		return 0, 0, fmt.Errorf("stats %q: %w", topic, err)
	}
	return res.Start, res.End, nil
}

// expectRead reads from offset and asserts the record values + offsets match
// wantValues (offsets are baseOffset, baseOffset+1, …) and next_offset.
func expectRead(c *harness.Client, topic string, offset, max, baseOffset int, wantValues []string, wantNext int) error {
	res, oor, err := readLog(c, topic, offset, max)
	if err != nil {
		return err
	}
	if oor {
		return fmt.Errorf("read %q@%d: unexpected OUT_OF_RANGE", topic, offset)
	}
	if len(res.Records) != len(wantValues) {
		return fmt.Errorf("read %q@%d: expected %d record(s), got %d", topic, offset, len(wantValues), len(res.Records))
	}
	for i, want := range wantValues {
		r := res.Records[i]
		if r.Offset != baseOffset+i || r.Value != want {
			return fmt.Errorf("read %q@%d record %d: expected {offset:%d value:%q}, got {offset:%d value:%q}",
				topic, offset, i, baseOffset+i, want, r.Offset, r.Value)
		}
	}
	if res.NextOffset != wantNext {
		return fmt.Errorf("read %q@%d: expected next_offset=%d, got %d", topic, offset, wantNext, res.NextOffset)
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

func testAppendRead(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	ctx.Logf("appending a, b, c — offsets must be 0, 1, 2")
	for i, v := range []string{"a", "b", "c"} {
		off, err := appendRec(c, "t", v)
		if err != nil {
			return err
		}
		if off != i {
			return fmt.Errorf("append %q: expected offset %d, got %d (offsets are 0-based and monotonic)", v, i, off)
		}
	}

	if err := expectRead(c, "t", 0, 0, 0, []string{"a", "b", "c"}, 3); err != nil {
		return fmt.Errorf("reading the whole log: %w", err)
	}
	if err := expectRead(c, "t", 1, 0, 1, []string{"b", "c"}, 3); err != nil {
		return fmt.Errorf("reading from an offset: %w", err)
	}
	// Reading is non-destructive: the same read again returns the same data.
	if err := expectRead(c, "t", 0, 0, 0, []string{"a", "b", "c"}, 3); err != nil {
		return fmt.Errorf("a log is replayable — re-reading must return the same records: %w", err)
	}
	// Reading at the end returns nothing, with next_offset at the end.
	if err := expectRead(c, "t", 3, 0, 3, nil, 3); err != nil {
		return fmt.Errorf("reading at the end of the log: %w", err)
	}
	ctx.Logf("offsets are monotonic; reads are replayable and non-destructive")
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	for _, v := range []string{"x", "y", "z"} {
		if _, err := appendRec(c, "t", v); err != nil {
			return err
		}
	}
	ctx.Logf("appended 3 records — now SIGKILL")
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	if err := expectRead(c, "t", 0, 0, 0, []string{"x", "y", "z"}, 3); err != nil {
		return fmt.Errorf("appended records (and their offsets) must survive a crash: %w", err)
	}
	// Appends continue from the right offset after recovery.
	off, err := appendRec(c, "t", "w")
	if err != nil {
		return err
	}
	if off != 3 {
		return fmt.Errorf("after recovery the next append must get offset 3, got %d", off)
	}
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	if err := expectRead(c, "t", 0, 0, 0, []string{"x", "y", "z", "w"}, 4); err != nil {
		return fmt.Errorf("a post-recovery append must also be durable: %w", err)
	}
	ctx.Logf("records and offsets survived two crashes; appends continued correctly")
	return nil
}

func testTopics(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// Each topic has its own independent, 0-based offset space.
	for i := 0; i < 3; i++ {
		if off, err := appendRec(c, "orders", fmt.Sprintf("o%d", i)); err != nil || off != i {
			return fmt.Errorf("orders append %d: off=%d err=%v", i, off, err)
		}
	}
	for i := 0; i < 2; i++ {
		if off, err := appendRec(c, "events", fmt.Sprintf("e%d", i)); err != nil || off != i {
			return fmt.Errorf("events append %d: off=%d err=%v (each topic's offsets are independent)", i, off, err)
		}
	}
	if err := expectRead(c, "orders", 0, 0, 0, []string{"o0", "o1", "o2"}, 3); err != nil {
		return err
	}
	if err := expectRead(c, "events", 0, 0, 0, []string{"e0", "e1"}, 2); err != nil {
		return fmt.Errorf("topics must be isolated: %w", err)
	}
	for _, tc := range []struct {
		topic            string
		start, end       int
	}{{"orders", 0, 3}, {"events", 0, 2}, {"ghost", 0, 0}} {
		s, e, err := stats(c, tc.topic)
		if err != nil {
			return err
		}
		if s != tc.start || e != tc.end {
			return fmt.Errorf("stats(%q): expected start=%d end=%d, got start=%d end=%d", tc.topic, tc.start, tc.end, s, e)
		}
	}
	ctx.Logf("topics keep independent offset spaces; stats report start/end")
	return nil
}

func testConsumerGroups(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	for i := 0; i < 5; i++ {
		if _, err := appendRec(c, "t", fmt.Sprintf("v%d", i)); err != nil {
			return err
		}
	}
	// A group with no committed offset starts at 0.
	if off, err := committedOffset(c, "g1", "t"); err != nil || off != 0 {
		return fmt.Errorf("a fresh group must report committed offset 0, got %d (err=%v)", off, err)
	}
	// Commit advances the group's position; it's read back exactly.
	if err := commitOffset(c, "g1", "t", 3); err != nil {
		return err
	}
	if off, err := committedOffset(c, "g1", "t"); err != nil || off != 3 {
		return fmt.Errorf("g1 committed offset: expected 3, got %d (err=%v)", off, err)
	}
	// A different group is independent.
	if off, err := committedOffset(c, "g2", "t"); err != nil || off != 0 {
		return fmt.Errorf("g2 must be independent of g1: expected 0, got %d (err=%v)", off, err)
	}
	// Consuming = read from your committed offset, then commit how far you got.
	res, _, err := readLog(c, "t", 3, 0)
	if err != nil {
		return err
	}
	if len(res.Records) != 2 || res.Records[0].Value != "v3" {
		return fmt.Errorf("g1 should resume at offset 3 and see v3, v4; got %+v", res.Records)
	}
	if err := commitOffset(c, "g1", "t", res.NextOffset); err != nil {
		return err
	}
	if off, _ := committedOffset(c, "g1", "t"); off != 5 {
		return fmt.Errorf("after consuming to the end, g1's committed offset should be 5, got %d", off)
	}
	ctx.Logf("consumer groups track independent offsets; reads don't consume")
	return nil
}

func testReplay(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	for i := 0; i < 6; i++ {
		if _, err := appendRec(c, "t", fmt.Sprintf("m%d", i)); err != nil {
			return err
		}
	}
	// max batches the read and advances next_offset.
	if err := expectRead(c, "t", 0, 2, 0, []string{"m0", "m1"}, 2); err != nil {
		return fmt.Errorf("batched read (max=2): %w", err)
	}
	if err := expectRead(c, "t", 2, 2, 2, []string{"m2", "m3"}, 4); err != nil {
		return fmt.Errorf("next batch: %w", err)
	}
	if err := expectRead(c, "t", 4, 2, 4, []string{"m4", "m5"}, 6); err != nil {
		return fmt.Errorf("final batch: %w", err)
	}
	// A full replay from 0 returns everything, repeatedly.
	all := []string{"m0", "m1", "m2", "m3", "m4", "m5"}
	for i := 0; i < 2; i++ {
		if err := expectRead(c, "t", 0, 100, 0, all, 6); err != nil {
			return fmt.Errorf("full replay must return the whole log every time: %w", err)
		}
	}
	ctx.Logf("max batches reads; the log replays in full from offset 0, every time")
	return nil
}

func testRetention(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	for i := 0; i < 6; i++ {
		if _, err := appendRec(c, "t", fmt.Sprintf("r%d", i)); err != nil {
			return err
		}
	}
	// A consumer committed at offset 1, then falls behind retention.
	if err := commitOffset(c, "slow", "t", 1); err != nil {
		return err
	}

	ctx.Logf("truncating everything before offset 3")
	if err := truncate(c, "t", 3); err != nil {
		return fmt.Errorf("truncate: %w", err)
	}
	// start rises to 3; end is unchanged — offsets are ABSOLUTE, not renumbered.
	if s, e, err := stats(c, "t"); err != nil || s != 3 || e != 6 {
		return fmt.Errorf("after truncate(before=3): expected start=3 end=6, got start=%d end=%d (err=%v) — retention must not renumber offsets", s, e, err)
	}
	// The surviving records keep their original absolute offsets and values.
	if err := expectRead(c, "t", 3, 0, 3, []string{"r3", "r4", "r5"}, 6); err != nil {
		return fmt.Errorf("surviving records must keep their absolute offsets: %w", err)
	}
	// Reading below the retained start is out of range.
	if _, oor, err := readLog(c, "t", 0, 0); err != nil {
		return err
	} else if !oor {
		return fmt.Errorf("reading below the earliest retained offset must return OUT_OF_RANGE")
	}
	// The slow consumer's committed offset (1) is now behind retention — it
	// stays stored, but reading from it is out of range (consumer lag).
	if off, err := committedOffset(c, "slow", "t"); err != nil || off != 1 {
		return fmt.Errorf("retention must not rewrite committed offsets: expected 1, got %d (err=%v)", off, err)
	}
	if _, oor, err := readLog(c, "t", 1, 0); err != nil {
		return err
	} else if !oor {
		return fmt.Errorf("a consumer that fell behind retention must get OUT_OF_RANGE when it reads")
	}
	// New appends keep climbing from the absolute end.
	if off, err := appendRec(c, "t", "r6"); err != nil || off != 6 {
		return fmt.Errorf("append after truncate must get offset 6, got %d (err=%v)", off, err)
	}
	ctx.Logf("retention dropped old records, kept offsets absolute, and left a lagging consumer out of range")
	return nil
}

func testOffsetDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	for i := 0; i < 5; i++ {
		if _, err := appendRec(c, "t", fmt.Sprintf("v%d", i)); err != nil {
			return err
		}
	}
	if err := commitOffset(c, "g1", "t", 2); err != nil {
		return err
	}
	if err := commitOffset(c, "g2", "t", 4); err != nil {
		return err
	}
	if err := truncate(c, "t", 1); err != nil {
		return err
	}
	ctx.Logf("committed offsets for g1=2, g2=4, truncated before 1 — now SIGKILL")

	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	if off, err := committedOffset(c, "g1", "t"); err != nil || off != 2 {
		return fmt.Errorf("g1 committed offset must survive a crash: expected 2, got %d (err=%v)", off, err)
	}
	if off, err := committedOffset(c, "g2", "t"); err != nil || off != 4 {
		return fmt.Errorf("g2 committed offset must survive a crash: expected 4, got %d (err=%v)", off, err)
	}
	if s, e, err := stats(c, "t"); err != nil || s != 1 || e != 5 {
		return fmt.Errorf("retention state must survive a crash: expected start=1 end=5, got start=%d end=%d (err=%v)", s, e, err)
	}
	// g1 resumes from 2 and reads the rest.
	if err := expectRead(c, "t", 2, 0, 2, []string{"v2", "v3", "v4"}, 5); err != nil {
		return fmt.Errorf("g1 must resume from its durable offset after the crash: %w", err)
	}
	ctx.Logf("committed offsets and retention survived the crash; the group resumed cleanly")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	topics := []string{"a", "b", "c"}
	// model: per topic, the full appended history (value by absolute offset)
	// and the current retained start.
	history := map[string][]string{}
	start := map[string]int{}
	groupOff := map[string]int{} // "group/topic" -> committed offset
	counter := 0

	verify := func(when string) error {
		for _, tp := range topics {
			s, e, err := stats(c, tp)
			if err != nil {
				return err
			}
			if s != start[tp] || e != len(history[tp]) {
				return fmt.Errorf("%s: stats(%q) start=%d end=%d, model start=%d end=%d", when, tp, s, e, start[tp], len(history[tp]))
			}
			if e > s { // read all retained, verify values + absolute offsets
				want := history[tp][s:e]
				if err := expectRead(c, tp, s, 1000, s, want, e); err != nil {
					return fmt.Errorf("%s: %w", when, err)
				}
			}
		}
		return nil
	}

	for round := 1; round <= 5; round++ {
		ctx.Logf("round %d: appends, commits, a truncate, then SIGKILL", round)
		// Appends.
		for _, tp := range topics {
			for j := 0; j < 1+round%3; j++ {
				counter++
				val := fmt.Sprintf("%s-%d", tp, counter)
				want := len(history[tp])
				off, err := appendRec(c, tp, val)
				if err != nil {
					return err
				}
				if off != want {
					return fmt.Errorf("round %d: append %q got offset %d, expected %d", round, tp, off, want)
				}
				history[tp] = append(history[tp], val)
			}
		}
		// Commit some group offsets.
		for gi := 1; gi <= 2; gi++ {
			tp := topics[(round+gi)%len(topics)]
			g := fmt.Sprintf("g%d", gi)
			pos := len(history[tp]) // commit at the current end
			if err := commitOffset(c, g, tp, pos); err != nil {
				return err
			}
			groupOff[g+"/"+tp] = pos
		}
		// A retention every other round.
		if round%2 == 0 {
			tp := topics[round%len(topics)]
			before := start[tp] + 1
			if before <= len(history[tp]) {
				if err := truncate(c, tp, before); err != nil {
					return err
				}
				start[tp] = before
			}
		}
		if err := verify(fmt.Sprintf("round %d before crash", round)); err != nil {
			return err
		}
		c, err = restart(ctx, c)
		if err != nil {
			return err
		}
		if err := verify(fmt.Sprintf("round %d after crash", round)); err != nil {
			return err
		}
		// Committed offsets survived too.
		for key, want := range groupOff {
			g, tp := splitKey(key)
			if off, err := committedOffset(c, g, tp); err != nil || off != want {
				return fmt.Errorf("round %d: committed offset %s = %d after crash, expected %d (err=%v)", round, key, off, want, err)
			}
		}
	}
	c.Close()
	ctx.Logf("5 rounds across 3 topics, with commits, retention, and 5 crashes — offsets, records, and group positions all held")
	return nil
}

func splitKey(s string) (group, topic string) {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}
