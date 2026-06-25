package runner

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds runner service settings (flags or environment variables).
type Config struct {
	Listen        string
	Token         string
	GradeImage    string
	MaxConcurrent int
	JobTimeout    time.Duration
	WorkDir       string
	MaxZipBytes   int64
}

// ConfigFromEnv loads configuration from environment variables.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Listen:        envOr("RUNNER_LISTEN", ":8080"),
		Token:         os.Getenv("RUNNER_TOKEN"),
		GradeImage:    envOr("RUNNER_GRADE_IMAGE", "open-crafters-grade:latest"),
		MaxConcurrent: envIntOr("RUNNER_MAX_CONCURRENT", 2),
		JobTimeout:    envDurationOr("RUNNER_JOB_TIMEOUT", 15*time.Minute),
		WorkDir:       envOr("RUNNER_WORK_DIR", "/var/lib/open-crafters/jobs"),
		MaxZipBytes:   envInt64Or("RUNNER_MAX_ZIP_BYTES", 10<<20), // 10 MiB
	}
	if cfg.Token == "" {
		return cfg, fmt.Errorf("RUNNER_TOKEN is required")
	}
	if cfg.MaxConcurrent < 1 {
		return cfg, fmt.Errorf("RUNNER_MAX_CONCURRENT must be >= 1")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envInt64Or(key string, fallback int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
