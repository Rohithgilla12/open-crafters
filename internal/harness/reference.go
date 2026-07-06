package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ReferenceProgramPath returns the path to a reference solution entry point.
// Search order: OPEN_CRAFTERS_ROOT, cwd walk-up (go.mod), executable walk-up.
func ReferenceProgramPath(challengeSlug, lang string) (string, error) {
	rel := filepath.Join("examples", "solutions", challengeSlug, lang, "your_program.sh")
	if root, ok := findRepoRoot(); ok {
		p := filepath.Join(root, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("reference solution not found for %s (%s): set OPEN_CRAFTERS_ROOT or run from the repo", challengeSlug, lang)
}

// RepoRoot locates the open-crafters module root when developing locally.
func RepoRoot() (string, bool) {
	return findRepoRoot()
}

func findRepoRoot() (string, bool) {
	if r := strings.TrimSpace(os.Getenv("OPEN_CRAFTERS_ROOT")); r != "" {
		if _, err := os.Stat(filepath.Join(r, "go.mod")); err == nil {
			return r, true
		}
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		if root := walkToModuleRoot(filepath.Dir(file)); root != "" {
			return root, true
		}
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	root := walkToModuleRoot(wd)
	return root, root != ""
}

func walkToModuleRoot(start string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if _, err := os.Stat(filepath.Join(dir, "examples", "solutions")); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
