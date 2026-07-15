package learn

import (
	"testing"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

func TestCatalogListsAllChallenges(t *testing.T) {
	c, err := NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Order) != len(ChallengeOrder) {
		t.Fatalf("catalog has %d challenges, want %d", len(c.Order), len(ChallengeOrder))
	}
	for i, slug := range ChallengeOrder {
		if c.Order[i] != slug {
			t.Fatalf("order[%d] = %q, want %q", i, c.Order[i], slug)
		}
		if _, ok := c.Challenges[slug]; !ok {
			t.Fatalf("missing challenge %q", slug)
		}
	}
	if len(c.Paths) != len(opencrafters.ChallengePaths) {
		t.Fatalf("catalog has %d paths, want %d", len(c.Paths), len(opencrafters.ChallengePaths))
	}
	if len(c.Roadmaps) != len(opencrafters.Roadmaps) {
		t.Fatalf("catalog has %d roadmaps, want %d", len(c.Roadmaps), len(opencrafters.Roadmaps))
	}
	if len(c.DesignOrder) != len(opencrafters.DesignProblemOrder) {
		t.Fatalf("catalog has %d design problems, want %d", len(c.DesignOrder), len(opencrafters.DesignProblemOrder))
	}
	if len(c.DesignRoadmaps) != len(opencrafters.DesignRoadmaps) {
		t.Fatalf("catalog has %d design roadmaps, want %d", len(c.DesignRoadmaps), len(opencrafters.DesignRoadmaps))
	}
	if len(c.DesignStacks) != len(opencrafters.DesignStacks) {
		t.Fatalf("catalog has %d design stacks, want %d", len(c.DesignStacks), len(opencrafters.DesignStacks))
	}
	for _, slug := range opencrafters.DesignProblemOrder {
		if _, ok := c.Designs[slug]; !ok {
			t.Fatalf("missing design problem %q", slug)
		}
		dp := c.Designs[slug]
		if len(dp.BuildSteps) == 0 {
			t.Fatalf("design %q has no build steps", slug)
		}
	}
	for _, slug := range opencrafters.ComposeChallenges {
		ch, ok := c.Challenges[slug]
		if !ok {
			t.Fatalf("missing compose challenge %q", slug)
		}
		if !ch.IsCompose {
			t.Fatalf("challenge %q should have IsCompose=true", slug)
		}
	}
	composeStacks := 0
	for _, st := range c.DesignStacks {
		if st.HasComposeCapstone {
			composeStacks++
			last := st.Milestones[len(st.Milestones)-1]
			if !last.IsCapstone || !last.IsCompose {
				t.Fatalf("stack %q: expected compose capstone on last milestone", st.Slug)
			}
		}
	}
	if composeStacks != 8 {
		t.Fatalf("want 8 design stacks with compose capstones, got %d", composeStacks)
	}
}
