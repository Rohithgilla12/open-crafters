// Package grader implements stage tests for the "Build your own harness"
// challenge. See challenges/build-your-own-harness/PROTOCOL.md.
package grader

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-harness/stages/"
	return harness.Challenge{
		Slug: "build-your-own-harness",
		Name: "Build your own harness",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the harness", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "spawn", Name: "Spawn a program", Difficulty: "easy", Instructions: docs + "02-spawn.md", Test: testSpawn},
			{Slug: "call", Name: "Proxy a call", Difficulty: "easy", Instructions: docs + "03-call.md", Test: testCall},
			{Slug: "set-get", Name: "Set and get via proxy", Difficulty: "easy", Instructions: docs + "04-set-get.md", Test: testSetGet},
			{Slug: "run-case", Name: "Run one case", Difficulty: "medium", Instructions: docs + "05-run-case.md", Test: testRunCase},
			{Slug: "run-suite", Name: "Run a suite", Difficulty: "medium", Instructions: docs + "06-run-suite.md", Test: testRunSuite},
			{Slug: "respawn", Name: "Spawn again", Difficulty: "medium", Instructions: docs + "07-respawn.md", Test: testRespawn},
			{Slug: "concurrent", Name: "Parallel calls", Difficulty: "hard", Instructions: docs + "08-concurrent.md", Test: testConcurrent},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", Test: testGauntlet},
		},
	}
}

func toyProgram() (string, error) {
	root, ok := harness.RepoRoot()
	if !ok {
		return "", fmt.Errorf("cannot locate repo root for toy KV fixture")
	}
	return filepath.Join(root, "challenges/build-your-own-harness/fixtures/toy-kv/go/your_program.sh"), nil
}

func pingHarness(c *harness.Client) error {
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

func pingAddr(addr string) error {
	c, err := harness.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()
	var res struct {
		Message string `json:"message"`
	}
	if err := c.Call("ping", nil, &res); err != nil {
		return err
	}
	if res.Message != "pong" {
		return fmt.Errorf(`direct ping %s: expected "pong", got %q`, addr, res.Message)
	}
	return nil
}

func harnessSpawn(c *harness.Client, program string) (string, error) {
	var res struct {
		Addr string `json:"addr"`
	}
	if err := c.Call("spawn", map[string]any{"program": program}, &res); err != nil {
		return "", err
	}
	if res.Addr == "" {
		return "", fmt.Errorf("spawn: empty addr")
	}
	return res.Addr, nil
}

func harnessCall(c *harness.Client, addr, method string, params map[string]any, out any) error {
	p := map[string]any{"addr": addr, "method": method}
	if params != nil {
		p["params"] = params
	} else {
		p["params"] = map[string]any{}
	}
	return c.Call("call", p, out)
}

func harnessRunCase(c *harness.Client, program, method string, params, expect map[string]any) error {
	p := map[string]any{
		"program": program,
		"method":  method,
		"expect":  expect,
	}
	if params != nil {
		p["params"] = params
	} else {
		p["params"] = map[string]any{}
	}
	return c.Call("run_case", p, nil)
}

func testBind(ctx *harness.Context) error {
	c1, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c1.Close()
	c2, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c2.Close()
	for i := 0; i < 3; i++ {
		if err := pingHarness(c1); err != nil {
			return err
		}
		if err := pingHarness(c2); err != nil {
			return err
		}
	}
	ctx.Logf("both connections answered ping")
	return nil
}

func testSpawn(ctx *harness.Context) error {
	toy, err := toyProgram()
	if err != nil {
		return err
	}
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	addr, err := harnessSpawn(c, toy)
	if err != nil {
		return err
	}
	if err := pingAddr(addr); err != nil {
		return err
	}
	ctx.Logf("spawned toy at %s", addr)
	return nil
}

func testCall(ctx *harness.Context) error {
	toy, err := toyProgram()
	if err != nil {
		return err
	}
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	addr, err := harnessSpawn(c, toy)
	if err != nil {
		return err
	}
	var res struct {
		Message string `json:"message"`
	}
	if err := harnessCall(c, addr, "ping", nil, &res); err != nil {
		return err
	}
	if res.Message != "pong" {
		return fmt.Errorf(`call ping: expected "pong", got %q`, res.Message)
	}
	ctx.Logf("call proxied ping through harness")
	return nil
}

func testSetGet(ctx *harness.Context) error {
	toy, err := toyProgram()
	if err != nil {
		return err
	}
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	addr, err := harnessSpawn(c, toy)
	if err != nil {
		return err
	}
	if err := harnessCall(c, addr, "set", map[string]any{"key": "color", "value": "blue"}, nil); err != nil {
		return err
	}
	var got map[string]any
	if err := harnessCall(c, addr, "get", map[string]any{"key": "color"}, &got); err != nil {
		return err
	}
	hit, _ := got["hit"].(bool)
	val, _ := got["value"].(string)
	if !hit || val != "blue" {
		return fmt.Errorf("get after set: got %#v", got)
	}
	ctx.Logf("set/get round-trip via harness proxy")
	return nil
}

func testRunCase(ctx *harness.Context) error {
	toy, err := toyProgram()
	if err != nil {
		return err
	}
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	if err := harnessRunCase(c, toy, "ping", nil, map[string]any{"message": "pong"}); err != nil {
		return err
	}
	ctx.Logf("run_case passed for toy ping")
	return nil
}

func testRunSuite(ctx *harness.Context) error {
	toy, err := toyProgram()
	if err != nil {
		return err
	}
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	cases := []struct {
		method string
		params map[string]any
		expect map[string]any
	}{
		{"ping", nil, map[string]any{"message": "pong"}},
		{"get", map[string]any{"key": "missing"}, map[string]any{"hit": false}},
		{"set", map[string]any{"key": "suite", "value": "ok"}, map[string]any{}},
	}
	for i, tc := range cases {
		if err := harnessRunCase(c, toy, tc.method, tc.params, tc.expect); err != nil {
			return fmt.Errorf("suite case %d (%s): %w", i+1, tc.method, err)
		}
	}
	ctx.Logf("%d run_case assertions passed", len(cases))
	return nil
}

func testRespawn(ctx *harness.Context) error {
	toy, err := toyProgram()
	if err != nil {
		return err
	}
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()
	addr1, err := harnessSpawn(c, toy)
	if err != nil {
		return err
	}
	if err := pingAddr(addr1); err != nil {
		return err
	}
	addr2, err := harnessSpawn(c, toy)
	if err != nil {
		return err
	}
	if addr2 == addr1 {
		return fmt.Errorf("second spawn reused addr %s", addr1)
	}
	if err := pingAddr(addr2); err != nil {
		return err
	}
	ctx.Logf("two independent spawns: %s and %s", addr1, addr2)
	return nil
}

func testConcurrent(ctx *harness.Context) error {
	toy, err := toyProgram()
	if err != nil {
		return err
	}
	setup, err := ctx.Dial()
	if err != nil {
		return err
	}
	addr, err := harnessSpawn(setup, toy)
	setup.Close()
	if err != nil {
		return err
	}
	var errs atomic.Value
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			client, err := ctx.Dial()
			if err != nil {
				errs.Store(err)
				return
			}
			defer client.Close()
			var res struct {
				Message string `json:"message"`
			}
			if err := harnessCall(client, addr, "ping", nil, &res); err != nil {
				errs.Store(err)
				return
			}
			if res.Message != "pong" {
				errs.Store(fmt.Errorf(`concurrent call: got %q`, res.Message))
			}
		}()
	}
	close(start)
	wg.Wait()
	if v := errs.Load(); v != nil {
		return v.(error)
	}
	ctx.Logf("two parallel call RPCs on separate connections succeeded")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	toy, err := toyProgram()
	if err != nil {
		return err
	}
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	runCases := []struct {
		method string
		params map[string]any
		expect map[string]any
	}{
		{"ping", nil, map[string]any{"message": "pong"}},
		{"get", map[string]any{"key": "ghost"}, map[string]any{"hit": false}},
		{"set", map[string]any{"key": "seed", "value": "x"}, map[string]any{}},
		{"ping", nil, map[string]any{"message": "pong"}},
	}
	for i, tc := range runCases {
		if err := harnessRunCase(c, toy, tc.method, tc.params, tc.expect); err != nil {
			return fmt.Errorf("gauntlet run_case %d: %w", i+1, err)
		}
	}

	keys := []string{"g:1", "g:2", "g:3"}
	addr, err := harnessSpawn(c, toy)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if err := harnessCall(c, addr, "set", map[string]any{"key": k, "value": "v-" + k}, nil); err != nil {
			return err
		}
	}
	for _, k := range keys {
		var got map[string]any
		if err := harnessCall(c, addr, "get", map[string]any{"key": k}, &got); err != nil {
			return err
		}
		want := map[string]any{"hit": true, "value": "v-" + k}
		if !jsonSubset(got, want) {
			return fmt.Errorf("gauntlet get %q: got %#v want subset %#v", k, got, want)
		}
	}
	ctx.Logf("gauntlet: %d run_case assertions + multi-key proxy", len(runCases))
	return nil
}

func jsonSubset(got, want map[string]any) bool {
	for k, wv := range want {
		gv, ok := got[k]
		if !ok {
			return false
		}
		gb, _ := json.Marshal(gv)
		wb, _ := json.Marshal(wv)
		if string(gb) != string(wb) {
			return false
		}
	}
	return true
}
