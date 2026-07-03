package opencrafters

import (
	"io/fs"
	"testing"
)

func TestDesignProblemsEmbedded(t *testing.T) {
	dfs := DesignFS()
	for _, slug := range DesignProblemOrder {
		def, ok := DesignBySlug(slug)
		if !ok {
			t.Fatalf("DesignBySlug(%q) = false", slug)
		}
		if def.Slug != slug {
			t.Fatalf("slug mismatch: %q vs %q", def.Slug, slug)
		}
		for _, f := range []string{"PROBLEM.md", "HINTS.md", "SOLUTION.md"} {
			if _, err := fs.Stat(dfs, slug+"/"+f); err != nil {
				t.Fatalf("design/%s/%s: %v", slug, f, err)
			}
		}
	}
}
