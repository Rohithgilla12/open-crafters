package learn

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSubmitProxy(t *testing.T) {
	runner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/grade" && r.Method == http.MethodPost {
			if got := r.Header.Get("Authorization"); got != "Bearer secret-token" {
				t.Fatalf("authorization = %q", got)
			}
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatal(err)
			}
			if r.FormValue("challenge") != "build-your-own-wal" {
				t.Fatalf("challenge = %q", r.FormValue("challenge"))
			}
			f, _, err := r.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			b, _ := io.ReadAll(f)
			if string(b) != "zipbytes" {
				t.Fatalf("file = %q", b)
			}
			writeJSON(w, http.StatusAccepted, map[string]any{"id": "abc123", "status": "queued"})
			return
		}
		if r.URL.Path == "/v1/jobs/abc123" {
			writeJSON(w, http.StatusOK, map[string]any{"id": "abc123", "status": "passed", "log": "ok"})
			return
		}
		http.NotFound(w, r)
	}))
	defer runner.Close()

	catalog, err := NewCatalog()
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(catalog, Config{RunnerURL: runner.URL, MaxZipBytes: 1 << 20})
	mux := srv.Handler()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("challenge", "build-your-own-wal")
	fw, _ := mw.CreateFormFile("file", "solution.zip")
	_, _ = fw.Write([]byte("zipbytes"))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/submit", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("submit status = %d body %s", rec.Code, rec.Body.String())
	}
	var job map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job["id"] != "abc123" {
		t.Fatalf("job id = %v", job["id"])
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/jobs/abc123", nil)
	req2.Header.Set("Authorization", "Bearer secret-token")
	req2.SetPathValue("id", "abc123")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("get job status = %d body %s", rec2.Code, rec2.Body.String())
	}
}

func TestLearnJS(t *testing.T) {
	if len(learnJS) < 100 {
		t.Fatal("learn.js not embedded")
	}
}
