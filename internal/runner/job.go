package runner

import (
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
	ID        string    `json:"id"`
	Status    Status    `json:"status"`
	Challenge string    `json:"challenge"`
	Stage     string    `json:"stage,omitempty"`
	All       bool      `json:"all"`
	ExitCode  int       `json:"exit_code,omitempty"`
	Log       string    `json:"log"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	StartedAt time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
}

// Store tracks in-flight and recent jobs.
type Store struct {
	mu   sync.RWMutex
	jobs map[string]*Job
}

func NewStore() *Store {
	return &Store{jobs: map[string]*Job{}}
}

func (s *Store) Put(job *Job) {
	s.mu.Lock()
	s.jobs[job.ID] = job
	s.mu.Unlock()
}

func (s *Store) Get(id string) (*Job, bool) {
	s.mu.RLock()
	job, ok := s.jobs[id]
	s.mu.RUnlock()
	return job, ok
}

func (s *Store) Update(id string, fn func(*Job)) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return false
	}
	fn(job)
	return true
}
