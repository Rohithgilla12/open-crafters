// Package chatservice implements the meta-compose "Build your own chat service"
// challenge. The harness spawns reference id-generator, log, and queue; the
// user implements the messaging gateway.
package chatservice

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

const (
	deliveryQueue = "delivery"
	pollTimeout   = 3 * time.Second
)

var composeServices = []harness.ServiceSpec{
	{Name: "idgen", ReferenceChallenge: "build-your-own-id-generator", EnvAddrKey: "IDGEN_ADDR"},
	{Name: "log", ReferenceChallenge: "build-your-own-log", EnvAddrKey: "LOG_ADDR"},
	{Name: "queue", ReferenceChallenge: "build-your-own-queue", EnvAddrKey: "QUEUE_ADDR"},
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-chat-service/stages/"
	return harness.Challenge{
		Slug:     "build-your-own-chat-service",
		Name:     "Build your own chat service",
		Services: composeServices,
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the stack", Difficulty: "easy", Instructions: docs + "01-bind.md", TestCompose: testBind},
			{Slug: "send", Name: "Send a message", Difficulty: "easy", Instructions: docs + "02-send.md", TestCompose: testSend},
			{Slug: "read", Name: "Read the log", Difficulty: "easy", Instructions: docs + "03-read.md", TestCompose: testRead},
			{Slug: "delivery", Name: "Fan-out queue", Difficulty: "medium", Instructions: docs + "04-delivery.md", TestCompose: testDelivery},
			{Slug: "ack", Name: "Ack delivery", Difficulty: "easy", Instructions: docs + "05-ack.md", TestCompose: testAck},
			{Slug: "ordering", Name: "Message order", Difficulty: "medium", Instructions: docs + "06-ordering.md", TestCompose: testOrdering},
			{Slug: "two-chats", Name: "Two conversations", Difficulty: "medium", Instructions: docs + "07-two-chats.md", TestCompose: testTwoChats},
			{Slug: "concurrent", Name: "Parallel sends", Difficulty: "hard", Instructions: docs + "08-concurrent.md", TestCompose: testConcurrent},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", TestCompose: testGauntlet},
		},
	}
}

type messageEnvelope struct {
	MessageID      string `json:"message_id"`
	ConversationID string `json:"conversation_id"`
	Sender         string `json:"sender"`
	Body           string `json:"body"`
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

func gwSend(c *harness.Client, conv, sender, body string) (string, int, error) {
	var res struct {
		MessageID string `json:"message_id"`
		Offset    int    `json:"offset"`
	}
	if err := c.Call("send_message", map[string]any{
		"conversation_id": conv,
		"sender":          sender,
		"body":            body,
	}, &res); err != nil {
		return "", 0, err
	}
	if res.MessageID == "" {
		return "", 0, fmt.Errorf("send_message: empty message_id")
	}
	return res.MessageID, res.Offset, nil
}

func gwRead(c *harness.Client, conv string, offset, max int) ([]map[string]any, error) {
	var res struct {
		Records []struct {
			Offset int    `json:"offset"`
			Value  string `json:"value"`
		} `json:"records"`
	}
	params := map[string]any{"conversation_id": conv, "offset": offset}
	if max > 0 {
		params["max"] = max
	}
	if err := c.Call("read_messages", params, &res); err != nil {
		return nil, err
	}
	var out []map[string]any
	for _, r := range res.Records {
		var env messageEnvelope
		if err := json.Unmarshal([]byte(r.Value), &env); err != nil {
			return nil, fmt.Errorf("record at offset %d: invalid envelope: %w", r.Offset, err)
		}
		out = append(out, map[string]any{
			"offset":          r.Offset,
			"message_id":      env.MessageID,
			"conversation_id": env.ConversationID,
			"sender":          env.Sender,
			"body":            env.Body,
		})
	}
	return out, nil
}

func gwPoll(c *harness.Client) (map[string]any, string, error) {
	var res struct {
		Message *struct {
			MessageID      string `json:"message_id"`
			ConversationID string `json:"conversation_id"`
			Sender         string `json:"sender"`
			Body           string `json:"body"`
			Receipt        string `json:"receipt"`
		} `json:"message"`
	}
	if err := c.Call("poll_delivery", nil, &res); err != nil {
		return nil, "", err
	}
	if res.Message == nil {
		return nil, "", nil
	}
	msg := map[string]any{
		"message_id":      res.Message.MessageID,
		"conversation_id": res.Message.ConversationID,
		"sender":          res.Message.Sender,
		"body":            res.Message.Body,
	}
	return msg, res.Message.Receipt, nil
}

func waitDelivery(c *harness.Client, timeout time.Duration) (map[string]any, string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		msg, receipt, err := gwPoll(c)
		if err != nil {
			return nil, "", err
		}
		if msg != nil {
			return msg, receipt, nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil, "", fmt.Errorf("no delivery within %s", timeout)
}

func gwAck(c *harness.Client, receipt string) error {
	return c.Call("ack_delivery", map[string]any{"receipt": receipt}, nil)
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
	for _, name := range []string{"idgen", "log", "queue"} {
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
	ctx.Logf("gateway and three reference services answered ping")
	return nil
}

func testSend(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	msgID, offset, err := gwSend(gw, "conv-1", "alice", "hello")
	if err != nil {
		return err
	}
	if offset != 0 {
		return fmt.Errorf("first message offset want 0, got %d", offset)
	}
	ctx.Logf("sent message %s at offset %d", msgID, offset)
	return nil
}

func testRead(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	if _, _, err := gwSend(gw, "conv-read", "bob", "hi there"); err != nil {
		return err
	}
	recs, err := gwRead(gw, "conv-read", 0, 10)
	if err != nil {
		return err
	}
	if len(recs) != 1 {
		return fmt.Errorf("expected 1 record, got %d", len(recs))
	}
	if recs[0]["body"] != "hi there" || recs[0]["sender"] != "bob" {
		return fmt.Errorf("unexpected record: %v", recs[0])
	}
	ctx.Logf("read message back from conversation log")
	return nil
}

func testDelivery(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	if _, _, err := gwSend(gw, "conv-del", "carol", "ping"); err != nil {
		return err
	}
	msg, _, err := waitDelivery(gw, pollTimeout)
	if err != nil {
		return err
	}
	if msg["body"] != "ping" || msg["conversation_id"] != "conv-del" {
		return fmt.Errorf("unexpected delivery: %v", msg)
	}
	ctx.Logf("delivery queue received fan-out notification")
	return nil
}

func testAck(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	if _, _, err := gwSend(gw, "conv-ack", "dave", "ack-me"); err != nil {
		return err
	}
	_, receipt, err := waitDelivery(gw, pollTimeout)
	if err != nil {
		return err
	}
	if err := gwAck(gw, receipt); err != nil {
		return err
	}
	msg, _, err := gwPoll(gw)
	if err != nil {
		return err
	}
	if msg != nil {
		return fmt.Errorf("expected no message after ack, got %v", msg)
	}
	ctx.Logf("delivery ack removed message from queue")
	return nil
}

func testOrdering(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	conv := "conv-order"
	bodies := []string{"one", "two", "three"}
	for _, b := range bodies {
		if _, _, err := gwSend(gw, conv, "eve", b); err != nil {
			return err
		}
	}
	recs, err := gwRead(gw, conv, 0, 10)
	if err != nil {
		return err
	}
	if len(recs) != 3 {
		return fmt.Errorf("expected 3 records, got %d", len(recs))
	}
	for i, want := range bodies {
		if recs[i]["body"] != want {
			return fmt.Errorf("record %d: want body %q, got %q", i, want, recs[i]["body"])
		}
		if recs[i]["offset"] != i {
			return fmt.Errorf("record %d: want offset %d, got %v", i, i, recs[i]["offset"])
		}
	}
	ctx.Logf("log preserves append order")
	return nil
}

func testTwoChats(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	if _, _, err := gwSend(gw, "room-a", "u1", "a"); err != nil {
		return err
	}
	if _, _, err := gwSend(gw, "room-b", "u2", "b"); err != nil {
		return err
	}
	ra, err := gwRead(gw, "room-a", 0, 10)
	if err != nil || len(ra) != 1 || ra[0]["body"] != "a" {
		return fmt.Errorf("room-a: %v err=%v", ra, err)
	}
	rb, err := gwRead(gw, "room-b", 0, 10)
	if err != nil || len(rb) != 1 || rb[0]["body"] != "b" {
		return fmt.Errorf("room-b: %v err=%v", rb, err)
	}
	ctx.Logf("conversations are isolated by topic")
	return nil
}

func testConcurrent(ctx *harness.ComposeContext) error {
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			gw, err := ctx.DialGateway()
			if err != nil {
				errs <- err
				return
			}
			defer gw.Close()
			conv := fmt.Sprintf("conv-par-%d", n)
			body := fmt.Sprintf("msg-%d", n)
			if _, _, err := gwSend(gw, conv, "user", body); err != nil {
				errs <- err
				return
			}
			recs, err := gwRead(gw, conv, 0, 1)
			if err != nil || len(recs) != 1 || recs[0]["body"] != body {
				errs <- fmt.Errorf("conv %s: %v", conv, recs)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}
	ctx.Logf("parallel sends to separate conversations succeeded")
	return nil
}

func testGauntlet(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()

	if _, _, err := gwSend(gw, "gauntlet", "alice", "first"); err != nil {
		return err
	}
	msg, receipt, err := waitDelivery(gw, pollTimeout)
	if err != nil {
		return err
	}
	if msg["body"] != "first" {
		return fmt.Errorf("delivery: %v", msg)
	}
	if err := gwAck(gw, receipt); err != nil {
		return err
	}
	if _, _, err := gwSend(gw, "gauntlet", "bob", "second"); err != nil {
		return err
	}
	recs, err := gwRead(gw, "gauntlet", 0, 10)
	if err != nil || len(recs) != 2 {
		return fmt.Errorf("read: %v len=%d", err, len(recs))
	}
	ctx.Logf("gauntlet: send, deliver, ack, multi-read")
	return nil
}
