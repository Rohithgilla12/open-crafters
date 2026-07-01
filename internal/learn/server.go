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
	cfg     Config
}

// NewServer returns an HTTP handler front door for the learn app.
func NewServer(catalog *Catalog, cfg Config) *Server {
	return &Server{
		catalog: catalog,
		assets:  opencrafters.AssetsFS(),
		cfg:     cfg,
	}
}

// Handler returns the root http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /api/challenges", s.handleAPIChallenges)
	mux.HandleFunc("GET /api/paths", s.handleAPIPaths)
	mux.HandleFunc("POST /api/submit", s.handleSubmit)
	mux.HandleFunc("GET /api/jobs/{id}", s.handleSubmitJob)
	mux.HandleFunc("GET /style.css", s.handleCSS)
	mux.HandleFunc("GET /learn.js", s.handleLearnJS)
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /paths/{slug}", s.handlePath)
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

func (s *Server) handleAPIPaths(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"paths": s.catalog.APIPaths()})
}

func (s *Server) handleCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(siteCSS))
}

func (s *Server) handleLearnJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(learnJS))
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	s.render(w, indexTmpl, s.catalog.Paths)
}

func (s *Server) handlePath(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	p, ok := s.catalog.GetPath(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, pathTmpl, p)
}

func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ch, ok := s.catalog.Get(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, challengeTmpl, challengePageData{
		Challenge:  ch,
		StageSlugs: stageSlugs(ch),
		PathSlug:   s.catalog.PathForChallenge(slug),
		PathName:   pathName(s.catalog, slug),
	})
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
		Challenge:  ch,
		Stage:      stage,
		StageSlugs: stageSlugs(ch),
		Prev:       neighborStage(ch, stage.Num-2),
		Next:       neighborStage(ch, stage.Num),
	}
	s.render(w, stageTmpl, data)
}

type stageNavLink struct {
	Slug string
	Name string
}

type stagePageData struct {
	Challenge  *Challenge
	Stage      *Stage
	StageSlugs string
	Prev       *stageNavLink
	Next       *stageNavLink
}

type challengePageData struct {
	Challenge  *Challenge
	StageSlugs string
	PathSlug   string
	PathName   string
}

func stageSlugs(ch *Challenge) string {
	var parts []string
	for _, st := range ch.Stages {
		parts = append(parts, st.Slug)
	}
	return strings.Join(parts, ",")
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

func pathName(c *Catalog, challengeSlug string) string {
	if p, ok := c.GetPath(c.PathForChallenge(challengeSlug)); ok {
		return p.Name
	}
	return ""
}
