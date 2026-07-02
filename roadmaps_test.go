package opencrafters

import "testing"

func TestRoadmapsAlignWithPaths(t *testing.T) {
	pathSlugs := map[string]bool{}
	for _, p := range ChallengePaths {
		pathSlugs[p.Slug] = true
	}
	for _, r := range Roadmaps {
		if r.Slug == "" {
			t.Fatal("roadmap with empty slug")
		}
		if r.Slug == "platform" {
			continue
		}
		if r.PathSlug == "" {
			t.Fatalf("roadmap %q missing PathSlug", r.Slug)
		}
		if !pathSlugs[r.PathSlug] {
			t.Fatalf("roadmap %q references unknown path %q", r.Slug, r.PathSlug)
		}
		challenges := ChallengesForRoadmap(r)
		if len(r.Milestones) != len(challenges) {
			t.Fatalf("roadmap %q: %d milestones vs %d challenges", r.Slug, len(r.Milestones), len(challenges))
		}
		for i, m := range r.Milestones {
			if m.Blurb == "" {
				t.Fatalf("roadmap %q milestone %d missing blurb", r.Slug, i)
			}
			if m.Challenge != challenges[i] {
				t.Fatalf("roadmap %q milestone[%d] challenge %q != path challenge %q", r.Slug, i, m.Challenge, challenges[i])
			}
		}
	}
}

func TestPlatformRoadmapCoversAllChallenges(t *testing.T) {
	var platform Roadmap
	for _, r := range Roadmaps {
		if r.Slug == "platform" {
			platform = r
		}
	}
	if platform.Slug == "" {
		t.Fatal("missing platform roadmap")
	}
	got := map[string]bool{}
	for _, slug := range ChallengesForRoadmap(platform) {
		got[slug] = true
	}
	for _, slug := range ChallengeOrder {
		if !got[slug] {
			t.Fatalf("platform roadmap missing challenge %q", slug)
		}
	}
}
