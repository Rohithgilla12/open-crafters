package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GradeRequest is the input to a sandboxed grading run.
type GradeRequest struct {
	Challenge   string
	Stage       string
	All         bool
	WorkDir     string
	ProgramPath string
}

// Executor runs crafters grade inside an isolated Docker container.
type Executor struct {
	GradeImage string
	JobTimeout time.Duration
	DockerBin  string
}

func NewExecutor(cfg Config) *Executor {
	docker := os.Getenv("DOCKER_BIN")
	if docker == "" {
		docker = "docker"
	}
	return &Executor{
		GradeImage: cfg.GradeImage,
		JobTimeout: cfg.JobTimeout,
		DockerBin:  docker,
	}
}

// Run executes a grading job and returns combined stdout/stderr and the exit code.
func (e *Executor) Run(ctx context.Context, jobID string, req GradeRequest) (log string, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(ctx, e.JobTimeout)
	defer cancel()

	container := "open-crafters-job-" + jobID
	args := []string{
		"run",
		"--rm",
		"--name", container,
		"--network", "none",
		"--cpus", "1",
		"--memory", "768m",
		"--pids-limit", "512",
		"--tmpfs", "/tmp:exec,size=512m",
		"-v", req.WorkDir + ":/work",
		"-w", "/work",
		e.GradeImage,
		"grade",
		"--challenge", req.Challenge,
		"--program", containerProgramPath(req.ProgramPath, req.WorkDir),
	}
	if req.All {
		args = append(args, "--all")
	} else if req.Stage != "" {
		args = append(args, "--stage", req.Stage)
	}

	cmd := exec.CommandContext(ctx, e.DockerBin, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	runErr := cmd.Run()
	exitCode = 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() != nil {
			_ = e.killContainer(context.Background(), container)
			return buf.String(), 1, fmt.Errorf("grading timed out after %s", e.JobTimeout)
		} else {
			return buf.String(), 1, fmt.Errorf("docker run: %w", runErr)
		}
	}
	return buf.String(), exitCode, nil
}

func containerProgramPath(hostProgram, workDir string) string {
	rel, err := filepath.Rel(workDir, hostProgram)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "/work/your_program.sh"
	}
	return "/work/" + filepath.ToSlash(rel)
}

func (e *Executor) killContainer(ctx context.Context, name string) error {
	cmd := exec.CommandContext(ctx, e.DockerBin, "rm", "-f", name)
	return cmd.Run()
}
