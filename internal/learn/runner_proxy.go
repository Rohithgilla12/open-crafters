package learn

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// handleSubmit proxies a solution zip to the hosted runner (avoids browser CORS).
func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := bearerToken(r)
	if token == "" {
		http.Error(w, "Authorization: Bearer <runner-token> required", http.StatusUnauthorized)
		return
	}
	if err := r.ParseMultipartForm(s.cfg.MaxZipBytes); err != nil {
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
		http.Error(w, "file is required (zip your solution directory)", http.StatusBadRequest)
		return
	}
	defer file.Close()
	if header.Size > s.cfg.MaxZipBytes {
		http.Error(w, fmt.Sprintf("zip too large (max %d bytes)", s.cfg.MaxZipBytes), http.StatusRequestEntityTooLarge)
		return
	}
	zipBytes, err := io.ReadAll(io.LimitReader(file, s.cfg.MaxZipBytes+1))
	if err != nil {
		http.Error(w, "reading upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	if int64(len(zipBytes)) > s.cfg.MaxZipBytes {
		http.Error(w, fmt.Sprintf("zip too large (max %d bytes)", s.cfg.MaxZipBytes), http.StatusRequestEntityTooLarge)
		return
	}

	job, status, err := s.proxyGrade(token, challenge, stage, all, zipBytes)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, status, job)
}

func (s *Server) handleSubmitJob(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "job id required", http.StatusBadRequest)
		return
	}
	body, status, err := s.proxyGetJob(token, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func (s *Server) proxyGrade(token, challenge, stage string, all bool, zipBytes []byte) (map[string]any, int, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("challenge", challenge)
	if stage != "" {
		_ = mw.WriteField("stage", stage)
	}
	if all {
		_ = mw.WriteField("all", "true")
	}
	fw, err := mw.CreateFormFile("file", "solution.zip")
	if err != nil {
		return nil, 0, err
	}
	if _, err := fw.Write(zipBytes); err != nil {
		return nil, 0, err
	}
	if err := mw.Close(); err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(s.cfg.RunnerURL, "/")+"/v1/grade", &buf)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("runner request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("runner: %s", strings.TrimSpace(string(data)))
	}
	var job map[string]any
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decoding runner response: %w", err)
	}
	return job, resp.StatusCode, nil
}

func (s *Server) proxyGetJob(token, id string) ([]byte, int, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(s.cfg.RunnerURL, "/")+"/v1/jobs/"+id, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("runner request: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, 0, err
	}
	return data, resp.StatusCode, nil
}

func bearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return strings.TrimSpace(r.Header.Get("X-Crafters-Token"))
}
