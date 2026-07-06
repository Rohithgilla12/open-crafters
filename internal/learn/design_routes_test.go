package learn

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDesignRoutes(t *testing.T) {
	catalog, err := NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(catalog, Config{})
	mux := srv.Handler()

	for _, path := range []string{
		"/design",
		"/design/design-chat-at-scale",
		"/design/roadmaps",
		"/design/roadmaps/interview-classics",
		"/design/stacks",
		"/design/stacks/url-shortener",
		"/challenges/build-your-own-id-generator",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		body := rec.Body.String()
		switch path {
		case "/design/stacks/url-shortener":
			if !strings.Contains(body, "URL shortener stack") {
				t.Fatalf("GET %s body missing stack title", path)
			}
		case "/challenges/build-your-own-id-generator":
			if !strings.Contains(body, "Whiteboard first") {
				t.Fatalf("GET %s body missing design bridge", path)
			}
		default:
			if !strings.Contains(body, "System design") && !strings.Contains(body, "chat at scale") && !strings.Contains(body, "Design stacks") {
				t.Fatalf("GET %s body missing expected content", path)
			}
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/design/no-such-problem", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET missing design = %d, want 404", rec.Code)
	}
}
