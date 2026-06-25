package runner_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rohithgilla12/open-crafters/internal/runner"
)

func TestHealthNoAuth(t *testing.T) {
	svc := runner.NewService(runner.Config{Token: "secret", WorkDir: t.TempDir()})
	srv := runner.NewServer(svc)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestGetJobRequiresAuth(t *testing.T) {
	svc := runner.NewService(runner.Config{Token: "secret", WorkDir: t.TempDir()})
	srv := runner.NewServer(svc)

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/abc", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}
