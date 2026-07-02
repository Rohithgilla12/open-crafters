package main

import (
	"fmt"
	"strings"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

func cmdRoadmap(args []string) {
	if len(args) == 0 {
		for _, r := range opencrafters.Roadmaps {
			fmt.Printf("\x1b[1m%s\x1b[0m — %s\n", r.Slug, r.Name)
			fmt.Printf("  %s\n", r.Tagline)
			n := len(opencrafters.ChallengesForRoadmap(r))
			fmt.Printf("  %d milestones · crafters roadmap %s\n", n, r.Slug)
		}
		return
	}
	slug := args[0]
	def, ok := opencrafters.RoadmapBySlug(slug)
	if !ok {
		die("unknown roadmap %q (try: crafters roadmap)", slug)
	}
	fmt.Printf("\x1b[1m%s\x1b[0m — %s\n", def.Name, def.Tagline)
	fmt.Printf("%s\n\n", def.Description)
	if len(def.Outcomes) > 0 {
		fmt.Println("Outcomes:")
		for _, o := range def.Outcomes {
			fmt.Printf("  • %s\n", o)
		}
		fmt.Println()
	}
	challengeSlugs := opencrafters.ChallengesForRoadmap(def)
	if len(challengeSlugs) > 0 {
		start := strings.TrimPrefix(challengeSlugs[0], "build-your-own-")
		fmt.Printf("Start: crafters start %s\n\n", start)
	}
	fmt.Println("Milestones:")
	for i, m := range def.Milestones {
		if strings.HasPrefix(m.Challenge, "__path:") {
			pathSlug := m.Challenge[7 : len(m.Challenge)-2]
			fmt.Printf("  %2d. [%s path] — %s\n", i+1, pathSlug, m.Blurb)
			continue
		}
		ch := challenges[m.Challenge]
		fmt.Printf("  %2d. %s — %s\n", i+1, ch.Name, m.Blurb)
	}
}
