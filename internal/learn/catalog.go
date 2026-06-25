package learn

import (
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

// ChallengeOrder is the canonical display order (WAL first — the recommended
// starting challenge). Matches cmd/crafters/main.go challengeOrder.
var ChallengeOrder = []string{
	"build-your-own-wal",
	"build-your-own-queue",
	"build-your-own-log",
	"build-your-own-mvcc",
	"build-your-own-temporal",
	"build-your-own-workflow-sdk",
	"build-your-own-raft",
	"build-your-own-scheduler",
}

// Stage is one step of a challenge.
type Stage struct {
	Num          int
	Slug         string
	Name         string
	Difficulty   string
	Instructions string // path relative to challenge root, e.g. stages/01-bind.md
	HTML         template.HTML
}

// Challenge is a catalog entry with rendered content.
type Challenge struct {
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	Tagline      string `json:"tagline"`
	Difficulty   string `json:"difficulty"`
	Stages       []Stage
	DiffMix      template.HTML
	ProtocolHTML template.HTML
}

// Catalog holds all challenges indexed by slug.
type Catalog struct {
	Order      []string
	Challenges map[string]*Challenge
}

// NewCatalog builds the challenge catalog from the embedded challenges FS.
func NewCatalog() (*Catalog, error) {
	cfs := opencrafters.ChallengesFS()
	md := newRenderer()

	c := &Catalog{
		Challenges: make(map[string]*Challenge),
	}

	for _, slug := range ChallengeOrder {
		y, err := fs.ReadFile(cfs, path.Join(slug, "challenge.yaml"))
		if err != nil {
			return nil, fmt.Errorf("reading %s/challenge.yaml: %w", slug, err)
		}

		ch := &Challenge{
			Slug:       slug,
			Name:       yamlField(y, "name"),
			Tagline:    tagline(y),
			Difficulty: yamlField(y, "difficulty"),
		}

		if html, err := md.render(path.Join(slug, "PROTOCOL.md")); err == nil {
			ch.ProtocolHTML = html
		}

		stages := parseStages(y)
		for i, s := range stages {
			inst := path.Join(slug, s.Instructions)
			html, err := md.render(inst)
			if err != nil {
				return nil, fmt.Errorf("rendering %s: %w", inst, err)
			}
			ch.Stages = append(ch.Stages, Stage{
				Num:          i + 1,
				Slug:         s.Slug,
				Name:         s.Name,
				Difficulty:   s.Difficulty,
				Instructions: s.Instructions,
				HTML:         html,
			})
		}
		ch.DiffMix = difficultyMix(ch.Stages)
		c.Challenges[slug] = ch
		c.Order = append(c.Order, slug)
	}

	return c, nil
}

func (c *Catalog) Get(slug string) (*Challenge, bool) {
	ch, ok := c.Challenges[slug]
	return ch, ok
}

func (c *Catalog) Stage(slug, stageSlug string) (*Challenge, *Stage, bool) {
	ch, ok := c.Challenges[slug]
	if !ok {
		return nil, nil, false
	}
	for i := range ch.Stages {
		if ch.Stages[i].Slug == stageSlug {
			return ch, &ch.Stages[i], true
		}
	}
	return nil, nil, false
}

type yamlStage struct {
	Slug, Name, Difficulty, Instructions string
}

func parseStages(y []byte) []yamlStage {
	var stages []yamlStage
	var cur *yamlStage
	inStages := false
	for _, ln := range strings.Split(string(y), "\n") {
		if strings.TrimSpace(ln) == "stages:" {
			inStages = true
			continue
		}
		if !inStages {
			continue
		}
		if strings.HasPrefix(ln, "  - slug:") {
			if cur != nil {
				stages = append(stages, *cur)
			}
			cur = &yamlStage{Slug: strings.TrimSpace(strings.TrimPrefix(ln, "  - slug:"))}
			continue
		}
		if cur == nil {
			continue
		}
		trimmed := strings.TrimSpace(ln)
		switch {
		case strings.HasPrefix(trimmed, "name:"):
			cur.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
		case strings.HasPrefix(trimmed, "difficulty:"):
			cur.Difficulty = strings.TrimSpace(strings.TrimPrefix(trimmed, "difficulty:"))
		case strings.HasPrefix(trimmed, "instructions:"):
			cur.Instructions = strings.TrimSpace(strings.TrimPrefix(trimmed, "instructions:"))
		}
	}
	if cur != nil {
		stages = append(stages, *cur)
	}
	return stages
}

func yamlField(y []byte, key string) string {
	for _, ln := range strings.Split(string(y), "\n") {
		if strings.HasPrefix(ln, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(ln, key+":"))
		}
	}
	return ""
}

func tagline(y []byte) string {
	var out []string
	capturing := false
	for _, ln := range strings.Split(string(y), "\n") {
		if strings.HasPrefix(ln, "short_description:") {
			capturing = true
			continue
		}
		if !capturing {
			continue
		}
		if strings.TrimSpace(ln) == "" || strings.HasPrefix(ln, " ") || strings.HasPrefix(ln, "\t") {
			if t := strings.TrimSpace(ln); t != "" {
				out = append(out, t)
			}
			continue
		}
		break
	}
	return strings.Join(out, " ")
}

func difficultyMix(stages []Stage) template.HTML {
	n := map[string]int{}
	for _, s := range stages {
		n[s.Difficulty]++
	}
	var parts []string
	for _, d := range []string{"easy", "medium", "hard"} {
		if n[d] > 0 {
			parts = append(parts, fmt.Sprintf(`<span class="diff diff-%s">%d %s</span>`, d, n[d], d))
		}
	}
	return template.HTML(strings.Join(parts, " ")) //nolint:gosec // fixed, internal strings
}

// APIChallenge is the JSON shape for /api/challenges.
type APIChallenge struct {
	Slug       string     `json:"slug"`
	Name       string     `json:"name"`
	Tagline    string     `json:"tagline"`
	Difficulty string     `json:"difficulty"`
	Stages     []APIStage `json:"stages"`
}

// APIStage is a stage summary in the JSON API.
type APIStage struct {
	Num        int    `json:"num"`
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	Difficulty string `json:"difficulty"`
}

func (c *Catalog) APIList() []APIChallenge {
	out := make([]APIChallenge, 0, len(c.Order))
	for _, slug := range c.Order {
		ch := c.Challenges[slug]
		ac := APIChallenge{
			Slug:       ch.Slug,
			Name:       ch.Name,
			Tagline:    ch.Tagline,
			Difficulty: ch.Difficulty,
		}
		for _, s := range ch.Stages {
			ac.Stages = append(ac.Stages, APIStage{
				Num:        s.Num,
				Slug:       s.Slug,
				Name:       s.Name,
				Difficulty: s.Difficulty,
			})
		}
		out = append(out, ac)
	}
	return out
}
