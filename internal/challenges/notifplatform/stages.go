// Package notifplatform implements the meta-compose "Build your own notification platform"
// challenge. The harness spawns reference queue, scheduler, and rate-limiter; the
// user implements the notification gateway.
package notifplatform

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

var composeServices = []harness.ServiceSpec{
	{Name: "queue", ReferenceChallenge: "build-your-own-queue", EnvAddrKey: "QUEUE_ADDR"},
	{Name: "scheduler", ReferenceChallenge: "build-your-own-scheduler", EnvAddrKey: "SCHEDULER_ADDR"},
	{Name: "ratelimiter", ReferenceChallenge: "build-your-own-rate-limiter", EnvAddrKey: "RATE_LIMITER_ADDR"},
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-notification-platform/stages/"
	return harness.Challenge{
		Slug:     "build-your-own-notification-platform",
		Name:     "Build your own notification platform",
		Services: composeServices,
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the stack", Difficulty: "easy", Instructions: docs + "01-bind.md", TestCompose: testBind},
			{Slug: "configure", Name: "Configure limits", Difficulty: "easy", Instructions: docs + "02-configure.md", TestCompose: testConfigure},
			{Slug: "notify", Name: "Immediate notify", Difficulty: "easy", Instructions: docs + "03-notify.md", TestCompose: testNotify},
			{Slug: "receive", Name: "Deliver and ack", Difficulty: "medium", Instructions: docs + "04-receive.md", TestCompose: testReceive},
			{Slug: "rate-limit", Name: "Enforce limits", Difficulty: "medium", Instructions: docs + "05-rate-limit.md", TestCompose: testRateLimit},
			{Slug: "schedule", Name: "Delayed digest", Difficulty: "medium", Instructions: docs + "06-schedule.md", TestCompose: testSchedule},
			{Slug: "multi", Name: "Several notifications", Difficulty: "medium", Instructions: docs + "07-multi.md", TestCompose: testMulti},
			{Slug: "concurrent", Name: "Parallel notifies", Difficulty: "hard", Instructions: docs + "08-concurrent.md", TestCompose: testConcurrent},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", TestCompose: testGauntlet},
		},
	}
}

const pollInterval = 50 * time.Millisecond

type notification struct {
	NotificationID string `json:"notification_id"`
	UserID         string `json:"user_id"`
	Channel        string `json:"channel"`
	Body           string `json:"body"`
	Receipt        string `json:"receipt"`
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

func configureLimit(c *harness.Client, userID string, limit, windowMS int) error {
	return c.Call("configure_limit", map[string]any{
		"user_id": userID, "limit": limit, "window_ms": windowMS,
	}, nil)
}

func notify(c *harness.Client, userID, channel, body string) (id string, queued, rateLimited bool, err error) {
	var res struct {
		NotificationID *string `json:"notification_id"`
		Queued         bool    `json:"queued"`
		RateLimited    bool    `json:"rate_limited"`
	}
	if err := c.Call("notify", map[string]any{
		"user_id": userID, "channel": channel, "body": body,
	}, &res); err != nil {
		return "", false, false, err
	}
	if res.NotificationID != nil {
		id = *res.NotificationID
	}
	return id, res.Queued, res.RateLimited, nil
}

func scheduleNotify(c *harness.Client, userID, channel, body string, delayMS int) (notifID, jobID string, err error) {
	var res struct {
		NotificationID string `json:"notification_id"`
		JobID          string `json:"job_id"`
	}
	if err := c.Call("schedule_notify", map[string]any{
		"user_id": userID, "channel": channel, "body": body, "delay_ms": delayMS,
	}, &res); err != nil {
		return "", "", err
	}
	if res.NotificationID == "" || res.JobID == "" {
		return "", "", fmt.Errorf("schedule_notify: empty notification_id or job_id")
	}
	return res.NotificationID, res.JobID, nil
}

func receive(c *harness.Client) (*notification, error) {
	var res struct {
		Notification *notification `json:"notification"`
	}
	if err := c.Call("receive", nil, &res); err != nil {
		return nil, err
	}
	return res.Notification, nil
}

func ack(c *harness.Client, receipt string) error {
	return c.Call("ack", map[string]any{"receipt": receipt}, nil)
}

func waitForNotif(c *harness.Client, timeout time.Duration) (*notification, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		n, err := receive(c)
		if err != nil {
			return nil, err
		}
		if n != nil {
			return n, nil
		}
		time.Sleep(pollInterval)
	}
	return nil, fmt.Errorf("no notification received within %s", timeout)
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
	for _, name := range []string{"queue", "scheduler", "ratelimiter"} {
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
	ctx.Logf("gateway and three reference services answered ping")
	return nil
}

func testConfigure(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := configureLimit(gw, "alice", 5, 60_000); err != nil {
		return err
	}
	ctx.Logf("configured limit for alice")
	return nil
}

func testNotify(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := configureLimit(gw, "bob", 10, 60_000); err != nil {
		return err
	}
	id, queued, limited, err := notify(gw, "bob", "email", "hello")
	if err != nil {
		return err
	}
	if !queued || limited || id == "" {
		return fmt.Errorf("notify: queued=%v rate_limited=%v id=%q", queued, limited, id)
	}
	ctx.Logf("queued notification %q", id)
	return nil
}

func testReceive(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := configureLimit(gw, "carol", 10, 60_000); err != nil {
		return err
	}
	id, _, _, err := notify(gw, "carol", "push", "ping")
	if err != nil {
		return err
	}
	n, err := waitForNotif(gw, 2*time.Second)
	if err != nil {
		return err
	}
	if n.NotificationID != id || n.UserID != "carol" || n.Channel != "push" || n.Body != "ping" || n.Receipt == "" {
		return fmt.Errorf("receive: unexpected payload %+v", n)
	}
	if err := ack(gw, n.Receipt); err != nil {
		return err
	}
	idle, err := receive(gw)
	if err != nil {
		return err
	}
	if idle != nil {
		return fmt.Errorf("expected empty queue after ack, got %q", idle.NotificationID)
	}
	ctx.Logf("delivered and acked %q", id)
	return nil
}

func testRateLimit(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := configureLimit(gw, "dave", 1, 60_000); err != nil {
		return err
	}
	id1, queued1, _, err := notify(gw, "dave", "sms", "one")
	if err != nil {
		return err
	}
	if !queued1 || id1 == "" {
		return fmt.Errorf("first notify should succeed")
	}
	id2, queued2, limited2, err := notify(gw, "dave", "sms", "two")
	if err != nil {
		return err
	}
	if queued2 || !limited2 || id2 != "" {
		return fmt.Errorf("second notify should be rate limited, got queued=%v limited=%v id=%q", queued2, limited2, id2)
	}
	n, err := waitForNotif(gw, 2*time.Second)
	if err != nil {
		return err
	}
	if n.NotificationID != id1 || n.Body != "one" {
		return fmt.Errorf("expected only first notification, got %+v", n)
	}
	if err := ack(gw, n.Receipt); err != nil {
		return err
	}
	extra, err := receive(gw)
	if err != nil {
		return err
	}
	if extra != nil {
		return fmt.Errorf("rate-limited notify leaked onto queue: %q", extra.NotificationID)
	}
	ctx.Logf("rate limit blocked second notify")
	return nil
}

func testSchedule(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	notifID, _, err := scheduleNotify(gw, "erin", "email", "digest", 300)
	if err != nil {
		return err
	}
	early, err := receive(gw)
	if err != nil {
		return err
	}
	if early != nil {
		return fmt.Errorf("scheduled notify delivered immediately: %q", early.NotificationID)
	}
	n, err := waitForNotif(gw, 2*time.Second)
	if err != nil {
		return err
	}
	if n.NotificationID != notifID || n.Body != "digest" {
		return fmt.Errorf("expected scheduled %q, got %+v", notifID, n)
	}
	if err := ack(gw, n.Receipt); err != nil {
		return err
	}
	ctx.Logf("delayed digest %q delivered", notifID)
	return nil
}

func testMulti(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := configureLimit(gw, "frank", 20, 60_000); err != nil {
		return err
	}
	bodies := []string{"a", "b", "c"}
	want := map[string]bool{}
	for _, b := range bodies {
		id, queued, _, err := notify(gw, "frank", "email", b)
		if err != nil {
			return err
		}
		if !queued {
			return fmt.Errorf("notify %q not queued", b)
		}
		want[id] = true
	}
	got := map[string]bool{}
	for range bodies {
		n, err := waitForNotif(gw, 3*time.Second)
		if err != nil {
			return err
		}
		got[n.NotificationID] = true
		if err := ack(gw, n.Receipt); err != nil {
			return err
		}
	}
	for id := range want {
		if !got[id] {
			return fmt.Errorf("missing notification %q", id)
		}
	}
	ctx.Logf("three notifications delivered")
	return nil
}

func testConcurrent(ctx *harness.ComposeContext) error {
	const workers = 10
	ids := make([]string, workers)
	var errs atomic.Value
	var wg sync.WaitGroup
	start := make(chan struct{})

	gw0, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	if err := configureLimit(gw0, "grace", 50, 60_000); err != nil {
		gw0.Close()
		return err
	}
	gw0.Close()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			gw, err := ctx.DialGateway()
			if err != nil {
				errs.Store(err)
				return
			}
			defer gw.Close()
			<-start
			id, queued, _, err := notify(gw, "grace", "push", fmt.Sprintf("n-%d", n))
			if err != nil {
				errs.Store(err)
				return
			}
			if !queued || id == "" {
				errs.Store(fmt.Errorf("worker %d: queued=%v id=%q", n, queued, id))
				return
			}
			ids[n] = id
		}(i)
	}
	close(start)
	wg.Wait()
	if v := errs.Load(); v != nil {
		return v.(error)
	}
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	received := map[string]bool{}
	for range workers {
		n, err := waitForNotif(gw, 5*time.Second)
		if err != nil {
			return err
		}
		received[n.NotificationID] = true
		if err := ack(gw, n.Receipt); err != nil {
			return err
		}
	}
	for _, id := range ids {
		if id == "" || !received[id] {
			return fmt.Errorf("notification %q not received", id)
		}
	}
	ctx.Logf("%d concurrent notifies all delivered", workers)
	return nil
}

func testGauntlet(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := configureLimit(gw, "henry", 1, 60_000); err != nil {
		return err
	}
	id1, queued1, _, err := notify(gw, "henry", "email", "now")
	if err != nil || !queued1 {
		return fmt.Errorf("immediate notify failed: %v queued=%v", err, queued1)
	}
	_, queued2, limited2, err := notify(gw, "henry", "email", "denied")
	if err != nil || queued2 || !limited2 {
		return fmt.Errorf("expected rate limit on second notify")
	}
	sid, _, err := scheduleNotify(gw, "henry", "email", "later", 200)
	if err != nil {
		return err
	}
	n1, err := waitForNotif(gw, 2*time.Second)
	if err != nil {
		return err
	}
	if n1.NotificationID != id1 {
		return fmt.Errorf("expected immediate %q first, got %q", id1, n1.NotificationID)
	}
	if err := ack(gw, n1.Receipt); err != nil {
		return err
	}
	n2, err := waitForNotif(gw, 2*time.Second)
	if err != nil {
		return err
	}
	if n2.NotificationID != sid || n2.Body != "later" {
		return fmt.Errorf("expected scheduled %q, got %+v", sid, n2)
	}
	if err := ack(gw, n2.Receipt); err != nil {
		return err
	}
	ctx.Logf("gauntlet: immediate + rate-limit + delayed")
	return nil
}
