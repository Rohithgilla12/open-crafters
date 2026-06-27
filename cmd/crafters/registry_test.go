package main

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

// These tests are the harness's self-checks: they guard the invariants that
// keep a challenge gradeable and shippable, so that adding or editing a
// challenge can't silently drift the Go stage registry, the challenge.yaml,
// the embedded instructions, the starters, and the reference solutions out of
// sync. They run as part of `go test ./...`, no subprocess grading required.

// requiredLangs every challenge must ship a starter and reference for.
var requiredLangs = []string{"go", "python", "typescript"}

// repoRoot locates the module root from this test file's path so the tests can
// reach examples/ (which is not embedded) regardless of the working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

// TestOrderMatchesRegistry: every registered challenge is ordered exactly once,
// and challengeOrder lists nothing that isn't registered.
func TestOrderMatchesRegistry(t *testing.T) {
	if len(challengeOrder) != len(challenges) {
		t.Fatalf("challengeOrder has %d entries but %d challenges are registered", len(challengeOrder), len(challenges))
	}
	seen := map[string]bool{}
	for _, slug := range challengeOrder {
		if _, ok := challenges[slug]; !ok {
			t.Errorf("challengeOrder lists %q which is not registered in the challenges map", slug)
		}
		if seen[slug] {
			t.Errorf("challengeOrder lists %q more than once", slug)
		}
		seen[slug] = true
	}
	for slug := range challenges {
		if !seen[slug] {
			t.Errorf("challenge %q is registered but missing from challengeOrder", slug)
		}
	}
}

// TestStagesAreRunnable: every stage has instructions that exist in the
// embedded content and a test function appropriate to the challenge shape.
func TestStagesAreRunnable(t *testing.T) {
	cfs := opencrafters.ChallengesFS()
	for slug, ch := range challenges {
		if ch.Slug != slug {
			t.Errorf("challenge registered under %q reports Slug %q", slug, ch.Slug)
		}
		if len(ch.Stages) == 0 {
			t.Errorf("%s: no stages", slug)
			continue
		}
		stageSlugs := map[string]bool{}
		for _, st := range ch.Stages {
			if stageSlugs[st.Slug] {
				t.Errorf("%s: duplicate stage slug %q", slug, st.Slug)
			}
			stageSlugs[st.Slug] = true
			if st.Instructions == "" {
				t.Errorf("%s/%s: empty Instructions", slug, st.Slug)
			} else if _, err := fs.Stat(cfs, strings.TrimPrefix(st.Instructions, "challenges/")); err != nil {
				t.Errorf("%s/%s: instructions file %q not found in embedded content: %v", slug, st.Slug, st.Instructions, err)
			}
			// Cluster challenges grade via TestCluster; single-process via Test.
			if ch.ClusterSize > 0 {
				if st.TestCluster == nil {
					t.Errorf("%s/%s: cluster challenge but TestCluster is nil", slug, st.Slug)
				}
			} else if st.Test == nil {
				t.Errorf("%s/%s: Test is nil", slug, st.Slug)
			}
		}
		if ch.Stages[0].Slug != "bind" {
			t.Errorf("%s: first stage is %q, expected a 'bind' hello-world stage", slug, ch.Stages[0].Slug)
		}
	}
}

// TestYAMLStagesMatchRegistry: the stage slugs (and their order) declared in
// each challenge.yaml match the Go stage registry exactly. This is the check
// that catches "I added a stage to one but not the other".
func TestYAMLStagesMatchRegistry(t *testing.T) {
	cfs := opencrafters.ChallengesFS()
	for slug, ch := range challenges {
		y, err := fs.ReadFile(cfs, path.Join(slug, "challenge.yaml"))
		if err != nil {
			t.Errorf("%s: reading challenge.yaml: %v", slug, err)
			continue
		}
		yamlSlugs := yamlStageSlugs(y)
		if len(yamlSlugs) != len(ch.Stages) {
			t.Errorf("%s: challenge.yaml lists %d stages, Go registry has %d", slug, len(yamlSlugs), len(ch.Stages))
			continue
		}
		for i, st := range ch.Stages {
			if yamlSlugs[i] != st.Slug {
				t.Errorf("%s: stage %d is %q in challenge.yaml but %q in the Go registry", slug, i+1, yamlSlugs[i], st.Slug)
			}
		}
	}
}

// TestStartersAndReferencesExist: every challenge ships a starter and a
// reference solution (with a your_program.sh entry point) in each required
// language, plus a PROTOCOL.md.
func TestStartersAndReferencesExist(t *testing.T) {
	cfs := opencrafters.ChallengesFS()
	root := repoRoot(t)
	for slug := range challenges {
		if _, err := fs.Stat(cfs, path.Join(slug, "PROTOCOL.md")); err != nil {
			t.Errorf("%s: missing PROTOCOL.md", slug)
		}
		for _, lang := range requiredLangs {
			starter := path.Join(slug, "starters", lang, "your_program.sh")
			if _, err := fs.Stat(cfs, starter); err != nil {
				t.Errorf("%s: missing starter %s (%s)", slug, lang, starter)
			}
			ref := filepath.Join(root, "examples", "solutions", slug, lang, "your_program.sh")
			if _, err := os.Stat(ref); err != nil {
				t.Errorf("%s: missing reference solution %s (%s)", slug, lang, ref)
			}
		}
	}
}

// TestWalkthroughCoverage: a walkthrough is optional, but when a challenge
// ships one it must cover every stage and every section must carry a hint —
// so the post-pass walkthrough and the `crafters hint` nudge can't go stale
// relative to the stage ladder.
func TestWalkthroughCoverage(t *testing.T) {
	for slug, ch := range challenges {
		if !opencrafters.HasWalkthrough(slug) {
			continue
		}
		// Build a registered-stage set and a count of how often each appears as
		// a walkthrough section. The two must correspond exactly: every stage
		// covered, no duplicate sections, and no stale sections for stages that
		// no longer exist.
		registered := map[string]bool{}
		for _, st := range ch.Stages {
			registered[st.Slug] = true
		}
		seen := map[string]int{}
		for _, s := range opencrafters.WalkthroughStageSlugs(slug) {
			seen[s]++
			if seen[s] == 1 && !registered[s] {
				t.Errorf("%s: walkthrough has a section for %q, which is not a registered stage", slug, s)
			}
			if seen[s] == 2 {
				t.Errorf("%s: walkthrough has a duplicate section for stage %q", slug, s)
			}
		}
		for _, st := range ch.Stages {
			if seen[st.Slug] == 0 {
				t.Errorf("%s: walkthrough is missing a section for stage %q", slug, st.Slug)
				continue
			}
			if hint, ok := opencrafters.StageHint(slug, st.Slug); !ok || strings.TrimSpace(hint) == "" {
				t.Errorf("%s/%s: walkthrough section has no hint (expected a leading > blockquote)", slug, st.Slug)
			}
		}
	}
}

// yamlStageSlugs extracts the ordered `- slug:` values under the top-level
// `stages:` key of a challenge.yaml. The repo hand-parses YAML (no dependency),
// so this mirrors that minimal approach.
func yamlStageSlugs(y []byte) []string {
	var slugs []string
	inStages := false
	for _, line := range strings.Split(string(y), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// A new top-level key ends the stages block.
			inStages = strings.HasPrefix(trimmed, "stages:")
			continue
		}
		if !inStages {
			continue
		}
		if strings.HasPrefix(trimmed, "- slug:") {
			slugs = append(slugs, strings.TrimSpace(strings.TrimPrefix(trimmed, "- slug:")))
		}
	}
	return slugs
}
