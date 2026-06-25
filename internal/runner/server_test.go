package runner_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rohithgilla12/open-crafters/internal/runner"
)

func testService(t *testing.T) *runner.Service {
	t.Helper()
	svc, err := runner.NewService(runner.Config{Token: "secret", WorkDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestHealthNoAuth(t *testing.T) {
	srv := runner.NewServer(testService(t))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestGetJobRequiresAuth(t *testing.T) {
	srv := runner.NewServer(testService(t))

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs/abc", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestListJobsRequiresAuth(t *testing.T) {
	srv := runner.NewServer(testService(t))

	req := httptest.NewRequest(http.MethodGet, "/v1/jobs", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestJobPersistence(t *testing.T) {
	dir := t.TempDir()
	svc, err := runner.NewService(runner.Config{Token: "secret", WorkDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	job, err := svc.Enqueue("build-your-own-wal", "bind", false, []byte("not a zip"))
	if err != nil {
		t.Fatal(err)
	}
	// wait briefly for async error path
	_ = job

	svc2, err := runner.NewService(runner.Config{Token: "secret", WorkDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	jobs := svc2.Store().ListRecent(10)
	if len(jobs) == 0 {
		t.Fatal("expected persisted job")
	}
}
