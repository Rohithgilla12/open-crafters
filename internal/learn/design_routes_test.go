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

	for _, path := range []string{"/design", "/design/design-chat-at-scale"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "System design") && !strings.Contains(body, "chat at scale") {
			t.Fatalf("GET %s body missing expected content", path)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/design/no-such-problem", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET missing design = %d, want 404", rec.Code)
	}
}
