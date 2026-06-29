// Package objectstore implements stage tests for the "Build your own object
// store" challenge. See challenges/build-your-own-object-store/PROTOCOL.md.
package objectstore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/Rohithgilla12/open-crafters/internal/harness"
)

func Challenge() harness.Challenge {
	docs := "challenges/build-your-own-object-store/stages/"
	return harness.Challenge{
		Slug: "build-your-own-object-store",
		Name: "Build your own object store",
		Stages: []harness.Stage{
			{Slug: "bind", Name: "Boot the server", Difficulty: "easy", Instructions: docs + "01-bind.md", Test: testBind},
			{Slug: "put-get", Name: "Put and get", Difficulty: "easy", Instructions: docs + "02-put-get.md", Test: testPutGet},
			{Slug: "head", Name: "Head metadata", Difficulty: "easy", Instructions: docs + "03-head.md", Test: testHead},
			{Slug: "overwrite", Name: "Overwrite", Difficulty: "easy", Instructions: docs + "04-overwrite.md", Test: testOverwrite},
			{Slug: "delete", Name: "Delete", Difficulty: "easy", Instructions: docs + "05-delete.md", Test: testDelete},
			{Slug: "list", Name: "List by prefix", Difficulty: "medium", Instructions: docs + "06-list.md", Test: testList},
			{Slug: "multipart", Name: "Multipart upload", Difficulty: "hard", Instructions: docs + "07-multipart.md", Test: testMultipart},
			{Slug: "durability", Name: "Survive a crash", Difficulty: "medium", Instructions: docs + "08-durability.md", Test: testDurability},
			{Slug: "gauntlet", Name: "The gauntlet", Difficulty: "hard", Instructions: docs + "09-gauntlet.md", Test: testGauntlet},
		},
	}
}

func bodyETag(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// --- RPC helpers ---

func ping(c *harness.Client) error {
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

func put(c *harness.Client, key, body string) (etag string, err error) {
	var res struct {
		ETag string `json:"etag"`
	}
	if err := c.Call("put", map[string]any{"key": key, "body": body}, &res); err != nil {
		return "", fmt.Errorf("put %q: %w", key, err)
	}
	want := bodyETag(body)
	if res.ETag != want {
		return "", fmt.Errorf("put %q: etag %q, expected lowercase hex SHA-256 %q", key, res.ETag, want)
	}
	return res.ETag, nil
}

type getResult struct {
	Found bool   `json:"found"`
	Body  string `json:"body"`
	ETag  string `json:"etag"`
	Size  int    `json:"size"`
}

func get(c *harness.Client, key string) (*getResult, error) {
	var res getResult
	if err := c.Call("get", map[string]any{"key": key}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

type headResult struct {
	Found bool   `json:"found"`
	ETag  string `json:"etag"`
	Size  int    `json:"size"`
}

func head(c *harness.Client, key string) (*headResult, error) {
	var res headResult
	if err := c.Call("head", map[string]any{"key": key}, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

func deleteKey(c *harness.Client, key string) (bool, error) {
	var res struct {
		Deleted bool `json:"deleted"`
	}
	if err := c.Call("delete", map[string]any{"key": key}, &res); err != nil {
		return false, fmt.Errorf("delete %q: %w", key, err)
	}
	return res.Deleted, nil
}

func listKeys(c *harness.Client, prefix string) ([]string, error) {
	var res struct {
		Keys []string `json:"keys"`
	}
	if err := c.Call("list", map[string]any{"prefix": prefix}, &res); err != nil {
		return nil, fmt.Errorf("list prefix %q: %w", prefix, err)
	}
	if res.Keys == nil {
		res.Keys = []string{}
	}
	return res.Keys, nil
}

func createMultipart(c *harness.Client, key string) (uploadID string, err error) {
	var res struct {
		UploadID string `json:"upload_id"`
	}
	if err := c.Call("create_multipart", map[string]any{"key": key}, &res); err != nil {
		return "", fmt.Errorf("create_multipart %q: %w", key, err)
	}
	if res.UploadID == "" {
		return "", fmt.Errorf("create_multipart %q returned empty upload_id", key)
	}
	return res.UploadID, nil
}

func uploadPart(c *harness.Client, uploadID string, partNumber int, body string) (string, error) {
	var res struct {
		ETag string `json:"etag"`
	}
	if err := c.Call("upload_part", map[string]any{
		"upload_id":   uploadID,
		"part_number": partNumber,
		"body":        body,
	}, &res); err != nil {
		return "", fmt.Errorf("upload_part %d: %w", partNumber, err)
	}
	want := bodyETag(body)
	if res.ETag != want {
		return "", fmt.Errorf("upload_part %d: etag %q, expected %q", partNumber, res.ETag, want)
	}
	return res.ETag, nil
}

type partSpec struct {
	PartNumber int    `json:"part_number"`
	ETag       string `json:"etag"`
}

func completeMultipart(c *harness.Client, uploadID string, parts []partSpec) (string, error) {
	var res struct {
		ETag string `json:"etag"`
	}
	if err := c.Call("complete_multipart", map[string]any{
		"upload_id": uploadID,
		"parts":     parts,
	}, &res); err != nil {
		return "", err
	}
	return res.ETag, nil
}

func expectRPCError(err error, code, context string) error {
	if err == nil {
		return fmt.Errorf("%s: expected error %q, call succeeded", context, code)
	}
	var rpcErr *harness.RPCError
	if !errors.As(err, &rpcErr) {
		return fmt.Errorf("%s: expected %q, got %v", context, code, err)
	}
	if rpcErr.Code != code {
		return fmt.Errorf("%s: expected %q, got %q (%s)", context, code, rpcErr.Code, rpcErr.Message)
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

	ctx.Logf("get on a missing key must return NOT_FOUND")
	if _, err := get(c, "missing"); expectRPCError(err, "NOT_FOUND", "get missing key") != nil {
		return expectRPCError(err, "NOT_FOUND", "get missing key")
	}

	const key, body = "photos/cat.jpg", "meow"
	etag, err := put(c, key, body)
	if err != nil {
		return err
	}
	ctx.Logf("put returned etag %s", etag[:16]+"...")

	res, err := get(c, key)
	if err != nil {
		return err
	}
	if !res.Found {
		return fmt.Errorf("get %q after put: expected found=true", key)
	}
	if res.Body != body {
		return fmt.Errorf("get %q: body %q, expected %q", key, res.Body, body)
	}
	if res.ETag != etag {
		return fmt.Errorf("get %q: etag %q, expected %q", key, res.ETag, etag)
	}
	if res.Size != len(body) {
		return fmt.Errorf("get %q: size %d, expected %d", key, res.Size, len(body))
	}
	ctx.Logf("round-trip put/get with correct etag and size")
	return nil
}

func testHead(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	ctx.Logf("head on a missing key must return NOT_FOUND")
	if _, err := head(c, "ghost"); expectRPCError(err, "NOT_FOUND", "head missing key") != nil {
		return expectRPCError(err, "NOT_FOUND", "head missing key")
	}

	const key, body = "docs/readme.txt", "# Hello\n"
	if _, err := put(c, key, body); err != nil {
		return err
	}

	h, err := head(c, key)
	if err != nil {
		return err
	}
	if !h.Found {
		return fmt.Errorf("head %q after put: expected found=true", key)
	}
	if h.ETag != bodyETag(body) {
		return fmt.Errorf("head %q: etag mismatch", key)
	}
	if h.Size != len(body) {
		return fmt.Errorf("head %q: size %d, expected %d", key, h.Size, len(body))
	}

	// head must not return body — verify via raw call that body field is absent or empty.
	var raw map[string]any
	if err := c.Call("head", map[string]any{"key": key}, &raw); err != nil {
		return err
	}
	if bodyVal, ok := raw["body"]; ok && bodyVal != nil && bodyVal != "" {
		return fmt.Errorf("head must not return body bytes, got %v", bodyVal)
	}
	ctx.Logf("head returns metadata only")
	return nil
}

func testOverwrite(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const key = "data/config.json"
	etag1, err := put(c, key, `{"v":1}`)
	if err != nil {
		return err
	}
	etag2, err := put(c, key, `{"v":2}`)
	if err != nil {
		return err
	}
	if etag1 == etag2 {
		return fmt.Errorf("overwrite must produce a new etag; both were %q", etag1)
	}

	res, err := get(c, key)
	if err != nil {
		return err
	}
	if res.Body != `{"v":2}` || res.ETag != etag2 {
		return fmt.Errorf("get after overwrite: expected body v2 and new etag, got body=%q etag=%q", res.Body, res.ETag)
	}
	ctx.Logf("second put replaced the object with a new etag")
	return nil
}

func testDelete(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	deleted, err := deleteKey(c, "never")
	if err != nil {
		return err
	}
	if deleted {
		return fmt.Errorf("delete of a missing key must return deleted=false")
	}

	const key, body = "tmp/scratch", "gone soon"
	if _, err := put(c, key, body); err != nil {
		return err
	}
	deleted, err = deleteKey(c, key)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("delete of an existing key must return deleted=true")
	}
	if _, err := get(c, key); expectRPCError(err, "NOT_FOUND", "get after delete") != nil {
		return expectRPCError(err, "NOT_FOUND", "get after delete")
	}
	deleted, err = deleteKey(c, key)
	if err != nil {
		return err
	}
	if deleted {
		return fmt.Errorf("second delete must return deleted=false")
	}
	ctx.Logf("delete removes the object; missing keys report deleted=false")
	return nil
}

func testList(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	keys := []string{"a/1", "a/2", "a/10", "b/1", "c"}
	for _, k := range keys {
		if _, err := put(c, k, k); err != nil {
			return err
		}
	}

	got, err := listKeys(c, "a/")
	if err != nil {
		return err
	}
	want := []string{"a/1", "a/10", "a/2"}
	if len(got) != len(want) {
		return fmt.Errorf("list prefix %q: got %v, want %v", "a/", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			return fmt.Errorf("list prefix %q: got %v, want lexicographic %v", "a/", got, want)
		}
	}

	all, err := listKeys(c, "")
	if err != nil {
		return err
	}
	if !sort.StringsAreSorted(all) {
		return fmt.Errorf("list with empty prefix must be lexicographically sorted, got %v", all)
	}
	if len(all) != len(keys) {
		return fmt.Errorf("list %q: expected %d keys, got %d", "", len(keys), len(all))
	}

	empty, err := listKeys(c, "z/")
	if err != nil {
		return err
	}
	if len(empty) != 0 {
		return fmt.Errorf("list prefix with no matches must return [], got %v", empty)
	}
	ctx.Logf("prefix listing is lexicographically sorted")
	return nil
}

func testMultipart(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	defer c.Close()

	const key = "archive/big.bin"
	uploadID, err := createMultipart(c, key)
	if err != nil {
		return err
	}

	if _, err := uploadPart(c, "no-such", 1, "x"); expectRPCError(err, "NO_SUCH_UPLOAD", "upload_part bad id") != nil {
		return expectRPCError(err, "NO_SUCH_UPLOAD", "upload_part bad id")
	}

	parts := []struct {
		num  int
		body string
	}{
		{1, "AAA"},
		{2, "BBB"},
		{3, "CCC"},
	}
	etags := make([]partSpec, len(parts))
	for i, p := range parts {
		etag, err := uploadPart(c, uploadID, p.num, p.body)
		if err != nil {
			return err
		}
		etags[i] = partSpec{PartNumber: p.num, ETag: etag}
	}

	// Wrong etag → INVALID_PART
	bad := append([]partSpec(nil), etags...)
	bad[1].ETag = strings.Repeat("0", 64)
	if _, err := completeMultipart(c, uploadID, bad); expectRPCError(err, "INVALID_PART", "wrong etag") != nil {
		return expectRPCError(err, "INVALID_PART", "wrong etag")
	}

	// Wrong order → INVALID_PART
	wrongOrder := []partSpec{etags[1], etags[0], etags[2]}
	if _, err := completeMultipart(c, uploadID, wrongOrder); expectRPCError(err, "INVALID_PART", "wrong order") != nil {
		return expectRPCError(err, "INVALID_PART", "wrong order")
	}

	assembled := parts[0].body + parts[1].body + parts[2].body
	finalETag, err := completeMultipart(c, uploadID, etags)
	if err != nil {
		return err
	}
	if finalETag != bodyETag(assembled) {
		return fmt.Errorf("complete_multipart etag %q, expected SHA-256 of assembled body %q", finalETag, bodyETag(assembled))
	}

	res, err := get(c, key)
	if err != nil {
		return err
	}
	if res.Body != assembled {
		return fmt.Errorf("assembled object body %q, expected %q", res.Body, assembled)
	}

	if _, err := completeMultipart(c, uploadID, etags); expectRPCError(err, "NO_SUCH_UPLOAD", "complete after done") != nil {
		return expectRPCError(err, "NO_SUCH_UPLOAD", "complete after done")
	}
	ctx.Logf("multipart upload assembled parts in order")
	return nil
}

func testDurability(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}

	keep := map[string]string{
		"durable/a": "alpha",
		"durable/b": "bravo",
	}
	for k, v := range keep {
		if _, err := put(c, k, v); err != nil {
			return err
		}
	}
	if deleted, err := deleteKey(c, "durable/tmp"); err != nil || deleted {
		return fmt.Errorf("delete missing before crash: deleted=%v err=%v", deleted, err)
	}
	if _, err := put(c, "durable/tmp", "ephemeral"); err != nil {
		return err
	}
	if _, err := deleteKey(c, "durable/tmp"); err != nil {
		return err
	}

	ctx.Logf("wrote two objects, deleted a third — SIGKILL")
	c, err = restart(ctx, c)
	if err != nil {
		return err
	}
	defer c.Close()

	for k, want := range keep {
		res, err := get(c, k)
		if err != nil {
			return fmt.Errorf("after restart get %q: %w", k, err)
		}
		if res.Body != want {
			return fmt.Errorf("after restart %q: body %q, want %q", k, res.Body, want)
		}
	}
	if _, err := get(c, "durable/tmp"); expectRPCError(err, "NOT_FOUND", "deleted key after restart") != nil {
		return expectRPCError(err, "NOT_FOUND", "deleted key after restart")
	}
	ctx.Logf("objects and deletes survived the crash")
	return nil
}

func testGauntlet(ctx *harness.Context) error {
	c, err := ctx.Dial()
	if err != nil {
		return err
	}
	rng := rand.New(rand.NewSource(42))

	type expect struct {
		body string
		ok   bool
	}
	state := map[string]expect{}

	putObj := func(key, body string) error {
		if _, err := put(c, key, body); err != nil {
			return err
		}
		state[key] = expect{body: body, ok: true}
		return nil
	}
	delObj := func(key string) error {
		deleted, err := deleteKey(c, key)
		if err != nil {
			return err
		}
		if deleted {
			delete(state, key)
		}
		return nil
	}
	checkGet := func(key string) error {
		exp, exists := state[key]
		if !exists {
			if _, err := get(c, key); expectRPCError(err, "NOT_FOUND", "get "+key) != nil {
				return expectRPCError(err, "NOT_FOUND", "get "+key)
			}
			return nil
		}
		res, err := get(c, key)
		if err != nil {
			return err
		}
		if res.Body != exp.body {
			return fmt.Errorf("get %q: body %q, expected %q", key, res.Body, exp.body)
		}
		return nil
	}

	for round := 1; round <= 3; round++ {
		ctx.Logf("round %d: mixed ops then SIGKILL", round)
		for i := 0; i < 8; i++ {
			key := fmt.Sprintf("g/%d/%d", round, i)
			switch rng.Intn(3) {
			case 0:
				if err := putObj(key, fmt.Sprintf("body-%d-%d", round, i)); err != nil {
					return err
				}
			case 1:
				if err := putObj(key, strings.Repeat("x", rng.Intn(50)+1)); err != nil {
					return err
				}
			default:
				// multipart upload
				uploadID, err := createMultipart(c, key)
				if err != nil {
					return err
				}
				var bodies []string
				var specs []partSpec
				for p := 1; p <= 2+rng.Intn(2); p++ {
					b := fmt.Sprintf("part%d-%d", p, rng.Intn(100))
					etag, err := uploadPart(c, uploadID, p, b)
					if err != nil {
						return err
					}
					bodies = append(bodies, b)
					specs = append(specs, partSpec{PartNumber: p, ETag: etag})
				}
				final, err := completeMultipart(c, uploadID, specs)
				if err != nil {
					return err
				}
				assembled := strings.Join(bodies, "")
				if final != bodyETag(assembled) {
					return fmt.Errorf("multipart etag mismatch for %q", key)
				}
				state[key] = expect{body: assembled, ok: true}
			}
		}
		// delete a random existing key
		for k := range state {
			if rng.Float64() < 0.3 {
				if err := delObj(k); err != nil {
					return err
				}
			}
		}
		// verify a sample
		for k := range state {
			if err := checkGet(k); err != nil {
				return err
			}
		}
		c, err = restart(ctx, c)
		if err != nil {
			return err
		}
	}

	// Final verification + list sanity
	for k, exp := range state {
		res, err := get(c, k)
		if err != nil {
			return fmt.Errorf("final get %q: %w", k, err)
		}
		if res.Body != exp.body {
			return fmt.Errorf("final get %q: body mismatch", k)
		}
		h, err := head(c, k)
		if err != nil {
			return err
		}
		if h.Size != len(exp.body) {
			return fmt.Errorf("final head %q: size %d, want %d", k, h.Size, len(exp.body))
		}
	}
	keys, err := listKeys(c, "g/")
	if err != nil {
		return err
	}
	if len(keys) != len(state) {
		return fmt.Errorf("list g/: got %d keys, expected %d in state", len(keys), len(state))
	}
	c.Close()
	ctx.Logf("survived 3 crashes with puts, deletes, multipart, list, head")
	return nil
}
