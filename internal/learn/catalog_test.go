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
}
