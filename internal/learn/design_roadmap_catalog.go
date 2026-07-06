package learn

import (
	"strings"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

// DesignRoadmapMilestoneView is one step on a design roadmap page.
type DesignRoadmapMilestoneView struct {
	Num     int
	Blurb   string
	Problem *DesignProblem
}

// DesignRoadmapView is a rendered design learning journey.
type DesignRoadmapView struct {
	Slug          string
	Name          string
	Tagline       string
	Description   string
	Outcomes      []string
	Milestones    []DesignRoadmapMilestoneView
	ProblemCSV    string
	TotalProblems int
	StartProblem  string
}

func loadDesignRoadmaps(c *Catalog) {
	for _, def := range opencrafters.DesignRoadmaps {
		rv := DesignRoadmapView{
			Slug:        def.Slug,
			Name:        def.Name,
			Tagline:     def.Tagline,
			Description: def.Description,
			Outcomes:    append([]string(nil), def.Outcomes...),
		}
		problems := opencrafters.DesignProblemsForRoadmap(def)
		rv.ProblemCSV = strings.Join(problems, ",")
		rv.TotalProblems = len(problems)
		if len(problems) > 0 {
			rv.StartProblem = problems[0]
		}
		for i, m := range def.Milestones {
			mv := DesignRoadmapMilestoneView{Num: i + 1, Blurb: m.Blurb}
			if dp, ok := c.Designs[m.Problem]; ok {
				mv.Problem = dp
			}
			rv.Milestones = append(rv.Milestones, mv)
		}
		c.DesignRoadmaps = append(c.DesignRoadmaps, rv)
	}
}

func (c *Catalog) GetDesignRoadmap(slug string) (*DesignRoadmapView, bool) {
	for i := range c.DesignRoadmaps {
		if c.DesignRoadmaps[i].Slug == slug {
			return &c.DesignRoadmaps[i], true
		}
	}
	return nil, false
}

// APIDesignRoadmap is the JSON shape for /api/design/roadmaps.
type APIDesignRoadmap struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Tagline       string   `json:"tagline"`
	Description   string   `json:"description"`
	Outcomes      []string `json:"outcomes"`
	Problems      []string `json:"problems"`
	TotalProblems int      `json:"total_problems"`
}

func (c *Catalog) APIDesignRoadmaps() []APIDesignRoadmap {
	out := make([]APIDesignRoadmap, 0, len(c.DesignRoadmaps))
	for _, r := range c.DesignRoadmaps {
		var problems []string
		if r.ProblemCSV != "" {
			problems = strings.Split(r.ProblemCSV, ",")
		}
		out = append(out, APIDesignRoadmap{
			Slug:          r.Slug,
			Name:          r.Name,
			Tagline:       r.Tagline,
			Description:   r.Description,
			Outcomes:      append([]string(nil), r.Outcomes...),
			Problems:      problems,
			TotalProblems: r.TotalProblems,
		})
	}
	return out
}
