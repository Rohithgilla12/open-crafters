package opencrafters

import "testing"

func TestChallengeOrderUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, slug := range ChallengeOrder {
		if slug == "" {
			t.Fatal("empty slug in ChallengeOrder")
		}
		if seen[slug] {
			t.Fatalf("duplicate slug %q in ChallengeOrder", slug)
		}
		seen[slug] = true
	}
}
