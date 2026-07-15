// Package payledger implements the meta-compose "Build your own payment ledger"
// challenge. The harness spawns reference WAL, id-generator, and MVCC; the user
// implements the ledger gateway.
package payledger

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

var composeServices = []harness.ServiceSpec{
	{Name: "wal", ReferenceChallenge: "build-your-own-wal", EnvAddrKey: "WAL_ADDR"},
	{Name: "idgen", ReferenceChallenge: "build-your-own-id-generator", EnvAddrKey: "IDGEN_ADDR"},
	{Name: "mvcc", ReferenceChallenge: "build-your-own-mvcc", EnvAddrKey: "MVCC_ADDR"},
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-payment-ledger/stages/"
	return harness.Challenge{
		Slug:     "build-your-own-payment-ledger",
		Name:     "Build your own payment ledger",
		Services: composeServices,
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the stack", Difficulty: "easy", Instructions: docs + "01-bind.md", TestCompose: testBind},
			{Slug: "open", Name: "Open accounts", Difficulty: "easy", Instructions: docs + "02-open.md", TestCompose: testOpen},
			{Slug: "transfer", Name: "Transfer funds", Difficulty: "medium", Instructions: docs + "03-transfer.md", TestCompose: testTransfer},
			{Slug: "balance", Name: "Read balances", Difficulty: "easy", Instructions: docs + "04-balance.md", TestCompose: testBalance},
			{Slug: "insufficient", Name: "Insufficient funds", Difficulty: "medium", Instructions: docs + "05-insufficient.md", TestCompose: testInsufficient},
			{Slug: "idempotent", Name: "Idempotent transfer", Difficulty: "medium", Instructions: docs + "06-idempotent.md", TestCompose: testIdempotent},
			{Slug: "multi", Name: "Several transfers", Difficulty: "medium", Instructions: docs + "07-multi.md", TestCompose: testMulti},
			{Slug: "concurrent", Name: "Parallel transfers", Difficulty: "hard", Instructions: docs + "08-concurrent.md", TestCompose: testConcurrent},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", TestCompose: testGauntlet},
		},
	}
}

type transferEnvelope struct {
	TransferID     string `json:"transfer_id"`
	FromAccount    string `json:"from_account"`
	ToAccount      string `json:"to_account"`
	Amount         int    `json:"amount"`
	IdempotencyKey string `json:"idempotency_key"`
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

func openAccount(c *harness.Client, id string, balance int) error {
	return c.Call("open_account", map[string]any{"account_id": id, "balance": balance}, nil)
}

func getBalance(c *harness.Client, id string) (int, bool, error) {
	var res struct {
		Balance int  `json:"balance"`
		Found   bool `json:"found"`
	}
	if err := c.Call("get_balance", map[string]any{"account_id": id}, &res); err != nil {
		return 0, false, err
	}
	return res.Balance, res.Found, nil
}

func transfer(c *harness.Client, from, to string, amount int, key string) (string, bool, error) {
	var res struct {
		TransferID string `json:"transfer_id"`
		Replayed   bool   `json:"replayed"`
	}
	if err := c.Call("transfer", map[string]any{
		"from_account": from, "to_account": to, "amount": amount, "idempotency_key": key,
	}, &res); err != nil {
		return "", false, err
	}
	if res.TransferID == "" {
		return "", false, fmt.Errorf("transfer: empty transfer_id")
	}
	return res.TransferID, res.Replayed, nil
}

func getTransfer(c *harness.Client, id string) (*transferEnvelope, bool, error) {
	var res struct {
		Found    bool               `json:"found"`
		Transfer *transferEnvelope `json:"transfer"`
	}
	if err := c.Call("get_transfer", map[string]any{"transfer_id": id}, &res); err != nil {
		return nil, false, err
	}
	return res.Transfer, res.Found, nil
}

func callErrCode(err error) string {
	if err == nil {
		return ""
	}
	var re *harness.RPCError
	if errors.As(err, &re) {
		return re.Code
	}
	return ""
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
	for _, name := range []string{"wal", "idgen", "mvcc"} {
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

func testOpen(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := openAccount(gw, "alice", 1000); err != nil {
		return err
	}
	bal, found, err := getBalance(gw, "alice")
	if err != nil {
		return err
	}
	if !found || bal != 1000 {
		return fmt.Errorf("alice balance=%d found=%v, want 1000/true", bal, found)
	}
	ctx.Logf("opened alice with 1000")
	return nil
}

func testTransfer(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := openAccount(gw, "a", 500); err != nil {
		return err
	}
	if err := openAccount(gw, "b", 100); err != nil {
		return err
	}
	id, replayed, err := transfer(gw, "a", "b", 50, "k1")
	if err != nil {
		return err
	}
	if replayed {
		return fmt.Errorf("first transfer should not be replayed")
	}
	tr, found, err := getTransfer(gw, id)
	if err != nil {
		return err
	}
	if !found || tr == nil || tr.Amount != 50 || tr.FromAccount != "a" {
		return fmt.Errorf("get_transfer unexpected: found=%v %+v", found, tr)
	}
	ctx.Logf("transfer %q recorded", id)
	return nil
}

func testBalance(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := openAccount(gw, "src", 200); err != nil {
		return err
	}
	if err := openAccount(gw, "dst", 0); err != nil {
		return err
	}
	if _, _, err := transfer(gw, "src", "dst", 75, "bal-1"); err != nil {
		return err
	}
	src, _, err := getBalance(gw, "src")
	if err != nil {
		return err
	}
	dst, _, err := getBalance(gw, "dst")
	if err != nil {
		return err
	}
	if src != 125 || dst != 75 {
		return fmt.Errorf("balances src=%d dst=%d, want 125/75", src, dst)
	}
	ctx.Logf("balances updated correctly")
	return nil
}

func testInsufficient(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := openAccount(gw, "poor", 10); err != nil {
		return err
	}
	if err := openAccount(gw, "rich", 0); err != nil {
		return err
	}
	_, _, err = transfer(gw, "poor", "rich", 50, "over")
	if callErrCode(err) != "INSUFFICIENT_FUNDS" {
		return fmt.Errorf("want INSUFFICIENT_FUNDS, got %v", err)
	}
	bal, _, err := getBalance(gw, "poor")
	if err != nil {
		return err
	}
	if bal != 10 {
		return fmt.Errorf("poor balance changed to %d", bal)
	}
	ctx.Logf("overdraft rejected")
	return nil
}

func testIdempotent(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := openAccount(gw, "x", 100); err != nil {
		return err
	}
	if err := openAccount(gw, "y", 0); err != nil {
		return err
	}
	id1, r1, err := transfer(gw, "x", "y", 40, "same-key")
	if err != nil || r1 {
		return fmt.Errorf("first: err=%v replayed=%v", err, r1)
	}
	id2, r2, err := transfer(gw, "x", "y", 40, "same-key")
	if err != nil || !r2 || id2 != id1 {
		return fmt.Errorf("replay: err=%v replayed=%v id=%q want %q", err, r2, id2, id1)
	}
	x, _, _ := getBalance(gw, "x")
	y, _, _ := getBalance(gw, "y")
	if x != 60 || y != 40 {
		return fmt.Errorf("double-spend: x=%d y=%d", x, y)
	}
	ctx.Logf("idempotent replay returned %q", id1)
	return nil
}

func testMulti(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := openAccount(gw, "m1", 300); err != nil {
		return err
	}
	if err := openAccount(gw, "m2", 0); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if _, _, err := transfer(gw, "m1", "m2", 50, fmt.Sprintf("m-%d", i)); err != nil {
			return err
		}
	}
	b1, _, _ := getBalance(gw, "m1")
	b2, _, _ := getBalance(gw, "m2")
	if b1 != 150 || b2 != 150 {
		return fmt.Errorf("want 150/150, got %d/%d", b1, b2)
	}
	ctx.Logf("three transfers applied")
	return nil
}

func testConcurrent(ctx *harness.ComposeContext) error {
	const n = 8
	gw0, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	if err := openAccount(gw0, "sink", 0); err != nil {
		gw0.Close()
		return err
	}
	for i := 0; i < n; i++ {
		if err := openAccount(gw0, fmt.Sprintf("c%d", i), 100); err != nil {
			gw0.Close()
			return err
		}
	}
	gw0.Close()

	var errs atomic.Value
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			gw, err := ctx.DialGateway()
			if err != nil {
				errs.Store(err)
				return
			}
			defer gw.Close()
			<-start
			if _, _, err := transfer(gw, fmt.Sprintf("c%d", i), "sink", 10, fmt.Sprintf("c-key-%d", i)); err != nil {
				errs.Store(err)
			}
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
	sink, _, err := getBalance(gw, "sink")
	if err != nil {
		return err
	}
	if sink != n*10 {
		return fmt.Errorf("sink=%d, want %d", sink, n*10)
	}
	for i := 0; i < n; i++ {
		b, _, err := getBalance(gw, fmt.Sprintf("c%d", i))
		if err != nil {
			return err
		}
		if b != 90 {
			return fmt.Errorf("c%d=%d, want 90", i, b)
		}
	}
	ctx.Logf("%d concurrent transfers settled", n)
	return nil
}

func testGauntlet(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	if err := openAccount(gw, "g1", 100); err != nil {
		return err
	}
	if err := openAccount(gw, "g2", 20); err != nil {
		return err
	}
	id, _, err := transfer(gw, "g1", "g2", 30, "g-key")
	if err != nil {
		return err
	}
	if _, _, err := transfer(gw, "g1", "g2", 30, "g-key"); err != nil {
		return err
	}
	_, _, err = transfer(gw, "g2", "g1", 1000, "g-over")
	if callErrCode(err) != "INSUFFICIENT_FUNDS" {
		return fmt.Errorf("want INSUFFICIENT_FUNDS, got %v", err)
	}
	b1, _, _ := getBalance(gw, "g1")
	b2, _, _ := getBalance(gw, "g2")
	if b1 != 70 || b2 != 50 {
		return fmt.Errorf("gauntlet balances %d/%d", b1, b2)
	}
	tr, found, err := getTransfer(gw, id)
	if err != nil || !found || tr.Amount != 30 {
		return fmt.Errorf("transfer lookup failed")
	}
	ctx.Logf("gauntlet passed")
	return nil
}
