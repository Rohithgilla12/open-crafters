package runner

import (
	"io"
	"net/http"
	"strings"
)

func (s *Server) handleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	if s.svc.cfg.GitHubWebhookSecret == "" {
		http.Error(w, "github webhooks not configured", http.StatusNotImplemented)
		return
	}
	if s.svc.github == nil {
		http.Error(w, "GITHUB_TOKEN not configured", http.StatusServiceUnavailable)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 5<<20))
	if err != nil {
		http.Error(w, "reading body", http.StatusBadRequest)
		return
	}
	sig := r.Header.Get("X-Hub-Signature-256")
	if !VerifyGitHubSignature(s.svc.cfg.GitHubWebhookSecret, body, sig) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "event " + event})
		return
	}

	ev, err := parsePushEvent(body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ev.Deleted || ev.After == strings.Repeat("0", 40) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "branch deleted"})
		return
	}

	branch := branchFromRef(ev.Ref)
	if !s.svc.cfg.allowsBranch(branch, ev.Repository.DefaultBranch) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "branch " + branch})
		return
	}

	owner, repo, err := splitRepo(ev.Repository.FullName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	challenge, zipBytes, err := s.svc.github.downloadAndPrepare(owner, repo, ev.After)
	if err != nil {
		if strings.Contains(err.Error(), "no open-crafters solution") {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": err.Error()})
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	checkID, err := s.svc.github.createCheck(owner, repo, ev.After)
	if err != nil {
		http.Error(w, "creating check run: "+err.Error(), http.StatusBadGateway)
		return
	}

	meta := &GitHubMeta{
		Owner:      owner,
		Repo:       repo,
		SHA:        ev.After,
		Ref:        ev.Ref,
		CheckRunID: checkID,
	}
	job, err := s.svc.EnqueueGitHub(challenge, zipBytes, meta)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.log.Printf("github push %s@%s job %s challenge=%s", ev.Repository.FullName, ev.After[:7], job.ID, challenge)
	writeJSON(w, http.StatusAccepted, job)
}
