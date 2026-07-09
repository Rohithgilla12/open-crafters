package learn

import (
	"strings"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

// DesignBuildStepView is one implement-this build step on a design page.
type DesignBuildStepView struct {
	Num          int
	Blurb        string
	Challenge    *Challenge
	StartCommand string
	IsCompose    bool
}

// DesignStackLink is a short link to a design stack from a problem or challenge page.
type DesignStackLink struct {
	Slug string
	Name string
}

// DesignStackMilestoneView is one milestone on a stack page.
type DesignStackMilestoneView struct {
	Num          int
	Kind         string
	Blurb        string
	Design       *DesignProblem
	Challenge    *Challenge
	StartCommand string
	Href         string
	Label        string
	IsCompose    bool
	IsCapstone   bool
}

// DesignStackView is a rendered whiteboard→build journey.
type DesignStackView struct {
	Slug               string
	Name               string
	Tagline            string
	Description        string
	Outcomes           []string
	Milestones         []DesignStackMilestoneView
	StepCSV            string
	TotalSteps         int
	HasComposeCapstone bool
}

func enrichDesignBridges(c *Catalog) {
	for slug, dp := range c.Designs {
		steps := opencrafters.BuildStepsForDesign(slug)
		for i, step := range steps {
			v := DesignBuildStepView{
				Num:          i + 1,
				Blurb:        step.Blurb,
				StartCommand: "crafters start " + strings.TrimPrefix(step.Challenge, "build-your-own-"),
			}
			if ch, ok := c.Challenges[step.Challenge]; ok {
				v.Challenge = ch
				v.IsCompose = ch.IsCompose
			}
			dp.BuildSteps = append(dp.BuildSteps, v)
		}
		for _, stSlug := range opencrafters.DesignStacksForDesign(slug) {
			if st, ok := c.GetDesignStack(stSlug); ok {
				dp.Stacks = append(dp.Stacks, DesignStackLink{Slug: st.Slug, Name: st.Name})
			}
		}
	}
}

func loadDesignStacks(c *Catalog) {
	for _, def := range opencrafters.DesignStacks {
		sv := DesignStackView{
			Slug:        def.Slug,
			Name:        def.Name,
			Tagline:     def.Tagline,
			Description: def.Description,
			Outcomes:    append([]string(nil), def.Outcomes...),
			TotalSteps:  len(def.Milestones),
		}
		var csv []string
		for i, m := range def.Milestones {
			mv := DesignStackMilestoneView{Num: i + 1, Kind: m.Kind, Blurb: m.Blurb}
			csv = append(csv, m.Kind+":"+m.Slug)
			switch m.Kind {
			case "design":
				mv.Href = "/design/" + m.Slug
				mv.Label = "Whiteboard"
				if dp, ok := c.Designs[m.Slug]; ok {
					mv.Design = dp
					mv.Label = dp.Name
				}
			case "build":
				mv.Href = "/challenges/" + m.Slug
				mv.StartCommand = "crafters start " + strings.TrimPrefix(m.Slug, "build-your-own-")
				mv.Label = "Build"
				if ch, ok := c.Challenges[m.Slug]; ok {
					mv.Challenge = ch
					mv.Label = ch.Name
					mv.IsCompose = ch.IsCompose
				}
			}
			if i == len(def.Milestones)-1 && mv.IsCompose {
				mv.IsCapstone = true
				sv.HasComposeCapstone = true
			}
			sv.Milestones = append(sv.Milestones, mv)
		}
		sv.StepCSV = strings.Join(csv, ",")
		c.DesignStacks = append(c.DesignStacks, sv)
	}
}

func (c *Catalog) GetDesignStack(slug string) (*DesignStackView, bool) {
	for i := range c.DesignStacks {
		if c.DesignStacks[i].Slug == slug {
			return &c.DesignStacks[i], true
		}
	}
	return nil, false
}

func (c *Catalog) DesignsForChallenge(challengeSlug string) []*DesignProblem {
	var out []*DesignProblem
	for _, dSlug := range opencrafters.DesignProblemsForChallenge(challengeSlug) {
		if dp, ok := c.Designs[dSlug]; ok {
			out = append(out, dp)
		}
	}
	return out
}

func (c *Catalog) StacksForChallenge(challengeSlug string) []DesignStackLink {
	var out []DesignStackLink
	for _, stSlug := range opencrafters.DesignStacksForChallenge(challengeSlug) {
		if st, ok := c.GetDesignStack(stSlug); ok {
			out = append(out, DesignStackLink{Slug: st.Slug, Name: st.Name})
		}
	}
	return out
}
