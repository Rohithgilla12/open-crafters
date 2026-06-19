// Package mvcc implements the stage tests for the "Build your own MVCC"
// challenge: a transactional key-value store with snapshot isolation. See
// challenges/build-your-own-mvcc/PROTOCOL.md.
//
// Everything is graded over the wire and (apart from the crash stage)
// deterministically — MVCC is a logical property, not a timing one.
package mvcc

import (
	"fmt"
	"math/rand"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-mvcc/stages/"
	return harness.Challenge{
		Slug: "build-your-own-mvcc",
		Name: "Build your own MVCC",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "transactions", Name: "Begin, commit, rollback", Difficulty: "easy", Instructions: docs + "02-transactions.md", Test: testTransactions},
			{Slug: "snapshot", Name: "Snapshot isolation", Difficulty: "medium", Instructions: docs + "03-snapshot.md", Test: testSnapshot},
			{Slug: "atomicity", Name: "All-or-nothing commits", Difficulty: "medium", Instructions: docs + "04-atomicity.md", Test: testAtomicity},
			{Slug: "conflicts", Name: "Write-write conflicts", Difficulty: "hard", Instructions: docs + "05-conflicts.md", Test: testConflicts},
			{Slug: "deletes", Name: "Deletes and tombstones", Difficulty: "medium", Instructions: docs + "06-deletes.md", Test: testDeletes},
			{Slug: "durability", Name: "Survive a crash", Difficulty: "medium", Instructions: docs + "07-durability.md", Test: testDurability},
			{Slug: "write-skew", Name: "The isolation boundary", Difficulty: "hard", Instructions: docs + "08-write-skew.md", Test: testWriteSkew},
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

func begin(c *harness.Client) (string, error) {
	var res struct {
		Txn string `json:"txn"`
	}
	if err := c.Call("begin", nil, &res); err != nil {
		return "", fmt.Errorf("begin: %w", err)
	}
	if res.Txn == "" {
		return "", fmt.Errorf("begin must return a non-empty transaction id")
	}
	return res.Txn, nil
}

func set(c *harness.Client, txn, key, value string) error {
	if err := c.Call("set", map[string]any{"txn": txn, "key": key, "value": value}, nil); err != nil {
		return fmt.Errorf("set %q=%q: %w", key, value, err)
	}
	return nil
}

func del(c *harness.Client, txn, key string) error {
	if err := c.Call("delete", map[string]any{"txn": txn, "key": key}, nil); err != nil {
		return fmt.Errorf("delete %q: %w", key, err)
	}
	return nil
}

// commit reports whether the server aborted with CONFLICT (conflicted=true) or
// committed (conflicted=false). A transport/other error is returned as err.
func commit(c *harness.Client, txn string) (conflicted bool, err error) {
	e := c.Call("commit", map[string]any{"txn": txn}, &struct {
		Committed bool `json:"committed"`
	}{})
	if e == nil {
		return false, nil
	}
	if re, ok := e.(*harness.RPCError); ok && re.Code == "CONFLICT" {
		return true, nil
	}
	return false, fmt.Errorf("commit: %w", e)
}

func mustCommit(c *harness.Client, txn string) error {
	conflicted, err := commit(c, txn)
	if err != nil {
		return err
	}
	if conflicted {
		return fmt.Errorf("commit unexpectedly returned CONFLICT")
	}
	return nil
}

func rollback(c *harness.Client, txn string) error {
	if err := c.Call("rollback", map[string]any{"txn": txn}, nil); err != nil {
		return fmt.Errorf("rollback: %w", err)
	}
	return nil
}

// expectGet asserts a key's value/presence within a transaction's view.
func expectGet(c *harness.Client, txn, key, wantValue string, wantFound bool) error {
	var res struct {
		Value *string `json:"value"`
		Found bool    `json:"found"`
	}
	if err := c.Call("get", map[string]any{"txn": txn, "key": key}, &res); err != nil {
		return fmt.Errorf("get %q: %w", key, err)
	}
	if res.Found != wantFound {
		got := "<absent>"
		if res.Value != nil {
			got = fmt.Sprintf("%q", *res.Value)
		}
		return fmt.Errorf("get %q: expected found=%v (value %q), got found=%v (value %s)", key, wantFound, wantValue, res.Found, got)
	}
	if wantFound && (res.Value == nil || *res.Value != wantValue) {
		got := "<nil>"
		if res.Value != nil {
			got = fmt.Sprintf("%q", *res.Value)
		}
		return fmt.Errorf("get %q: expected value %q, got %s", key, wantValue, got)
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

func testTransactions(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	t1, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, t1, "a", "", false); err != nil {
		return fmt.Errorf("empty store: %w", err)
	}
	if err := set(c, t1, "a", "1"); err != nil {
		return err
	}
	if err := expectGet(c, t1, "a", "1", true); err != nil {
		return fmt.Errorf("read-your-writes: a transaction must see its own uncommitted writes: %w", err)
	}
	// Overwrite within the same transaction.
	if err := set(c, t1, "a", "2"); err != nil {
		return err
	}
	if err := expectGet(c, t1, "a", "2", true); err != nil {
		return err
	}

	// An uncommitted write is invisible to another transaction.
	t2, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, t2, "a", "", false); err != nil {
		return fmt.Errorf("uncommitted writes must be invisible to other transactions: %w", err)
	}
	ctx.Logf("read-your-writes works; uncommitted writes stay private")

	if err := mustCommit(c, t1); err != nil {
		return err
	}
	// A transaction begun AFTER the commit sees it.
	t3, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, t3, "a", "2", true); err != nil {
		return fmt.Errorf("a transaction begun after a commit must see it: %w", err)
	}
	ctx.Logf("committed writes are visible to later transactions")

	// Rollback discards.
	t4, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t4, "b", "x"); err != nil {
		return err
	}
	if err := rollback(c, t4); err != nil {
		return err
	}
	t5, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, t5, "b", "", false); err != nil {
		return fmt.Errorf("rolled-back writes must vanish: %w", err)
	}
	ctx.Logf("rollback discards a transaction's writes")
	return nil
}

func testSnapshot(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// Seed k=v0.
	t0, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t0, "k", "v0"); err != nil {
		return err
	}
	if err := mustCommit(c, t0); err != nil {
		return err
	}

	// A reader captures a snapshot containing v0.
	reader, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, reader, "k", "v0", true); err != nil {
		return err
	}
	ctx.Logf("reader's snapshot sees k=v0")

	// A concurrent writer commits v1.
	writer, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, writer, "k", "v1"); err != nil {
		return err
	}
	if err := mustCommit(c, writer); err != nil {
		return err
	}

	// The reader's snapshot is frozen: it must still see v0, repeatedly.
	for i := 0; i < 2; i++ {
		if err := expectGet(c, reader, "k", "v0", true); err != nil {
			return fmt.Errorf("snapshot isolation: an open transaction must NOT see writes committed after it began: %w", err)
		}
	}
	ctx.Logf("after a concurrent commit, the reader still sees v0 (frozen snapshot)")

	// A transaction begun now sees v1.
	reader2, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, reader2, "k", "v1", true); err != nil {
		return fmt.Errorf("a new transaction must see the latest committed value: %w", err)
	}
	ctx.Logf("a fresh snapshot sees v1")
	return nil
}

func testAtomicity(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	t, err := begin(c)
	if err != nil {
		return err
	}
	for _, kv := range [][2]string{{"x", "1"}, {"y", "2"}, {"z", "3"}} {
		if err := set(c, t, kv[0], kv[1]); err != nil {
			return err
		}
	}
	// Before commit, none of the three are visible to another transaction.
	other, err := begin(c)
	if err != nil {
		return err
	}
	for _, k := range []string{"x", "y", "z"} {
		if err := expectGet(c, other, k, "", false); err != nil {
			return fmt.Errorf("a multi-key transaction must be invisible until commit: %w", err)
		}
	}
	if err := mustCommit(c, t); err != nil {
		return err
	}
	// After commit, all three appear together.
	a, err := begin(c)
	if err != nil {
		return err
	}
	for _, kv := range [][2]string{{"x", "1"}, {"y", "2"}, {"z", "3"}} {
		if err := expectGet(c, a, kv[0], kv[1], true); err != nil {
			return fmt.Errorf("commit must apply all writes atomically: %w", err)
		}
	}
	ctx.Logf("a 3-key transaction became visible all at once")

	// Rollback of a multi-key transaction leaves nothing.
	t2, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t2, "x", "99"); err != nil {
		return err
	}
	if err := set(c, t2, "w", "1"); err != nil {
		return err
	}
	if err := rollback(c, t2); err != nil {
		return err
	}
	a2, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, a2, "x", "1", true); err != nil {
		return fmt.Errorf("rollback must not change committed data: %w", err)
	}
	if err := expectGet(c, a2, "w", "", false); err != nil {
		return fmt.Errorf("rollback must discard new keys: %w", err)
	}
	ctx.Logf("rollback left committed data untouched and new keys absent")
	return nil
}

func testConflicts(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// Seed k=base.
	t0, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t0, "k", "base"); err != nil {
		return err
	}
	if err := mustCommit(c, t0); err != nil {
		return err
	}

	// Two concurrent transactions write the same key.
	t1, err := begin(c)
	if err != nil {
		return err
	}
	t2, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t1, "k", "t1"); err != nil {
		return err
	}
	if err := set(c, t2, "k", "t2"); err != nil {
		return err
	}
	if err := mustCommit(c, t1); err != nil {
		return fmt.Errorf("the first committer must succeed: %w", err)
	}
	conflicted, err := commit(c, t2)
	if err != nil {
		return err
	}
	if !conflicted {
		return fmt.Errorf("the second transaction wrote a key committed by a concurrent transaction after its snapshot — commit must return CONFLICT (lost-update prevention)")
	}
	ctx.Logf("first committer won; the second's commit was rejected with CONFLICT")

	// The winner's value stands.
	v, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, v, "k", "t1", true); err != nil {
		return fmt.Errorf("after the conflict, the first committer's value must stand: %w", err)
	}

	// Disjoint writes never conflict.
	a, err := begin(c)
	if err != nil {
		return err
	}
	b, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, a, "p", "1"); err != nil {
		return err
	}
	if err := set(c, b, "q", "1"); err != nil {
		return err
	}
	if err := mustCommit(c, a); err != nil {
		return err
	}
	if err := mustCommit(c, b); err != nil {
		return fmt.Errorf("disjoint writes must not conflict: %w", err)
	}
	ctx.Logf("concurrent transactions writing different keys both committed")

	// A transaction whose snapshot already includes the latest write does not
	// conflict when it writes that key.
	t3, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t3, "k", "t3"); err != nil {
		return err
	}
	if err := mustCommit(c, t3); err != nil {
		return fmt.Errorf("a non-concurrent write (snapshot already current) must not conflict: %w", err)
	}
	ctx.Logf("a write whose snapshot was current committed cleanly")
	return nil
}

func testDeletes(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	seed := func(key, val string) error {
		t, err := begin(c)
		if err != nil {
			return err
		}
		if err := set(c, t, key, val); err != nil {
			return err
		}
		return mustCommit(c, t)
	}
	if err := seed("k", "v"); err != nil {
		return err
	}

	// Delete within a transaction is visible to itself, then to later ones.
	t1, err := begin(c)
	if err != nil {
		return err
	}
	if err := del(c, t1, "k"); err != nil {
		return err
	}
	if err := expectGet(c, t1, "k", "", false); err != nil {
		return fmt.Errorf("a transaction must see its own delete: %w", err)
	}
	if err := mustCommit(c, t1); err != nil {
		return err
	}
	t2, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, t2, "k", "", false); err != nil {
		return fmt.Errorf("a committed delete must be visible to later transactions: %w", err)
	}
	ctx.Logf("delete commits a tombstone; the key reads as absent")

	// Re-setting a deleted key brings it back.
	if err := seed("k", "again"); err != nil {
		return err
	}
	t3, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, t3, "k", "again", true); err != nil {
		return fmt.Errorf("re-setting a deleted key must restore it: %w", err)
	}

	// A reader's snapshot survives a concurrent delete.
	reader, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, reader, "k", "again", true); err != nil {
		return err
	}
	deleter, err := begin(c)
	if err != nil {
		return err
	}
	if err := del(c, deleter, "k"); err != nil {
		return err
	}
	if err := mustCommit(c, deleter); err != nil {
		return err
	}
	if err := expectGet(c, reader, "k", "again", true); err != nil {
		return fmt.Errorf("a delete committed after a reader's snapshot must be invisible to it: %w", err)
	}
	ctx.Logf("a concurrent delete didn't disturb the reader's snapshot")

	// delete-vs-write is still a write-write conflict.
	if err := seed("m", "x"); err != nil {
		return err
	}
	ta, err := begin(c)
	if err != nil {
		return err
	}
	tb, err := begin(c)
	if err != nil {
		return err
	}
	if err := del(c, ta, "m"); err != nil {
		return err
	}
	if err := set(c, tb, "m", "y"); err != nil {
		return err
	}
	if err := mustCommit(c, ta); err != nil {
		return err
	}
	conflicted, err := commit(c, tb)
	if err != nil {
		return err
	}
	if !conflicted {
		return fmt.Errorf("a write racing a concurrent delete of the same key must CONFLICT")
	}
	ctx.Logf("delete-vs-write conflict detected")
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}

	t1, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t1, "d1", "1"); err != nil {
		return err
	}
	if err := mustCommit(c, t1); err != nil {
		return err
	}
	// An open, uncommitted transaction at crash time.
	t2, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t2, "d2", "2"); err != nil {
		return err
	}
	ctx.Logf("committed d1; left d2 uncommitted — now SIGKILL")

	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	a, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, a, "d1", "1", true); err != nil {
		return fmt.Errorf("a committed value must survive a crash: %w", err)
	}
	if err := expectGet(c, a, "d2", "", false); err != nil {
		return fmt.Errorf("an uncommitted write must NOT survive a crash: %w", err)
	}
	ctx.Logf("committed survived, uncommitted vanished")

	// Writes made after recovery are durable too.
	t3, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t3, "d3", "3"); err != nil {
		return err
	}
	if err := mustCommit(c, t3); err != nil {
		return err
	}
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	b, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, b, "d3", "3", true); err != nil {
		return fmt.Errorf("a post-recovery commit must also be durable: %w", err)
	}
	ctx.Logf("post-recovery commit survived a second crash")
	return nil
}

func testWriteSkew(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// Seed x=0, y=0.
	t0, err := begin(c)
	if err != nil {
		return err
	}
	if err := set(c, t0, "x", "0"); err != nil {
		return err
	}
	if err := set(c, t0, "y", "0"); err != nil {
		return err
	}
	if err := mustCommit(c, t0); err != nil {
		return err
	}

	// Two transactions each read both keys, then write a DIFFERENT key.
	t1, err := begin(c)
	if err != nil {
		return err
	}
	t2, err := begin(c)
	if err != nil {
		return err
	}
	for _, tx := range []string{t1, t2} {
		if err := expectGet(c, tx, "x", "0", true); err != nil {
			return err
		}
		if err := expectGet(c, tx, "y", "0", true); err != nil {
			return err
		}
	}
	if err := set(c, t1, "x", "1"); err != nil {
		return err
	}
	if err := set(c, t2, "y", "1"); err != nil {
		return err
	}
	if err := mustCommit(c, t1); err != nil {
		return err
	}
	// Under snapshot isolation this must succeed (write skew is allowed): the
	// two transactions wrote disjoint keys, so there is no write-write
	// conflict — even though each read what the other changed.
	conflicted, err := commit(c, t2)
	if err != nil {
		return err
	}
	if conflicted {
		return fmt.Errorf("disjoint writes must not conflict under snapshot isolation — this is write skew, which SI permits. Only write-write conflicts on the SAME key abort.")
	}
	ctx.Logf("write skew allowed: disjoint writes both committed (this is snapshot isolation, not serializable)")

	v, err := begin(c)
	if err != nil {
		return err
	}
	if err := expectGet(c, v, "x", "1", true); err != nil {
		return err
	}
	if err := expectGet(c, v, "y", "1", true); err != nil {
		return err
	}
	ctx.Logf("final state x=1, y=1")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	rng := rand.New(rand.NewSource(7))
	keys := []string{"g0", "g1", "g2", "g3", "g4"}
	model := map[string]string{} // committed values; absent = not found
	counter := 0

	verifyWorld := func(when string) error {
		v, err := begin(c)
		if err != nil {
			return err
		}
		for _, k := range keys {
			want, ok := model[k]
			if err := expectGet(c, v, k, want, ok); err != nil {
				return fmt.Errorf("%s: %w", when, err)
			}
		}
		return rollback(c, v)
	}

	for round := 1; round <= 5; round++ {
		// Open several transactions at the SAME snapshot (all begin before any
		// of them commit), so they are genuinely concurrent.
		n := 3
		txns := make([]string, n)
		for i := 0; i < n; i++ {
			txns[i], err = begin(c)
			if err != nil {
				return err
			}
		}
		ctx.Logf("round %d: %d concurrent transactions, then SIGKILL", round, n)

		// All txns share the pre-round snapshot, so their reads must equal the
		// model as it was before any of this round's commits.
		preRound := map[string]string{}
		for k, v := range model {
			preRound[k] = v
		}

		committedKeys := map[string]bool{} // keys committed by earlier txns this round
		for i := 0; i < n; i++ {
			tx := txns[i]
			for _, k := range keys {
				want, ok := preRound[k]
				if err := expectGet(c, tx, k, want, ok); err != nil {
					return fmt.Errorf("round %d txn %d snapshot read: %w", round, i, err)
				}
			}
			// Write 1-2 keys.
			nw := 1 + rng.Intn(2)
			chosen := map[string]string{}
			for j := 0; j < nw; j++ {
				k := keys[rng.Intn(len(keys))]
				counter++
				val := fmt.Sprintf("r%d-%d", round, counter)
				if err := set(c, tx, k, val); err != nil {
					return err
				}
				chosen[k] = val
			}
			// Conflict expected iff any chosen key was already committed this
			// round by an earlier transaction.
			expectConflict := false
			for k := range chosen {
				if committedKeys[k] {
					expectConflict = true
				}
			}
			conflicted, err := commit(c, tx)
			if err != nil {
				return err
			}
			if conflicted != expectConflict {
				return fmt.Errorf("round %d txn %d: expected conflict=%v, got %v (keys %v already-committed-this-round %v)",
					round, i, expectConflict, conflicted, keysOf(chosen), committedKeys)
			}
			if !conflicted {
				for k, val := range chosen {
					model[k] = val
					committedKeys[k] = true
				}
			}
		}

		if err := verifyWorld(fmt.Sprintf("round %d before crash", round)); err != nil {
			return err
		}
		c, err = restart(ctx, c)
		if err != nil {
			return err
		}
		if err := verifyWorld(fmt.Sprintf("round %d after crash", round)); err != nil {
			return err
		}
	}
	c.Close()
	ctx.Logf("5 rounds, 15 concurrent transactions, 5 crashes — every conflict and commit matched the model")
	return nil
}

func keysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
