package opencrafters

import (
	"strings"
)

// Walkthrough returns the full WALKTHROUGH.md for a challenge, if one is
// embedded. The walkthrough is post-pass teaching: how the reference solution
// approaches each stage, design-level.
func Walkthrough(slug string) (string, bool) {
	data, err := content.ReadFile(challengeDir(slug) + "/WALKTHROUGH.md")
	if err != nil {
		return "", false
	}
	return string(data), true
}

// HasWalkthrough reports whether a challenge ships a walkthrough.
func HasWalkthrough(slug string) bool {
	_, ok := Walkthrough(slug)
	return ok
}

// walkthroughSections splits a walkthrough into (stageSlug -> section body),
// keyed by the first token of each "## <slug> — <title>" heading. The returned
// order slice preserves document order.
func walkthroughSections(doc string) (map[string]string, []string) {
	sections := map[string]string{}
	var order []string
	var curSlug string
	var b strings.Builder
	flush := func() {
		if curSlug != "" {
			sections[curSlug] = strings.TrimSpace(b.String())
			b.Reset()
		}
	}
	for _, line := range strings.Split(doc, "\n") {
		if rest, ok := strings.CutPrefix(line, "## "); ok {
			flush()
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				curSlug = fields[0]
				order = append(order, curSlug)
			} else {
				curSlug = ""
			}
			b.WriteString(line + "\n")
			continue
		}
		if curSlug != "" {
			b.WriteString(line + "\n")
		}
	}
	flush()
	return sections, order
}

// WalkthroughSection returns the walkthrough section for one stage (heading
// plus body), matched by the stage slug.
func WalkthroughSection(slug, stageSlug string) (string, bool) {
	doc, ok := Walkthrough(slug)
	if !ok {
		return "", false
	}
	sections, _ := walkthroughSections(doc)
	body, ok := sections[stageSlug]
	return body, ok
}

// WalkthroughStageSlugs returns the stage slugs a challenge's walkthrough
// covers, in document order. Empty when there is no walkthrough.
func WalkthroughStageSlugs(slug string) []string {
	doc, ok := Walkthrough(slug)
	if !ok {
		return nil
	}
	_, order := walkthroughSections(doc)
	return order
}

// StageHint extracts the spoiler-free hint from a stage's walkthrough section:
// the leading "> ..." blockquote, with the markers stripped. It is the nudge
// shown when a learner is stuck, distinct from the full walkthrough body.
func StageHint(slug, stageSlug string) (string, bool) {
	section, ok := WalkthroughSection(slug, stageSlug)
	if !ok {
		return "", false
	}
	var quote []string
	started := false
	for _, line := range strings.Split(section, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ">") {
			started = true
			q := strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
			quote = append(quote, q)
			continue
		}
		if started {
			break // blockquote ended
		}
	}
	if len(quote) == 0 {
		return "", false
	}
	hint := strings.Join(quote, " ")
	hint = strings.TrimSpace(strings.TrimPrefix(hint, "**Hint:**"))
	return hint, true
}
