package runner

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Service accepts grading jobs and runs them in a worker pool.
type Service struct {
	cfg      Config
	store    *Store
	executor *Executor
	sem      chan struct{}
}

func NewService(cfg Config) (*Service, error) {
	store, err := NewStore(historyDir(cfg))
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:      cfg,
		store:    store,
		executor: NewExecutor(cfg),
		sem:      make(chan struct{}, cfg.MaxConcurrent),
	}, nil
}

func (s *Service) Store() *Store { return s.store }

// Enqueue stores a job and starts processing asynchronously.
func (s *Service) Enqueue(challenge, stage string, all bool, zipBytes []byte) (*Job, error) {
	id, err := newJobID()
	if err != nil {
		return nil, err
	}
	job := &Job{
		ID:        id,
		Status:    StatusQueued,
		Challenge: challenge,
		Stage:     stage,
		All:       all,
		CreatedAt: time.Now().UTC(),
	}
	s.store.Put(job)

	go s.runJob(job, zipBytes)
	return job, nil
}

func (s *Service) runJob(job *Job, zipBytes []byte) {
	s.sem <- struct{}{}
	defer func() { <-s.sem }()

	s.store.Update(job.ID, func(j *Job) {
		j.Status = StatusRunning
		j.StartedAt = time.Now().UTC()
	})

	workDir, err := os.MkdirTemp(s.cfg.WorkDir, "job-"+job.ID+"-*")
	if err != nil {
		s.finishError(job.ID, fmt.Errorf("creating work dir: %w", err))
		return
	}
	defer os.RemoveAll(workDir)

	zipPath := filepath.Join(workDir, "submission.zip")
	if err := os.WriteFile(zipPath, zipBytes, 0o600); err != nil {
		s.finishError(job.ID, fmt.Errorf("writing submission: %w", err))
		return
	}

	extractDir := filepath.Join(workDir, "src")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		s.finishError(job.ID, err)
		return
	}
	programPath, err := ExtractZip(bytesReaderAt(zipBytes), int64(len(zipBytes)), extractDir)
	if err != nil {
		s.finishError(job.ID, err)
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

	s.store.Update(job.ID, func(j *Job) {
		j.Status = status
		j.ExitCode = exitCode
		j.Log = logText
		j.Error = errMsg
		j.FinishedAt = time.Now().UTC()
	})
}

func (s *Service) finishError(id string, err error) {
	s.store.Update(id, func(j *Job) {
		j.Status = StatusError
		j.Error = err.Error()
		j.FinishedAt = time.Now().UTC()
	})
}

// Server is the HTTP front door for the runner service.
type Server struct {
	svc *Service
	log *log.Logger
}

func NewServer(svc *Service) *Server {
	return &Server{svc: svc, log: log.New(os.Stdout, "runner ", log.LstdFlags)}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.handleDashboard)
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /v1/jobs", s.handleListJobs)
	mux.HandleFunc("GET /v1/jobs/{id}", s.handleGetJob)
	mux.HandleFunc("POST /v1/grade", s.handleGrade)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"jobs": s.svc.store.ListRecent(limit),
	})
}

func (s *Server) handleGetJob(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	job, ok := s.svc.store.Get(id)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleGrade(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(s.svc.cfg.MaxZipBytes); err != nil {
		http.Error(w, "invalid multipart form: "+err.Error(), http.StatusBadRequest)
		return
	}
	challenge := strings.TrimSpace(r.FormValue("challenge"))
	if challenge == "" {
		http.Error(w, "challenge is required", http.StatusBadRequest)
		return
	}
	stage := strings.TrimSpace(r.FormValue("stage"))
	all := r.FormValue("all") == "true" || r.FormValue("all") == "1"

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file is required (zip of your solution directory)", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if header.Size > s.svc.cfg.MaxZipBytes {
		http.Error(w, fmt.Sprintf("zip too large (max %d bytes)", s.svc.cfg.MaxZipBytes), http.StatusRequestEntityTooLarge)
		return
	}
	zipBytes, err := io.ReadAll(io.LimitReader(file, s.svc.cfg.MaxZipBytes+1))
	if err != nil {
		http.Error(w, "reading upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if int64(len(zipBytes)) > s.svc.cfg.MaxZipBytes {
		http.Error(w, fmt.Sprintf("zip too large (max %d bytes)", s.svc.cfg.MaxZipBytes), http.StatusRequestEntityTooLarge)
		return
	}

	job, err := s.svc.Enqueue(challenge, stage, all, zipBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.log.Printf("queued job %s challenge=%s all=%v stage=%q", job.ID, challenge, all, stage)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) authorize(r *http.Request) bool {
	got := tokenFromRequest(r)
	want := s.svc.cfg.Token
	if got == "" || want == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func tokenFromRequest(r *http.Request) string {
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return r.Header.Get("X-Crafters-Token")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func newJobID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// bytesReaderAt adapts a byte slice to io.ReaderAt for zip.NewReader.
type bytesReaderAt []byte

func (b bytesReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
