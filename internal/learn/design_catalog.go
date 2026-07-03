package learn

import (
	"fmt"
	"html/template"
	"io/fs"
	"path"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

// DesignProblem is a rendered system design scenario.
type DesignProblem struct {
	Slug              string `json:"slug"`
	Name              string `json:"name"`
	Tagline           string `json:"tagline"`
	Difficulty        string `json:"difficulty"`
	Category          string `json:"category"`
	TimeMinutes       int    `json:"time_minutes"`
	RelatedChallenges []string
	Related           []*Challenge // resolved from catalog
	DiscussionPrompts []string     `json:"discussion_prompts"`
	ProblemHTML       template.HTML
	HintsHTML         template.HTML
	SolutionHTML      template.HTML
}

func loadDesignProblems(c *Catalog, md *renderer) error {
	dfs := opencrafters.DesignFS()
	c.Designs = make(map[string]*DesignProblem)

	for _, slug := range opencrafters.DesignProblemOrder {
		def, ok := opencrafters.DesignBySlug(slug)
		if !ok {
			return fmt.Errorf("design problem %q not in registry", slug)
		}
		for _, f := range []string{"PROBLEM.md", "HINTS.md", "SOLUTION.md"} {
			if _, err := fs.Stat(dfs, path.Join(slug, f)); err != nil {
				return fmt.Errorf("design/%s/%s: %w", slug, f, err)
			}
		}

		dp := &DesignProblem{
			Slug:              def.Slug,
			Name:              def.Name,
			Tagline:           def.Tagline,
			Difficulty:        def.Difficulty,
			Category:          def.Category,
			TimeMinutes:       def.TimeMinutes,
			RelatedChallenges: append([]string(nil), def.RelatedChallenges...),
			DiscussionPrompts: append([]string(nil), def.DiscussionPrompts...),
		}
		for _, chSlug := range def.RelatedChallenges {
			if ch, ok := c.Challenges[chSlug]; ok {
				dp.Related = append(dp.Related, ch)
			}
		}

		var err error
		if dp.ProblemHTML, err = md.renderDesign(slug, "PROBLEM.md"); err != nil {
			return fmt.Errorf("rendering design/%s/PROBLEM.md: %w", slug, err)
		}
		if dp.HintsHTML, err = md.renderDesign(slug, "HINTS.md"); err != nil {
			return fmt.Errorf("rendering design/%s/HINTS.md: %w", slug, err)
		}
		if dp.SolutionHTML, err = md.renderDesign(slug, "SOLUTION.md"); err != nil {
			return fmt.Errorf("rendering design/%s/SOLUTION.md: %w", slug, err)
		}

		c.Designs[slug] = dp
		c.DesignOrder = append(c.DesignOrder, slug)
	}
	return nil
}

// APIDesign is the JSON shape for /api/design.
type APIDesign struct {
	Slug              string   `json:"slug"`
	Name              string   `json:"name"`
	Tagline           string   `json:"tagline"`
	Difficulty        string   `json:"difficulty"`
	Category          string   `json:"category"`
	TimeMinutes       int      `json:"time_minutes"`
	RelatedChallenges []string `json:"related_challenges"`
	DiscussionPrompts []string `json:"discussion_prompts"`
}

func (c *Catalog) APIDesignList() []APIDesign {
	out := make([]APIDesign, 0, len(c.DesignOrder))
	for _, slug := range c.DesignOrder {
		d := c.Designs[slug]
		out = append(out, APIDesign{
			Slug:              d.Slug,
			Name:              d.Name,
			Tagline:           d.Tagline,
			Difficulty:        d.Difficulty,
			Category:          d.Category,
			TimeMinutes:       d.TimeMinutes,
			RelatedChallenges: append([]string(nil), d.RelatedChallenges...),
			DiscussionPrompts: append([]string(nil), d.DiscussionPrompts...),
		})
	}
	return out
}

func (c *Catalog) GetDesign(slug string) (*DesignProblem, bool) {
	d, ok := c.Designs[slug]
	return d, ok
}
