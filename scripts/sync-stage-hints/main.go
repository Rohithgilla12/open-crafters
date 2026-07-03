package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

const (
	startMarker = "<!-- crafters-stage-hint -->"
	endMarker   = "<!-- /crafters-stage-hint -->"
)

type yamlStage struct {
	slug         string
	instructions string
}

func main() {
	root, err := findRepoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "find repo root: %v\n", err)
		os.Exit(1)
	}

	var updated []string
	for _, slug := range opencrafters.ChallengeOrder {
		challengeYAML := filepath.Join(root, "challenges", slug, "challenge.yaml")
		y, err := os.ReadFile(challengeYAML)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", challengeYAML, err)
			os.Exit(1)
		}
		for _, st := range yamlStages(y) {
			if st.instructions == "" {
				continue
			}
			hint, ok := opencrafters.StageHint(slug, st.slug)
			if !ok {
				continue
			}
			instPath := filepath.Join(root, "challenges", slug, st.instructions)
			old, err := os.ReadFile(instPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "read %s: %v\n", instPath, err)
				os.Exit(1)
			}
			newContent := applyHintBlock(string(old), hintBlock(slug, st.slug, hint))
			if newContent == string(old) {
				continue
			}
			if err := os.WriteFile(instPath, []byte(newContent), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "write %s: %v\n", instPath, err)
				os.Exit(1)
			}
			rel, _ := filepath.Rel(root, instPath)
			updated = append(updated, rel)
		}
	}

	fmt.Printf("Updated %d file(s)\n", len(updated))
	for _, p := range updated {
		fmt.Println(p)
	}
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func yamlStages(y []byte) []yamlStage {
	var out []yamlStage
	inStages := false
	var cur *yamlStage
	for _, line := range strings.Split(string(y), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inStages = strings.HasPrefix(trimmed, "stages:")
			if !inStages {
				cur = nil
			}
			continue
		}
		if !inStages {
			continue
		}
		if strings.HasPrefix(trimmed, "- slug:") {
			s := strings.TrimSpace(strings.TrimPrefix(trimmed, "- slug:"))
			out = append(out, yamlStage{slug: s})
			cur = &out[len(out)-1]
			continue
		}
		if cur != nil && strings.HasPrefix(trimmed, "instructions:") {
			cur.instructions = strings.TrimSpace(strings.TrimPrefix(trimmed, "instructions:"))
		}
	}
	return out
}

func hintBlock(challengeSlug, stageSlug, hint string) string {
	short := strings.TrimPrefix(challengeSlug, "build-your-own-")
	return fmt.Sprintf(`---

%s
## Stuck?

<details>
<summary><strong>Spoiler-free hint</strong></summary>

> **Hint:** %s

Or run: <code>crafters hint %s --stage %s</code>
</details>
%s
`, startMarker, hint, short, stageSlug, endMarker)
}

func applyHintBlock(content, block string) string {
	block = strings.TrimRight(block, "\n") + "\n"
	if start := strings.Index(content, startMarker); start >= 0 {
		if endRel := strings.Index(content[start:], endMarker); endRel >= 0 {
			end := start + endRel + len(endMarker)
			prefix := content[:start]
			prefix = strings.TrimRight(prefix, " \t\r\n")
			if strings.HasSuffix(prefix, "---") {
				prefix = strings.TrimSuffix(prefix, "---")
				prefix = strings.TrimRight(prefix, " \t\r\n")
			}
			suffix := strings.TrimLeft(content[end:], "\n")
			var b strings.Builder
			if prefix != "" {
				b.WriteString(prefix)
				b.WriteByte('\n')
			}
			b.WriteString(block)
			if suffix != "" {
				b.WriteString("\n")
				b.WriteString(suffix)
			}
			return b.String()
		}
	}
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n"
	}
	return content + block
}
