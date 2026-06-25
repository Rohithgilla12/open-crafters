package runner

import (
	"sort"
	"sync"
	"time"
)

// Status is the lifecycle state of a grading job.
type Status string

const (
	StatusQueued  Status = "queued"
	StatusRunning Status = "running"
	StatusPassed  Status = "passed"
	StatusFailed  Status = "failed"
	StatusError   Status = "error"
)

// Job is a single remote grading run.
type Job struct {
	ID         string       `json:"id"`
	Status     Status       `json:"status"`
	Source     string       `json:"source,omitempty"`
	Challenge  string       `json:"challenge"`
	Stage      string       `json:"stage,omitempty"`
	All        bool         `json:"all"`
	ExitCode   int          `json:"exit_code,omitempty"`
	Log        string       `json:"log"`
	Error      string       `json:"error,omitempty"`
	GitHub     *GitHubMeta  `json:"github,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	StartedAt  time.Time    `json:"started_at,omitempty"`
	FinishedAt time.Time    `json:"finished_at,omitempty"`
}

// GitHubMeta links a job to a repository push and check run.
type GitHubMeta struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	SHA        string `json:"sha"`
	Ref        string `json:"ref"`
	CheckRunID int64  `json:"check_run_id,omitempty"`
}

// Store tracks in-flight and recent jobs.
type Store struct {
	mu         sync.RWMutex
	jobs       map[string]*Job
	historyDir string
}

func NewStore(historyDir string) (*Store, error) {
	s := &Store{jobs: map[string]*Job{}, historyDir: historyDir}
	if err := s.loadHistory(historyDir); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Put(job *Job) {
	s.mu.Lock()
	cp := *job
	s.jobs[job.ID] = &cp
	dir := s.historyDir
	s.mu.Unlock()
	_ = s.saveJob(dir, &cp)
}

func (s *Store) Get(id string) (*Job, bool) {
	s.mu.RLock()
	job, ok := s.jobs[id]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	cp := *job
	return &cp, true
}

func (s *Store) Update(id string, fn func(*Job)) bool {
	s.mu.Lock()
	job, ok := s.jobs[id]
	if !ok {
		s.mu.Unlock()
		return false
	}
	fn(job)
	cp := *job
	dir := s.historyDir
	s.mu.Unlock()
	_ = s.saveJob(dir, &cp)
	return true
}

// ListRecent returns up to limit jobs newest-first.
func (s *Store) ListRecent(limit int) []JobSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 {
		limit = 50
	}
	all := make([]*Job, 0, len(s.jobs))
	for _, j := range s.jobs {
		all = append(all, j)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedAt.After(all[j].CreatedAt)
	})
	if len(all) > limit {
		all = all[:limit]
	}
	out := make([]JobSummary, len(all))
	for i, j := range all {
		out[i] = summarizeJob(j)
	}
	return out
}
