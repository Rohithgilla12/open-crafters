// Package progress persists which stages a user has passed, per challenge.
// The file lives at <submission dir>/.open-crafters/progress.json so it
// travels with the user's solution repo.
package progress

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type File struct {
	Challenges map[string]*ChallengeProgress `json:"challenges"`
}

type ChallengeProgress struct {
	// Passed maps stage slug -> RFC 3339 timestamp of the first pass.
	Passed map[string]string `json:"passed"`
}

// PathFor returns the progress file path for a submission entry point.
func PathFor(programPath string) string {
	return filepath.Join(filepath.Dir(programPath), ".open-crafters", "progress.json")
}

// Load reads a progress file; a missing file is an empty progress.
func Load(path string) (*File, error) {
	f := &File{Challenges: map[string]*ChallengeProgress{}}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return f, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, f); err != nil {
		return nil, err
	}
	if f.Challenges == nil {
		f.Challenges = map[string]*ChallengeProgress{}
	}
	return f, nil
}

func (f *File) challenge(slug string) *ChallengeProgress {
	cp := f.Challenges[slug]
	if cp == nil {
		cp = &ChallengeProgress{Passed: map[string]string{}}
		f.Challenges[slug] = cp
	}
	if cp.Passed == nil {
		cp.Passed = map[string]string{}
	}
	return cp
}

// HasPassed reports whether a stage was ever passed.
func (f *File) HasPassed(challengeSlug, stageSlug string) bool {
	cp := f.Challenges[challengeSlug]
	return cp != nil && cp.Passed[stageSlug] != ""
}

// MarkPassed records a stage pass (keeping the original timestamp on
// re-runs).
func (f *File) MarkPassed(challengeSlug, stageSlug string) {
	cp := f.challenge(challengeSlug)
	if cp.Passed[stageSlug] == "" {
		cp.Passed[stageSlug] = time.Now().UTC().Format(time.RFC3339)
	}
}

// Save writes the progress file, creating its directory if needed.
func Save(path string, f *File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
