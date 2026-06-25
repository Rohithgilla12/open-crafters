package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func (s *Service) runJob(job *Job, zipBytes []byte) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	s.store.Update(job.ID, func(j *Job) {
		j.Status = StatusRunning
		j.StartedAt = nowUTC()
	})

	workDir, err := os.MkdirTemp(s.cfg.WorkDir, "job-"+job.ID+"-*")
	if err != nil {
		s.finishJob(job.ID, StatusError, 1, "", fmt.Errorf("creating work dir: %w", err))
		return
	}
	defer os.RemoveAll(workDir)

	extractDir := filepath.Join(workDir, "src")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		s.finishJob(job.ID, StatusError, 1, "", err)
		return
	}
	programPath, err := ExtractZip(bytesReaderAt(zipBytes), int64(len(zipBytes)), extractDir)
	if err != nil {
		s.finishJob(job.ID, StatusError, 1, "", err)
		return
	}

	logText, exitCode, runErr := s.executor.Run(context.Background(), job.ID, GradeRequest{
		Challenge:   job.Challenge,
		Stage:       job.Stage,
		All:         job.All,
		WorkDir:     extractDir,
		ProgramPath: programPath,
	})

	status := StatusPassed
	errMsg := ""
	if runErr != nil {
		status = StatusError
		errMsg = runErr.Error()
	} else if exitCode != 0 {
		status = StatusFailed
	}

	s.finishJob(job.ID, status, exitCode, logText, errMsg)
}

func (s *Service) finishJob(id string, status Status, exitCode int, logText string, errMsg any) {
	var msg string
	switch v := errMsg.(type) {
	case nil:
	case string:
		msg = v
	case error:
		if v != nil {
			msg = v.Error()
		}
	}

	var finished Job
	s.store.Update(id, func(j *Job) {
		j.Status = status
		j.ExitCode = exitCode
		j.Log = logText
		j.Error = msg
		j.FinishedAt = nowUTC()
		finished = *j
	})

	if finished.GitHub != nil && s.github != nil {
		s.github.finishCheck(&finished, status, logText, msg)
	}
}

func nowUTC() time.Time {
	return time.Now().UTC()
}
