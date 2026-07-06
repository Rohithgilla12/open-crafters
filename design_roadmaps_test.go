package opencrafters

import "testing"

func TestDesignRoadmapsReferenceProblems(t *testing.T) {
	known := map[string]bool{}
	for _, slug := range DesignProblemOrder {
		known[slug] = true
	}
	for _, rm := range DesignRoadmaps {
		if rm.Slug == "" || rm.Name == "" {
			t.Fatalf("roadmap missing slug or name: %+v", rm)
		}
		if len(rm.Milestones) == 0 {
			t.Fatalf("roadmap %q has no milestones", rm.Slug)
		}
		for _, m := range rm.Milestones {
			if !known[m.Problem] {
				t.Errorf("roadmap %q references unknown problem %q", rm.Slug, m.Problem)
			}
		}
	}
}
