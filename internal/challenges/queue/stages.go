// Package queue implements the stage tests for the "Build your own message
// queue" challenge. See challenges/build-your-own-queue/PROTOCOL.md.
//
// Everything here is graded over the wire: the tester acts as a producer and
// a set of consumers, and SIGKILLs the process to check at-least-once
// durability. Unlike the WAL challenge, the on-disk format is not inspected.
package queue

import (
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-queue/stages/"
	return harness.Challenge{
		Slug: "build-your-own-queue",
		Name: "Build your own message queue",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "send-receive", Name: "Send, receive, ack", Difficulty: "easy", Instructions: docs + "02-send-receive.md", Test: testSendReceive},
			{Slug: "durability", Name: "Survive a crash", Difficulty: "medium", Instructions: docs + "03-durability.md", Test: testDurability},
			{Slug: "redelivery", Name: "Visibility timeouts", Difficulty: "medium", Instructions: docs + "04-redelivery.md", Test: testRedelivery},
			{Slug: "nack", Name: "Negative acknowledgement", Difficulty: "easy", Instructions: docs + "05-nack.md", Test: testNack},
			{Slug: "fencing", Name: "Receipt fencing", Difficulty: "hard", Instructions: docs + "06-fencing.md", Test: testFencing},
			{Slug: "dead-letter", Name: "Dead-letter queues", Difficulty: "hard", Instructions: docs + "07-dead-letter.md", Test: testDeadLetter},
			{Slug: "queues", Name: "Many queues and stats", Difficulty: "medium", Instructions: docs + "08-queues.md", Test: testQueues},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", Test: testGauntlet},
		},
	}
}

// --- RPC wrappers ---

type recvMsg struct {
	ID       string `json:"id"`
	Body     string `json:"body"`
	Receipt  string `json:"receipt"`
	Receives int    `json:"receives"`
}

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

func send(c *harness.Client, queue, body string) (string, error) {
	var res struct {
		ID string `json:"id"`
	}
	if err := c.Call("send", map[string]any{"queue": queue, "body": body}, &res); err != nil {
		return "", fmt.Errorf("send to %q: %w", queue, err)
	}
	if res.ID == "" {
		return "", fmt.Errorf("send to %q returned an empty id (each message needs a unique server-assigned id)", queue)
	}
	return res.ID, nil
}

// receive polls once. visMs <= 0 omits visibility_timeout_ms (server default).
func receive(c *harness.Client, queue string, visMs int) (*recvMsg, error) {
	params := map[string]any{"queue": queue}
	if visMs > 0 {
		params["visibility_timeout_ms"] = visMs
	}
	var res struct {
		Message *recvMsg `json:"message"`
	}
	if err := c.Call("receive", params, &res); err != nil {
		return nil, fmt.Errorf("receive on %q: %w", queue, err)
	}
	if res.Message != nil && res.Message.Receipt == "" {
		return nil, fmt.Errorf("receive on %q returned a message with an empty receipt (you need a receipt to ack it)", queue)
	}
	return res.Message, nil
}

// receiveWithin polls receive until it yields a message or `within` elapses.
func receiveWithin(c *harness.Client, queue string, visMs int, within time.Duration) (*recvMsg, error) {
	deadline := time.Now().Add(within)
	for {
		m, err := receive(c, queue, visMs)
		if err != nil {
			return nil, err
		}
		if m != nil {
			return m, nil
		}
		if !time.Now().Before(deadline) {
			return nil, nil
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func ack(c *harness.Client, queue, receipt string) (bool, error) {
	var res struct {
		Acked bool `json:"acked"`
	}
	if err := c.Call("ack", map[string]any{"queue": queue, "receipt": receipt}, &res); err != nil {
		return false, fmt.Errorf("ack on %q: %w", queue, err)
	}
	return res.Acked, nil
}

func nack(c *harness.Client, queue, receipt string) (bool, error) {
	var res struct {
		Nacked bool `json:"nacked"`
	}
	if err := c.Call("nack", map[string]any{"queue": queue, "receipt": receipt}, &res); err != nil {
		return false, fmt.Errorf("nack on %q: %w", queue, err)
	}
	return res.Nacked, nil
}

func stats(c *harness.Client, queue string) (visible, inflight int, err error) {
	var res struct {
		Visible  int `json:"visible"`
		Inflight int `json:"inflight"`
	}
	if err := c.Call("stats", map[string]any{"queue": queue}, &res); err != nil {
		return 0, 0, fmt.Errorf("stats on %q: %w", queue, err)
	}
	return res.Visible, res.Inflight, nil
}

func configure(c *harness.Client, queue string, maxReceives int, dlq string) error {
	return c.Call("configure", map[string]any{
		"queue":             queue,
		"max_receives":      maxReceives,
		"dead_letter_queue": dlq,
	}, nil)
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

func testSendReceive(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// An empty queue yields null, not an error.
	ctx.Logf("receiving from an empty queue")
	m, err := receive(c, "q", 0)
	if err != nil {
		return err
	}
	if m != nil {
		return fmt.Errorf("receive on an empty queue must return null, got message %q", m.Body)
	}

	ctx.Logf("sending a, b, c then receiving them in order")
	ids := map[string]string{}
	for _, body := range []string{"a", "b", "c"} {
		id, err := send(c, "q", body)
		if err != nil {
			return err
		}
		ids[body] = id
	}
	if ids["a"] == ids["b"] || ids["b"] == ids["c"] {
		return fmt.Errorf("send must assign a distinct id per message; got duplicates among %v", ids)
	}

	// FIFO: oldest first. Each receive hands out the next message, in-flight
	// under the default (long) timeout, so it is not handed out twice.
	var receipts []string
	var prevReceipt string
	for i, want := range []string{"a", "b", "c"} {
		m, err := receive(c, "q", 0)
		if err != nil {
			return err
		}
		if m == nil {
			return fmt.Errorf("receive #%d returned null, expected message %q (in-flight messages must not block later visible ones)", i+1, want)
		}
		if m.Body != want {
			return fmt.Errorf("receive #%d: expected body %q (FIFO, oldest first), got %q", i+1, want, m.Body)
		}
		if m.ID != ids[want] {
			return fmt.Errorf("receive #%d: body %q came back with id %q, but send assigned it %q", i+1, want, m.ID, ids[want])
		}
		if m.Receives != 1 {
			return fmt.Errorf("receive #%d: first delivery of %q must report receives=1, got %d", i+1, want, m.Receives)
		}
		if m.Receipt == prevReceipt {
			return fmt.Errorf("each delivery needs its own receipt; %q reused the previous one", want)
		}
		prevReceipt = m.Receipt
		receipts = append(receipts, m.Receipt)
	}

	// All three are now in-flight; nothing visible.
	m, err = receive(c, "q", 0)
	if err != nil {
		return err
	}
	if m != nil {
		return fmt.Errorf("all messages are in-flight, so receive must return null, got %q", m.Body)
	}
	ctx.Logf("FIFO ordering, unique ids and receipts, in-flight hiding all correct")

	// ack semantics.
	acked, err := ack(c, "q", receipts[1]) // ack "b"
	if err != nil {
		return err
	}
	if !acked {
		return fmt.Errorf("ack of an in-flight message must return acked=true")
	}
	acked, err = ack(c, "q", receipts[1])
	if err != nil {
		return err
	}
	if acked {
		return fmt.Errorf("acking the same receipt twice must return acked=false the second time (the message is already gone)")
	}
	acked, err = ack(c, "q", "no-such-receipt")
	if err != nil {
		return err
	}
	if acked {
		return fmt.Errorf("ack of an unknown receipt must return acked=false")
	}
	ctx.Logf("ack removes once, then reports acked=false")
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	for _, body := range []string{"x", "y", "z"} {
		if _, err := send(c, "q", body); err != nil {
			return err
		}
	}

	// Receive x and ack it; receive y and leave it in-flight; z stays visible.
	m, err := receive(c, "q", 0)
	if err != nil {
		return err
	}
	if m == nil || m.Body != "x" {
		return fmt.Errorf("expected to receive %q first", "x")
	}
	if acked, err := ack(c, "q", m.Receipt); err != nil || !acked {
		return fmt.Errorf("acking %q before the crash: acked=%v err=%v", "x", acked, err)
	}
	m, err = receive(c, "q", 0)
	if err != nil {
		return err
	}
	if m == nil || m.Body != "y" {
		return fmt.Errorf("expected to receive %q second", "y")
	}
	ctx.Logf("acked x; left y in-flight (un-acked); z still visible — now SIGKILL")

	c, err = restart(ctx, c)
	if err != nil {
		return err
	}

	// x is gone for good; y (in-flight, un-acked) and z come back, in order.
	var got []string
	for {
		m, err := receiveWithin(c, "q", 0, 5*time.Second)
		if err != nil {
			return err
		}
		if m == nil {
			break
		}
		got = append(got, m.Body)
		if acked, err := ack(c, "q", m.Receipt); err != nil || !acked {
			return fmt.Errorf("acking %q after restart: acked=%v err=%v", m.Body, acked, err)
		}
		if len(got) > 3 {
			return fmt.Errorf("drained more than the 2 surviving messages: %v", got)
		}
	}
	if len(got) != 2 || got[0] != "y" || got[1] != "z" {
		return fmt.Errorf("after the crash expected the un-acked messages [y z] (in send order), got %v — an acked message must stay gone and an un-acked one must survive", got)
	}
	ctx.Logf("acked message stayed gone; un-acked y and z survived in order")

	// A second crash: everything is now acked, so the queue must be empty.
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()
	m, err = receiveWithin(c, "q", 0, 2*time.Second)
	if err != nil {
		return err
	}
	if m != nil {
		return fmt.Errorf("after acking everything and crashing again, the queue must be empty, got %q", m.Body)
	}
	ctx.Logf("all-acked queue is empty after a second crash")
	return nil
}

func testRedelivery(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	id, err := send(c, "q", "once")
	if err != nil {
		return err
	}

	const visMs = 500
	start := time.Now()
	m, err := receive(c, "q", visMs)
	if err != nil {
		return err
	}
	if m == nil || m.Body != "once" {
		return fmt.Errorf("expected to receive %q with a %dms visibility timeout", "once", visMs)
	}
	if m.Receives != 1 {
		return fmt.Errorf("first delivery must report receives=1, got %d", m.Receives)
	}
	firstReceipt := m.Receipt
	ctx.Logf("received with a %dms visibility timeout; it must stay hidden until then", visMs)

	// It must NOT come back before the timeout elapses (lower bound w/ slack).
	earlyDeadline := start.Add(visMs / 2 * time.Millisecond)
	for time.Now().Before(earlyDeadline) {
		m, err := receive(c, "q", visMs)
		if err != nil {
			return err
		}
		if m != nil {
			return fmt.Errorf("message was redelivered after %v, but its visibility timeout is %dms — an in-flight message must stay hidden for the full timeout",
				time.Since(start).Round(time.Millisecond), visMs)
		}
		time.Sleep(40 * time.Millisecond)
	}

	// It MUST come back after the timeout (generous upper bound).
	upper := time.Duration(visMs)*time.Millisecond + 3*time.Second
	m, err = receiveWithin(c, "q", 5000, upper-time.Since(start))
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("message was not redelivered within %v of its %dms timeout expiring", upper, visMs)
	}
	ctx.Logf("redelivered after %v", time.Since(start).Round(time.Millisecond))
	if m.Body != "once" || m.ID != id {
		return fmt.Errorf("the redelivered message must be the same one (id %q, body %q), got id %q body %q", id, "once", m.ID, m.Body)
	}
	if m.Receives != 2 {
		return fmt.Errorf("a redelivery must report receives=2, got %d", m.Receives)
	}
	if m.Receipt == firstReceipt {
		return fmt.Errorf("a redelivery must carry a NEW receipt; got the same receipt as the first delivery")
	}

	// The old receipt is now dead.
	acked, err := ack(c, "q", firstReceipt)
	if err != nil {
		return err
	}
	if acked {
		return fmt.Errorf("acking with the first (expired) receipt must return acked=false once the message has been redelivered")
	}
	// The new receipt works.
	acked, err = ack(c, "q", m.Receipt)
	if err != nil {
		return err
	}
	if !acked {
		return fmt.Errorf("acking with the current receipt must return acked=true")
	}
	m, err = receiveWithin(c, "q", 0, time.Second)
	if err != nil {
		return err
	}
	if m != nil {
		return fmt.Errorf("after a successful ack the message must be gone, but it came back: %q", m.Body)
	}
	ctx.Logf("old receipt rejected, new receipt acked the message away")
	return nil
}

func testNack(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := send(c, "q", "n"); err != nil {
		return err
	}
	m, err := receive(c, "q", 0) // long default timeout: only a nack should bring it back
	if err != nil {
		return err
	}
	if m == nil || m.Body != "n" || m.Receives != 1 {
		return fmt.Errorf("expected to receive %q with receives=1", "n")
	}
	firstReceipt := m.Receipt

	ctx.Logf("nacking the message — it must become visible again immediately")
	nacked, err := nack(c, "q", firstReceipt)
	if err != nil {
		return err
	}
	if !nacked {
		return fmt.Errorf("nack of an in-flight message must return nacked=true")
	}

	// Immediately visible — no waiting on the (default 30s) timeout.
	m2, err := receiveWithin(c, "q", 0, time.Second)
	if err != nil {
		return err
	}
	if m2 == nil {
		return fmt.Errorf("after a nack the message must be redeliverable immediately, but it stayed hidden")
	}
	if m2.Body != "n" {
		return fmt.Errorf("nack redelivered the wrong message: got %q", m2.Body)
	}
	if m2.Receives != 2 {
		return fmt.Errorf("a nacked-then-redelivered message must report receives=2, got %d", m2.Receives)
	}
	if m2.Receipt == firstReceipt {
		return fmt.Errorf("the redelivery after a nack must carry a new receipt")
	}
	ctx.Logf("redelivered immediately with receives=2 and a fresh receipt")

	// The nacked receipt is now stale for both ack and nack.
	nacked, err = nack(c, "q", firstReceipt)
	if err != nil {
		return err
	}
	if nacked {
		return fmt.Errorf("nacking with the stale receipt must return nacked=false")
	}
	acked, err := ack(c, "q", firstReceipt)
	if err != nil {
		return err
	}
	if acked {
		return fmt.Errorf("acking with the stale receipt must return acked=false")
	}
	if acked, err := ack(c, "q", m2.Receipt); err != nil || !acked {
		return fmt.Errorf("acking with the current receipt must succeed: acked=%v err=%v", acked, err)
	}
	ctx.Logf("stale receipt rejected by both ack and nack")
	return nil
}

func testFencing(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	if _, err := send(c, "q", "job"); err != nil {
		return err
	}

	// Consumer A takes the job with a short timeout, then stalls.
	const visMs = 300
	start := time.Now()
	a, err := receive(c, "q", visMs)
	if err != nil {
		return err
	}
	if a == nil || a.Body != "job" {
		return fmt.Errorf("consumer A: expected to receive %q", "job")
	}
	ctx.Logf("consumer A holds the job (timeout %dms) then stalls", visMs)

	// After A's timeout expires, consumer B picks it up with a long timeout.
	var b *recvMsg
	for time.Since(start) < 4*time.Second {
		if time.Since(start) < visMs*time.Millisecond {
			time.Sleep(40 * time.Millisecond)
			continue
		}
		b, err = receive(c, "q", 0)
		if err != nil {
			return err
		}
		if b != nil {
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	if b == nil {
		return fmt.Errorf("consumer B never got the redelivered job after A's timeout expired")
	}
	if b.Receipt == a.Receipt {
		return fmt.Errorf("consumer B must get a new receipt, distinct from A's")
	}
	ctx.Logf("consumer B now owns the job under a fresh receipt (receives=%d)", b.Receives)

	// A finally wakes up and acks with its STALE receipt. This must be fenced
	// off: it must not succeed and must not remove B's in-flight message.
	acked, err := ack(c, "q", a.Receipt)
	if err != nil {
		return err
	}
	if acked {
		return fmt.Errorf("consumer A's stale ack must return acked=false — A no longer owns the message")
	}
	ctx.Logf("consumer A's late ack with the stale receipt was rejected")

	// B's receipt must still be valid: if A's ack had wrongly deleted the
	// message, this would now return acked=false.
	acked, err = ack(c, "q", b.Receipt)
	if err != nil {
		return err
	}
	if !acked {
		return fmt.Errorf("consumer B's ack must succeed — A's stale ack must NOT have removed B's in-flight message (this is how queue-backed systems silently lose work)")
	}
	m, err := receiveWithin(c, "q", 0, time.Second)
	if err != nil {
		return err
	}
	if m != nil {
		return fmt.Errorf("after B's ack the queue must be empty, got %q", m.Body)
	}
	ctx.Logf("only B's ack removed the job — exactly-one consumer won")
	return nil
}

func testDeadLetter(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	ctx.Logf("configuring q with max_receives=2, dead_letter_queue=dlq")
	if err := configure(c, "q", 2, "dlq"); err != nil {
		return fmt.Errorf("configure: %w", err)
	}
	// poison is sent first, so it sorts ahead of good and blocks the queue
	// until it is dead-lettered.
	if _, err := send(c, "q", "poison"); err != nil {
		return err
	}
	if _, err := send(c, "q", "good"); err != nil {
		return err
	}

	// Fail "poison" max_receives times. Using nack keeps this deterministic.
	for attempt := 1; attempt <= 2; attempt++ {
		m, err := receive(c, "q", 0)
		if err != nil {
			return err
		}
		if m == nil || m.Body != "poison" {
			got := "null"
			if m != nil {
				got = m.Body
			}
			return fmt.Errorf("delivery %d: expected the head-of-line message %q (it sorts ahead of %q), got %q", attempt, "poison", "good", got)
		}
		if m.Receives != attempt {
			return fmt.Errorf("delivery %d of poison must report receives=%d, got %d", attempt, attempt, m.Receives)
		}
		nacked, err := nack(c, "q", m.Receipt)
		if err != nil {
			return err
		}
		if !nacked {
			return fmt.Errorf("nack of poison on delivery %d must succeed", attempt)
		}
	}
	ctx.Logf("poison delivered twice and nacked each time — the next failure must dead-letter it")

	// poison has now been delivered max_receives (2) times. It was nacked the
	// 2nd time, which is the failure that should move it to the DLQ. So the
	// source queue's head is now "good".
	m, err := receiveWithin(c, "q", 0, 2*time.Second)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("after poison was dead-lettered, %q should be receivable from q", "good")
	}
	if m.Body != "good" {
		return fmt.Errorf("expected %q from q (poison should be gone to the DLQ), got %q — a message must be dead-lettered after max_receives failures so it stops blocking the queue", "good", m.Body)
	}
	if acked, err := ack(c, "q", m.Receipt); err != nil || !acked {
		return fmt.Errorf("acking %q: acked=%v err=%v", "good", acked, err)
	}
	ctx.Logf("source queue unblocked: %q delivered and acked", "good")

	// poison must now live in the DLQ as an ordinary, fresh message.
	d, err := receiveWithin(c, "dlq", 0, 2*time.Second)
	if err != nil {
		return err
	}
	if d == nil {
		return fmt.Errorf("the dead-lettered message must appear in the dlq, but the dlq was empty")
	}
	if d.Body != "poison" {
		return fmt.Errorf("dlq should contain %q, got %q", "poison", d.Body)
	}
	if d.Receives != 1 {
		return fmt.Errorf("a message arriving in the dlq is a fresh delivery there: expected receives=1, got %d", d.Receives)
	}
	if acked, err := ack(c, "dlq", d.Receipt); err != nil || !acked {
		return fmt.Errorf("acking the dead-lettered message: acked=%v err=%v", acked, err)
	}
	ctx.Logf("poison landed in the dlq with a fresh receives count and was acked there")

	// A queue with no policy must redeliver forever (no accidental dropping).
	if _, err := send(c, "plain", "forever"); err != nil {
		return err
	}
	for i := 0; i < 4; i++ {
		m, err := receive(c, "plain", 0)
		if err != nil {
			return err
		}
		if m == nil || m.Body != "forever" {
			return fmt.Errorf("unconfigured queue: delivery %d should keep redelivering %q (no dead-letter policy), got %v", i+1, "forever", m)
		}
		if _, err := nack(c, "plain", m.Receipt); err != nil {
			return err
		}
	}
	ctx.Logf("an unconfigured queue redelivers without ever dropping the message")
	return nil
}

func testQueues(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// Three independent queues with different depths, sent interleaved.
	counts := map[string]int{"orders": 5, "emails": 3, "audit": 2}
	order := []string{"orders", "emails", "audit"}
	for i := 0; i < 5; i++ {
		for _, q := range order {
			if i < counts[q] {
				if _, err := send(c, q, fmt.Sprintf("%s-%d", q, i)); err != nil {
					return err
				}
			}
		}
	}
	for _, q := range order {
		v, f, err := stats(c, q)
		if err != nil {
			return err
		}
		if v != counts[q] || f != 0 {
			return fmt.Errorf("stats(%q): expected visible=%d inflight=0, got visible=%d inflight=%d", q, counts[q], v, f)
		}
	}
	// A never-seen queue reports zeroes.
	if v, f, err := stats(c, "ghost"); err != nil || v != 0 || f != 0 {
		return fmt.Errorf("stats of a never-used queue must be zeroes, got visible=%d inflight=%d err=%v", v, f, err)
	}
	ctx.Logf("three queues report independent depths; an unknown queue reports zero")

	// Receiving from one queue must not disturb the others; stats reflect the
	// in-flight move.
	m, err := receive(c, "orders", 0)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("expected a message from %q", "orders")
	}
	if v, f, err := stats(c, "orders"); err != nil || v != counts["orders"]-1 || f != 1 {
		return fmt.Errorf("after one receive, stats(orders) should be visible=%d inflight=1, got visible=%d inflight=%d err=%v", counts["orders"]-1, v, f, err)
	}
	if v, _, err := stats(c, "emails"); err != nil || v != counts["emails"] {
		return fmt.Errorf("receiving from orders must not affect emails; stats(emails).visible=%d, expected %d", v, counts["emails"])
	}

	// Isolation: a receipt from one queue can't ack a message in another.
	if acked, err := ack(c, "emails", m.Receipt); err != nil || acked {
		return fmt.Errorf("a receipt from %q must not ack anything in %q (got acked=%v)", "orders", "emails", acked)
	}
	// ...but it still acks its own queue.
	if acked, err := ack(c, "orders", m.Receipt); err != nil || !acked {
		return fmt.Errorf("the receipt must still ack its own queue: acked=%v err=%v", acked, err)
	}
	ctx.Logf("queues are isolated: cross-queue acks are rejected, depths stay independent")

	// stats applies visibility timeouts: an expired in-flight message counts
	// as visible again.
	m, err = receive(c, "audit", 300)
	if err != nil {
		return err
	}
	if m == nil {
		return fmt.Errorf("expected a message from %q", "audit")
	}
	if _, f, err := stats(c, "audit"); err != nil || f != 1 {
		return fmt.Errorf("immediately after receive, stats(audit).inflight should be 1, got %d (err=%v)", f, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, f, err := stats(c, "audit")
		if err != nil {
			return err
		}
		if f == 0 {
			break
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("stats must apply the visibility timeout: %dms after receiving with a 300ms timeout, the message still counts as inflight", 3000)
		}
		time.Sleep(50 * time.Millisecond)
	}
	ctx.Logf("stats honors visibility timeouts: the expired in-flight message is visible again")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	rng := rand.New(rand.NewSource(1))
	queues := []string{"g0", "g1", "g2"}

	// unacked[body] = queue it lives in; removed once we successfully ack it.
	unacked := map[string]string{}
	counter := 0
	produce := func(n int) error {
		for i := 0; i < n; i++ {
			q := queues[rng.Intn(len(queues))]
			body := fmt.Sprintf("m%d", counter)
			counter++
			if _, err := send(c, q, body); err != nil {
				return err
			}
			unacked[body] = q
		}
		return nil
	}

	if err := produce(30); err != nil {
		return err
	}

	for round := 1; round <= 4; round++ {
		ctx.Logf("round %d: receive/ack/nack across %d queues, then SIGKILL", round, len(queues))
		for _, q := range queues {
			for j := 0; j < 6; j++ {
				m, err := receive(c, q, 200)
				if err != nil {
					return err
				}
				if m == nil {
					break
				}
				if _, ok := unacked[m.Body]; !ok {
					return fmt.Errorf("round %d: received %q from %q, but that message was already acked — an acked message must never be redelivered", round, m.Body, q)
				}
				if got := unacked[m.Body]; got != q {
					return fmt.Errorf("round %d: message %q surfaced in queue %q but was sent to %q", round, m.Body, q, got)
				}
				switch r := rng.Float64(); {
				case r < 0.5: // ack — only count it if the broker agrees it was in-flight
					acked, err := ack(c, q, m.Receipt)
					if err != nil {
						return err
					}
					if acked {
						delete(unacked, m.Body)
					}
				case r < 0.75: // nack — back to visible
					if _, err := nack(c, q, m.Receipt); err != nil {
						return err
					}
				default: // leave it in-flight; the 200ms timeout will redeliver it
				}
			}
		}
		if err := produce(5); err != nil {
			return err
		}
		c, err = restart(ctx, c)
		if err != nil {
			return err
		}
	}

	// Drain everything. After the final restart all un-acked messages are
	// visible. Every drained body must be one we still owe; nothing acked may
	// reappear; nothing may be lost.
	ctx.Logf("draining all queues; %d message(s) still owed", len(unacked))
	drainDeadline := time.Now().Add(20 * time.Second)
	for len(unacked) > 0 {
		if !time.Now().Before(drainDeadline) {
			return fmt.Errorf("%d message(s) were never delivered after the crashes (lost): %s", len(unacked), someKeys(unacked))
		}
		progressed := false
		for _, q := range queues {
			for {
				m, err := receive(c, q, 2000)
				if err != nil {
					return err
				}
				if m == nil {
					break
				}
				if _, ok := unacked[m.Body]; !ok {
					return fmt.Errorf("drain: received %q which was already acked — an acked message must never come back", m.Body)
				}
				acked, err := ack(c, q, m.Receipt)
				if err != nil {
					return err
				}
				if acked {
					delete(unacked, m.Body)
					progressed = true
				}
			}
		}
		if !progressed {
			time.Sleep(100 * time.Millisecond)
		}
	}
	c.Close()
	ctx.Logf("survived 4 crashes: every message delivered at least once, every ack honored, nothing lost or duplicated past its ack")
	return nil
}

func someKeys(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 8 {
		keys = keys[:8]
	}
	return fmt.Sprintf("%v", keys)
}
