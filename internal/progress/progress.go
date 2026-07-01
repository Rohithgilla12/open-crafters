// Package progress persists which stages a user has passed, per challenge.
// The file lives at <submission dir>/.open-crafters/progress.json so it
// travels with the user's solution repo. The same JSON shape is used by the
// learn app (browser localStorage) for import/export sync.
package progress

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// FormatVersion is the canonical progress.json schema version.
const FormatVersion = 1

type File struct {
	Version    int                          `json:"version,omitempty"`
	Challenges map[string]*ChallengeProgress `json:"challenges"`
}

type ChallengeProgress struct {
	// Passed maps stage slug -> RFC 3339 timestamp of the first pass.
	Passed map[string]string `json:"passed,omitempty"`
	// Read maps stage slug -> RFC 3339 timestamp of first visit (learn app).
	Read map[string]string `json:"read,omitempty"`
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
		return Normalize(f), nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, f); err != nil {
		return nil, err
	}
	return Normalize(f), nil
}

// Normalize ensures version and non-nil maps on a progress file.
func Normalize(f *File) *File {
	if f == nil {
		f = &File{}
	}
	if f.Version == 0 {
		f.Version = FormatVersion
	}
	if f.Challenges == nil {
		f.Challenges = map[string]*ChallengeProgress{}
	}
	for slug, cp := range f.Challenges {
		f.Challenges[slug] = normalizeChallenge(cp)
	}
	return f
}

func normalizeChallenge(cp *ChallengeProgress) *ChallengeProgress {
	if cp == nil {
		return &ChallengeProgress{Passed: map[string]string{}, Read: map[string]string{}}
	}
	if cp.Passed == nil {
		cp.Passed = map[string]string{}
	}
	if cp.Read == nil {
		cp.Read = map[string]string{}
	}
	return cp
}

func (f *File) challenge(slug string) *ChallengeProgress {
	cp := f.Challenges[slug]
	if cp == nil {
		cp = &ChallengeProgress{Passed: map[string]string{}, Read: map[string]string{}}
		f.Challenges[slug] = cp
	}
	return normalizeChallenge(cp)
}

// HasPassed reports whether a stage was ever passed.
func (f *File) HasPassed(challengeSlug, stageSlug string) bool {
	cp := f.Challenges[challengeSlug]
	return cp != nil && cp.Passed[stageSlug] != ""
}

// MarkPassed records a stage pass (keeping the original timestamp on re-runs).
func (f *File) MarkPassed(challengeSlug, stageSlug string) {
	cp := f.challenge(challengeSlug)
	if cp.Passed[stageSlug] == "" {
		cp.Passed[stageSlug] = time.Now().UTC().Format(time.RFC3339)
	}
}

// Merge folds src into dst, keeping the earliest timestamp per stage.
func Merge(dst, src *File) {
	if dst == nil || src == nil {
		return
	}
	Normalize(dst)
	Normalize(src)
	for slug, scp := range src.Challenges {
		dcp := dst.challenge(slug)
		mergeTimestamps(dcp.Passed, scp.Passed)
		mergeTimestamps(dcp.Read, scp.Read)
	}
}

func mergeTimestamps(dst, src map[string]string) {
	for k, v := range src {
		if v == "" {
			continue
		}
		if dst[k] == "" || v < dst[k] {
			dst[k] = v
		}
	}
}

// Save writes the progress file, creating its directory if needed.
func Save(path string, f *File) error {
	f = Normalize(f)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// MarshalJSON returns the canonical export bytes for a progress file.
func MarshalJSON(f *File) ([]byte, error) {
	return json.MarshalIndent(Normalize(f), "", "  ")
}
