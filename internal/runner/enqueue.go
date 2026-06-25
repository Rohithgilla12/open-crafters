package runner

import (
	"strings"
	"time"
)

func (cfg Config) AllowsBranch(branch, defaultBranch string) bool {
	return cfg.allowsBranch(branch, defaultBranch)
}

func (cfg Config) allowsBranch(branch, defaultBranch string) bool {
	switch cfg.GitHubBranches {
	case "", "default":
		return branch == defaultBranch
	case "*":
		return true
	}
	for _, b := range strings.Split(cfg.GitHubBranches, ",") {
		if strings.TrimSpace(b) == branch {
			return true
		}
	}
	return false
}

// Enqueue stores a job from an uploaded zip and starts processing.
func (s *Service) Enqueue(challenge, stage string, all bool, zipBytes []byte) (*Job, error) {
	return s.enqueue(challenge, stage, all, zipBytes, "upload", nil)
}

func (s *Service) enqueue(challenge, stage string, all bool, zipBytes []byte, source string, gh *GitHubMeta) (*Job, error) {
	id, err := newJobID()
	if err != nil {
		return nil, err
	}
	job := &Job{
		ID:        id,
		Status:    StatusQueued,
		Source:    source,
		Challenge: challenge,
		Stage:     stage,
		All:       all,
		GitHub:    gh,
		CreatedAt: time.Now().UTC(),
	}
	s.store.Put(job)
	go s.runJob(job, zipBytes)
	return job, nil
}

func (s *Service) finishError(id string, err error) {
	s.finishJob(id, StatusError, 1, "", err)
}
