package learn

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSEOAndBlogRoutes(t *testing.T) {
	catalog, err := NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.BlogPosts) < 9 {
		t.Fatalf("expected >= 9 blog posts, got %d", len(catalog.BlogPosts))
	}
	mux := NewServer(catalog, Config{}).Handler()

	t.Run("robots", func(t *testing.T) {
		rec := get(t, mux, "/robots.txt")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Allow: /") {
			t.Fatalf("missing Allow: %q", body)
		}
		if !strings.Contains(body, "Sitemap: https://learn.gilla.fun/sitemap.xml") {
			t.Fatalf("missing sitemap line: %q", body)
		}
	})

	t.Run("sitemap", func(t *testing.T) {
		rec := get(t, mux, "/sitemap.xml")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{
			"https://learn.gilla.fun/",
			"https://learn.gilla.fun/blog",
			"https://learn.gilla.fun/blog/write-ahead-log-durability",
			"https://learn.gilla.fun/blog/object-store-durability",
			"https://learn.gilla.fun/challenges/build-your-own-wal",
			"https://learn.gilla.fun/roadmaps",
			"https://learn.gilla.fun/design",
		} {
			if !strings.Contains(body, "<loc>"+want+"</loc>") {
				t.Fatalf("sitemap missing %s", want)
			}
		}
	})

	t.Run("home seo", func(t *testing.T) {
		rec := get(t, mux, "/")
		body := rec.Body.String()
		if !strings.Contains(body, `rel="canonical" href="https://learn.gilla.fun/"`) {
			t.Fatal("missing canonical")
		}
		if !strings.Contains(body, `property="og:title"`) {
			t.Fatal("missing og:title")
		}
		if !strings.Contains(body, `"@type":"WebSite"`) {
			t.Fatal("missing WebSite JSON-LD")
		}
		if !strings.Contains(body, "From the blog") && !strings.Contains(body, "/blog/") {
			t.Fatal("home missing blog teaser")
		}
	})

	t.Run("challenge seo", func(t *testing.T) {
		rec := get(t, mux, "/challenges/build-your-own-wal")
		body := rec.Body.String()
		if !strings.Contains(body, `rel="canonical" href="https://learn.gilla.fun/challenges/build-your-own-wal"`) {
			t.Fatal("missing challenge canonical")
		}
		if !strings.Contains(body, `"@type":"LearningResource"`) {
			t.Fatal("missing LearningResource JSON-LD")
		}
	})

	t.Run("blog index", func(t *testing.T) {
		rec := get(t, mux, "/blog")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Infrastructure essays") {
			t.Fatal("missing blog index title")
		}
		if !strings.Contains(body, "/blog/write-ahead-log-durability") {
			t.Fatal("missing WAL post link")
		}
		if !strings.Contains(body, "Blog") {
			t.Fatal("nav should include Blog")
		}
	})

	t.Run("blog post", func(t *testing.T) {
		rec := get(t, mux, "/blog/write-ahead-log-durability")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "write-ahead log") && !strings.Contains(body, "write-ahead") {
			t.Fatal("missing post body")
		}
		if !strings.Contains(body, `"@type":"Article"`) {
			t.Fatal("missing Article JSON-LD")
		}
		if !strings.Contains(body, "/challenges/build-your-own-wal") {
			t.Fatal("missing related challenge CTA")
		}
	})

	t.Run("blog 404", func(t *testing.T) {
		rec := get(t, mux, "/blog/no-such-post")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status %d, want 404", rec.Code)
		}
	})

	t.Run("og png", func(t *testing.T) {
		rec := get(t, mux, "/og.png")
		if rec.Code != http.StatusOK {
			t.Fatalf("status %d", rec.Code)
		}
	})
}

func get(t *testing.T, mux http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}
