package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/Rohithgilla12/open-crafters/internal/runner"
)

func main() {
	listen := flag.String("listen", "", "listen address (default :8080 or RUNNER_LISTEN)")
	token := flag.String("token", "", "API bearer token (default RUNNER_TOKEN)")
	gradeImage := flag.String("grade-image", "", "Docker image for grading jobs (default RUNNER_GRADE_IMAGE)")
	flag.Parse()

	cfg, err := runner.ConfigFromEnv()
	if err != nil && os.Getenv("RUNNER_TOKEN") == "" && *token == "" {
		log.Fatalf("runner: %v", err)
	}
	if *listen != "" {
		cfg.Listen = *listen
	}
	if *token != "" {
		cfg.Token = *token
	}
	if *gradeImage != "" {
		cfg.GradeImage = *gradeImage
	}
	if cfg.Token == "" {
		log.Fatal("runner: --token or RUNNER_TOKEN is required")
	}
	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		log.Fatalf("runner: creating work dir: %v", err)
	}

	svc, err := runner.NewService(cfg)
	if err != nil {
		log.Fatalf("runner: %v", err)
	}
	srv := runner.NewServer(svc)
	log.Printf("listening on %s (grade image %s, max concurrent %d)", cfg.Listen, cfg.GradeImage, cfg.MaxConcurrent)
	if err := http.ListenAndServe(cfg.Listen, srv.Handler()); err != nil {
		log.Fatalf("runner: %v", err)
	}
}
