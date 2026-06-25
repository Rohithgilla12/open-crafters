package learn

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

// Server serves the learner web app.
type Server struct {
	catalog *Catalog
	assets  fs.FS
}

// NewServer returns an HTTP handler front door for the learn app.
func NewServer(catalog *Catalog) *Server {
	return &Server{
		catalog: catalog,
		assets:  opencrafters.AssetsFS(),
	}
}

// Handler returns the root http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/challenges", s.handleAPIChallenges)
	mux.HandleFunc("GET /style.css", s.handleCSS)
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /challenges/{slug}", s.handleChallenge)
	mux.HandleFunc("GET /challenges/{slug}/stages/{stage}", s.handleStage)
	mux.Handle("GET /favicon.svg", http.FileServer(http.FS(s.assets)))
	mux.Handle("GET /apple-touch-icon.png", http.FileServer(http.FS(s.assets)))
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleAPIChallenges(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"challenges": s.catalog.APIList()})
}

func (s *Server) handleCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(siteCSS))
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	challenges := make([]*Challenge, 0, len(s.catalog.Order))
	for _, slug := range s.catalog.Order {
		challenges = append(challenges, s.catalog.Challenges[slug])
	}
	s.render(w, indexTmpl, challenges)
}

func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ch, ok := s.catalog.Get(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, challengeTmpl, ch)
}

func (s *Server) handleStage(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	stageSlug := r.PathValue("stage")
	ch, stage, ok := s.catalog.Stage(slug, stageSlug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	data := stagePageData{
		Challenge: ch,
		Stage:     stage,
		Prev:      neighborStage(ch, stage.Num-2),
		Next:      neighborStage(ch, stage.Num),
	}
	s.render(w, stageTmpl, data)
}

type stageNavLink struct {
	Slug string
	Name string
}

type stagePageData struct {
	Challenge *Challenge
	Stage     *Stage
	Prev      *stageNavLink
	Next      *stageNavLink
}

func neighborStage(ch *Challenge, idx int) *stageNavLink {
	if idx < 0 || idx >= len(ch.Stages) {
		return nil
	}
	s := ch.Stages[idx]
	return &stageNavLink{Slug: s.Slug, Name: s.Name}
}

func (s *Server) render(w http.ResponseWriter, tmpl *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// shortSlug strips the build-your-own- prefix for display.
func shortSlug(slug string) string {
	return strings.TrimPrefix(slug, "build-your-own-")
}
