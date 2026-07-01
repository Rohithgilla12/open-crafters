package opencrafters

import "testing"

func TestChallengePathsCoverCatalog(t *testing.T) {
	inPath := map[string]bool{}
	for _, p := range ChallengePaths {
		if p.Slug == "" {
			t.Fatal("path with empty slug")
		}
		for _, slug := range p.Challenges {
			if slug == "" {
				t.Fatalf("path %q has empty challenge slug", p.Slug)
			}
			if inPath[slug] {
				t.Fatalf("challenge %q appears in more than one path", slug)
			}
			inPath[slug] = true
		}
	}
	for _, slug := range ChallengeOrder {
		if !inPath[slug] {
			t.Fatalf("challenge %q in ChallengeOrder is not in any ChallengePath", slug)
		}
	}
	for slug := range inPath {
		found := false
		for _, ordered := range ChallengeOrder {
			if ordered == slug {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("path references unknown challenge %q", slug)
		}
	}
}
