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
		"/",
		"/design",
		"/design/design-chat-at-scale",
		"/design/roadmaps",
		"/design/roadmaps/interview-classics",
		"/design/stacks",
		"/design/stacks/url-shortener",
		"/design/stacks/distributed-cache",
		"/challenges/build-your-own-id-generator",
		"/challenges/build-your-own-url-shortener",
		"/challenges/build-your-own-workflow-worker",
		"/roadmaps/integration",
		"/roadmaps/workflow",
		"/design/stacks/workflow-platform",
		"/design/design-workflow-platform",
	} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		body := rec.Body.String()
		switch path {
		case "/":
			if !strings.Contains(body, "Build locally") || !strings.Contains(body, "Browse journeys") {
				t.Fatalf("GET %s body missing dual CTAs", path)
			}
			if !strings.Contains(body, "Wire graded primitives into real systems") {
				t.Fatalf("GET %s body missing compose promo", path)
			}
			if !strings.Contains(body, "Start here") {
				t.Fatalf("GET %s body missing start-here section", path)
			}
			if strings.Contains(body, "Compose capstones") && strings.Contains(body, "Whiteboard mode") && strings.Contains(body, "Remote grading") {
				t.Fatalf("GET %s still has old four-card hero promo stack", path)
			}
		case "/roadmaps/integration":
			if !strings.Contains(body, "Compose &amp; meta") {
				t.Fatalf("GET %s body missing integration roadmap title", path)
			}
		case "/design/stacks/url-shortener":
			if !strings.Contains(body, "URL shortener stack") {
				t.Fatalf("GET %s body missing stack title", path)
			}
			if !strings.Contains(body, "Open compose challenge") {
				t.Fatalf("GET %s body missing compose capstone CTA", path)
			}
		case "/design/stacks/distributed-cache":
			if !strings.Contains(body, "Compose capstone") {
				t.Fatalf("GET %s body missing compose capstone label", path)
			}
		case "/challenges/build-your-own-url-shortener":
			if !strings.Contains(body, "Compose capstone") {
				t.Fatalf("GET %s body missing compose callout", path)
			}
		case "/challenges/build-your-own-workflow-worker":
			if !strings.Contains(body, "Compose capstone") {
				t.Fatalf("GET %s body missing compose callout", path)
			}
		case "/roadmaps/workflow":
			if !strings.Contains(body, "Workflow engines") {
				t.Fatalf("GET %s body missing workflow roadmap title", path)
			}
			if !strings.Contains(body, "compose") {
				t.Fatalf("GET %s body missing compose badge on workflow worker", path)
			}
		case "/design/stacks/workflow-platform":
			if !strings.Contains(body, "Workflow platform stack") {
				t.Fatalf("GET %s body missing stack title", path)
			}
			if !strings.Contains(body, "Open compose challenge") {
				t.Fatalf("GET %s body missing compose capstone CTA", path)
			}
		case "/design/design-workflow-platform":
			if !strings.Contains(body, "workflow-worker") && !strings.Contains(body, "workflow worker") {
				t.Fatalf("GET %s body missing workflow worker build step", path)
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
