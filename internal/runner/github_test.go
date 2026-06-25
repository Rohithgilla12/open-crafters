package runner

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyGitHubSignature(t *testing.T) {
	secret := "hunter2"
	body := []byte(`{"ref":"refs/heads/main"}`)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !VerifyGitHubSignature(secret, body, sig) {
		t.Fatal("expected valid signature")
	}
	if VerifyGitHubSignature(secret, body, "sha256=deadbeef") {
		t.Fatal("expected invalid signature")
	}
}

func TestFindSolutionRoot(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "user-repo-abc123")
	sol := filepath.Join(repo, "my-wal")
	if err := os.MkdirAll(filepath.Join(sol, ".open-crafters"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sol, ".open-crafters", "challenge"), []byte("build-your-own-wal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sol, "your_program.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindSolutionRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if got != sol {
		t.Fatalf("got %q want %q", got, sol)
	}
}

func TestAllowsBranch(t *testing.T) {
	cfg := Config{GitHubBranches: "default"}
	if !cfg.AllowsBranch("main", "main") {
		t.Fatal("expected default branch")
	}
	if cfg.AllowsBranch("dev", "main") {
		t.Fatal("expected dev ignored")
	}
	cfg.GitHubBranches = "*"
	if !cfg.AllowsBranch("dev", "main") {
		t.Fatal("expected any branch")
	}
}
