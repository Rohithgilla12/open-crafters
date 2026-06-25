package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const maxHistoryJobs = 200

func (s *Store) loadHistory(dir string) error {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var job Job
		if err := json.Unmarshal(data, &job); err != nil {
			continue
		}
		if job.ID == "" {
			continue
		}
		s.jobs[job.ID] = &job
	}
	return nil
}

func (s *Store) saveJob(dir string, job *Job) error {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(job, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, job.ID+".json")
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return pruneHistory(dir, maxHistoryJobs)
}

func pruneHistory(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	type item struct {
		name string
		mod  int64
	}
	var files []item
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, item{e.Name(), info.ModTime().UnixNano()})
	}
	if len(files) <= keep {
		return nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	for _, f := range files[keep:] {
		_ = os.Remove(filepath.Join(dir, f.name))
	}
	return nil
}

func historyDir(cfg Config) string {
	if cfg.JobHistoryDir != "" {
		return cfg.JobHistoryDir
	}
	return filepath.Join(cfg.WorkDir, "history")
}

// JobSummary is a job without the full grading log (for list endpoints).
type JobSummary struct {
	ID         string `json:"id"`
	Status     Status `json:"status"`
	Source     string `json:"source,omitempty"`
	Challenge  string `json:"challenge"`
	Stage      string `json:"stage,omitempty"`
	All        bool   `json:"all"`
	ExitCode   int    `json:"exit_code,omitempty"`
	Error      string `json:"error,omitempty"`
	GitHubRepo string `json:"github_repo,omitempty"`
	CreatedAt  string `json:"created_at"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

func summarizeJob(j *Job) JobSummary {
	sum := JobSummary{
		ID:         j.ID,
		Status:     j.Status,
		Source:     j.Source,
		Challenge:  j.Challenge,
		Stage:      j.Stage,
		All:        j.All,
		ExitCode:   j.ExitCode,
		Error:      j.Error,
		CreatedAt:  formatTime(j.CreatedAt),
		StartedAt:  formatTime(j.StartedAt),
		FinishedAt: formatTime(j.FinishedAt),
	}
	if j.GitHub != nil {
		sum.GitHubRepo = j.GitHub.Owner + "/" + j.GitHub.Repo
	}
	return sum
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
