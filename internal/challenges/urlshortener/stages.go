// Package urlshortener implements the meta-compose "Build your own URL shortener"
// challenge. The harness spawns reference id-generator, bloom-filter, and
// object-store binaries; the user implements the gateway.
package urlshortener

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

var composeServices = []harness.ServiceSpec{
	{Name: "idgen", ReferenceChallenge: "build-your-own-id-generator", EnvAddrKey: "IDGEN_ADDR"},
	{Name: "bloom", ReferenceChallenge: "build-your-own-bloom-filter", EnvAddrKey: "BLOOM_ADDR"},
	{Name: "store", ReferenceChallenge: "build-your-own-object-store", EnvAddrKey: "STORE_ADDR"},
}

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-url-shortener/stages/"
	return harness.Challenge{
		Slug:     "build-your-own-url-shortener",
		Name:     "Build your own URL shortener",
		Services: composeServices,
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the stack", Difficulty: "easy", Instructions: docs + "01-bind.md", TestCompose: testBind},
			{Slug: "shorten", Name: "Mint a short code", Difficulty: "easy", Instructions: docs + "02-shorten.md", TestCompose: testShorten},
			{Slug: "resolve", Name: "Resolve a code", Difficulty: "easy", Instructions: docs + "03-resolve.md", TestCompose: testResolve},
			{Slug: "not-found", Name: "Unknown codes", Difficulty: "easy", Instructions: docs + "04-not-found.md", TestCompose: testNotFound},
			{Slug: "analytics", Name: "Record a click", Difficulty: "medium", Instructions: docs + "05-analytics.md", TestCompose: testAnalytics},
			{Slug: "bloom", Name: "Bloom membership", Difficulty: "medium", Instructions: docs + "06-bloom.md", TestCompose: testBloom},
			{Slug: "multi-url", Name: "Many URLs", Difficulty: "medium", Instructions: docs + "07-multi-url.md", TestCompose: testMultiURL},
			{Slug: "concurrent", Name: "Parallel shortens", Difficulty: "hard", Instructions: docs + "08-concurrent.md", TestCompose: testConcurrent},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", TestCompose: testGauntlet},
		},
	}
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

func shorten(c *harness.Client, url string) (string, error) {
	var res struct {
		Code string `json:"code"`
	}
	if err := c.Call("shorten", map[string]any{"url": url}, &res); err != nil {
		return "", err
	}
	if res.Code == "" {
		return "", fmt.Errorf("shorten: empty code")
	}
	return res.Code, nil
}

func resolve(c *harness.Client, code string) (bool, string, error) {
	var res struct {
		Found bool   `json:"found"`
		URL   string `json:"url"`
	}
	if err := c.Call("resolve", map[string]any{"code": code}, &res); err != nil {
		return false, "", err
	}
	return res.Found, res.URL, nil
}

func recordClick(c *harness.Client, code string) error {
	return c.Call("record_click", map[string]any{"code": code}, nil)
}

func storeGet(ctx *harness.ComposeContext, key string) (bool, string, error) {
	sc, err := ctx.DialService("store")
	if err != nil {
		return false, "", err
	}
	defer sc.Close()
	var res struct {
		Found bool   `json:"found"`
		Body  string `json:"body"`
	}
	if err := sc.Call("get", map[string]any{"key": key}, &res); err != nil {
		var rpc *harness.RPCError
		if errors.As(err, &rpc) && rpc.Code == "NOT_FOUND" {
			return false, "", nil
		}
		return false, "", err
	}
	return res.Found, res.Body, nil
}

func bloomContains(ctx *harness.ComposeContext, item string) (bool, error) {
	bc, err := ctx.DialService("bloom")
	if err != nil {
		return false, err
	}
	defer bc.Close()
	var res struct {
		MaybePresent bool `json:"maybe_present"`
	}
	if err := bc.Call("contains", map[string]any{"filter_id": "codes", "item": item}, &res); err != nil {
		return false, err
	}
	return res.MaybePresent, nil
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
	for _, name := range []string{"idgen", "bloom", "store"} {
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

func testShorten(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	code, err := shorten(gw, "https://example.com/hello")
	if err != nil {
		return err
	}
	ctx.Logf("shortened to code %q", code)
	return nil
}

func testResolve(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	const url = "https://open-crafters.dev/docs"
	code, err := shorten(gw, url)
	if err != nil {
		return err
	}
	found, got, err := resolve(gw, code)
	if err != nil {
		return err
	}
	if !found || got != url {
		return fmt.Errorf("resolve %q: found=%v url=%q want %q", code, found, got, url)
	}
	ctx.Logf("round-trip resolve OK")
	return nil
}

func testNotFound(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	found, _, err := resolve(gw, "never-minted-code-999")
	if err != nil {
		return err
	}
	if found {
		return fmt.Errorf("resolve unknown code: expected found=false")
	}
	ctx.Logf("unknown code returns found=false")
	return nil
}

func testAnalytics(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	code, err := shorten(gw, "https://analytics.test/page")
	if err != nil {
		return err
	}
	if err := recordClick(gw, code); err != nil {
		return err
	}
	found, _, err := storeGet(ctx, "clicks/"+code)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("expected click object at clicks/%s in object store", code)
	}
	ctx.Logf("click recorded in object store")
	return nil
}

func testBloom(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	code, err := shorten(gw, "https://bloom.test/x")
	if err != nil {
		return err
	}
	present, err := bloomContains(ctx, code)
	if err != nil {
		return err
	}
	if !present {
		return fmt.Errorf("code %q should be in bloom filter after shorten", code)
	}
	ctx.Logf("bloom filter contains minted code")
	return nil
}

func testMultiURL(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	urls := []string{
		"https://a.example/1",
		"https://b.example/2",
		"https://c.example/3",
	}
	codes := make([]string, len(urls))
	for i, u := range urls {
		codes[i], err = shorten(gw, u)
		if err != nil {
			return err
		}
	}
	for i, u := range urls {
		found, got, err := resolve(gw, codes[i])
		if err != nil || !found || got != u {
			return fmt.Errorf("resolve %q: found=%v got=%q want %q", codes[i], found, got, u)
		}
	}
	ctx.Logf("%d distinct URLs round-trip", len(urls))
	return nil
}

func testConcurrent(ctx *harness.ComposeContext) error {
	const workers = 12
	codes := make([]string, workers)
	var errs atomic.Value
	var wg sync.WaitGroup
	start := make(chan struct{})
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
			url := fmt.Sprintf("https://concurrent.test/%d", n)
			code, err := shorten(gw, url)
			if err != nil {
				errs.Store(err)
				return
			}
			codes[n] = code
		}(i)
	}
	close(start)
	wg.Wait()
	if v := errs.Load(); v != nil {
		return v.(error)
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if c == "" {
			return fmt.Errorf("empty code from concurrent shorten")
		}
		if seen[c] {
			return fmt.Errorf("duplicate code %q under concurrent shorten", c)
		}
		seen[c] = true
	}
	ctx.Logf("%d concurrent shortens produced unique codes", workers)
	return nil
}

func testGauntlet(ctx *harness.ComposeContext) error {
	gw, err := ctx.DialGateway()
	if err != nil {
		return err
	}
	defer gw.Close()
	for i := 0; i < 20; i++ {
		url := fmt.Sprintf("https://gauntlet.test/%d", i)
		code, err := shorten(gw, url)
		if err != nil {
			return err
		}
		found, got, err := resolve(gw, code)
		if err != nil || !found || got != url {
			return fmt.Errorf("gauntlet round-trip %d failed", i)
		}
		if i%3 == 0 {
			if err := recordClick(gw, code); err != nil {
				return err
			}
		}
	}
	found, _, err := resolve(gw, "missing-gauntlet-code")
	if err != nil || found {
		return fmt.Errorf("gauntlet miss check failed")
	}
	ctx.Logf("20 shorten/resolve cycles + clicks + miss check")
	return nil
}
