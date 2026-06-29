package learn

import (
	"os"
	"strconv"
)

// Config configures the learn HTTP server.
type Config struct {
	// RunnerURL is the hosted grader API (browser submits proxy here).
	RunnerURL string
	// MaxZipBytes caps upload size for browser submit.
	MaxZipBytes int64
}

// ConfigFromEnv reads learn server settings from the environment.
func ConfigFromEnv() Config {
	url := os.Getenv("LEARN_RUNNER_URL")
	if url == "" {
		url = "https://runner.gilla.fun"
	}
	max := int64(10 << 20) // 10 MiB
	if v := os.Getenv("LEARN_MAX_ZIP_BYTES"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			max = n
		}
	}
	return Config{RunnerURL: url, MaxZipBytes: max}
}
