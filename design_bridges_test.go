package opencrafters

import "testing"

func TestDesignBuildStepsCoverRelatedChallenges(t *testing.T) {
	for _, d := range DesignProblems {
		steps := DesignBuildSteps[d.Slug]
		if len(steps) == 0 {
			t.Errorf("%s: no DesignBuildSteps entry", d.Slug)
			continue
		}
		got := map[string]bool{}
		for _, s := range steps {
			got[s.Challenge] = true
		}
		for _, ch := range d.RelatedChallenges {
			if !got[ch] {
				t.Errorf("%s: BuildSteps missing related challenge %q", d.Slug, ch)
			}
		}
	}
}

func TestDesignStacksValid(t *testing.T) {
	designKnown := map[string]bool{}
	for _, s := range DesignProblemOrder {
		designKnown[s] = true
	}
	buildKnown := map[string]bool{}
	for _, s := range ChallengeOrder {
		buildKnown[s] = true
	}
	for _, st := range DesignStacks {
		if len(st.Milestones) < 2 {
			t.Errorf("stack %q needs at least 2 milestones", st.Slug)
		}
		for _, m := range st.Milestones {
			switch m.Kind {
			case "design":
				if !designKnown[m.Slug] {
					t.Errorf("stack %q: unknown design %q", st.Slug, m.Slug)
				}
			case "build":
				if !buildKnown[m.Slug] {
					t.Errorf("stack %q: unknown build %q", st.Slug, m.Slug)
				}
			default:
				t.Errorf("stack %q: invalid kind %q", st.Slug, m.Kind)
			}
		}
	}
}
