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
	mux.HandleFunc("GET /api/roadmaps", s.handleAPIRoadmaps)
	mux.HandleFunc("GET /api/design", s.handleAPIDesign)
	mux.HandleFunc("GET /api/design/roadmaps", s.handleAPIDesignRoadmaps)
	mux.HandleFunc("GET /api/blog", s.handleAPIBlog)
	mux.HandleFunc("POST /api/submit", s.handleSubmit)
	mux.HandleFunc("GET /api/jobs/{id}", s.handleSubmitJob)
	mux.HandleFunc("GET /style.css", s.handleCSS)
	mux.HandleFunc("GET /learn.js", s.handleLearnJS)
	mux.HandleFunc("GET /robots.txt", s.handleRobots)
	mux.HandleFunc("GET /sitemap.xml", s.handleSitemap)
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("GET /blog", s.handleBlogIndex)
	mux.HandleFunc("GET /blog/{slug}", s.handleBlogPost)
	mux.HandleFunc("GET /roadmaps", s.handleRoadmapsIndex)
	mux.HandleFunc("GET /roadmaps/{slug}", s.handleRoadmap)
	mux.HandleFunc("GET /design", s.handleDesignIndex)
	mux.HandleFunc("GET /design/roadmaps", s.handleDesignRoadmapsIndex)
	mux.HandleFunc("GET /design/roadmaps/{slug}", s.handleDesignRoadmap)
	mux.HandleFunc("GET /design/stacks", s.handleDesignStacksIndex)
	mux.HandleFunc("GET /design/stacks/{slug}", s.handleDesignStack)
	mux.HandleFunc("GET /design/{slug}", s.handleDesignProblem)
	mux.HandleFunc("GET /paths/{slug}", s.handlePathRedirect)
	mux.HandleFunc("GET /challenges/{slug}", s.handleChallenge)
	mux.HandleFunc("GET /challenges/{slug}/stages/{stage}", s.handleStage)
	mux.Handle("GET /favicon.svg", http.FileServer(http.FS(s.assets)))
	mux.Handle("GET /apple-touch-icon.png", http.FileServer(http.FS(s.assets)))
	mux.Handle("GET /og.png", http.FileServer(http.FS(s.assets)))
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

func (s *Server) handleAPIRoadmaps(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"roadmaps": s.catalog.APIRoadmaps()})
}

func (s *Server) handleAPIDesign(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"design": s.catalog.APIDesignList()})
}

func (s *Server) handleAPIDesignRoadmaps(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"roadmaps": s.catalog.APIDesignRoadmaps()})
}

func (s *Server) handleAPIBlog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"posts": s.catalog.APIBlogList()})
}

func (s *Server) handleCSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(learnCSS))
}

func (s *Server) handleLearnJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(learnJS))
}

type pathSummary struct {
	Slug         string
	Name         string
	Description  string
	Count        int
	Samples      []*Challenge
	HasCompose   bool
	ChallengeCSV string
	TotalStages  int
}

type indexPageData struct {
	ChallengeCount  int
	DesignCount     int
	StartHere       *RoadmapView
	Roadmaps        []RoadmapView
	FeaturedDesigns []*DesignProblem
	Paths           []pathSummary
	BlogPosts       []BlogPost
	SEO             SEO
}

var featuredDesignSlugs = []string{
	"design-distributed-scheduler",
	"design-workflow-platform",
	"design-url-shortener",
	"design-distributed-cache",
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	var startHere *RoadmapView
	for i := range s.catalog.Roadmaps {
		if s.catalog.Roadmaps[i].Slug == "durability" {
			startHere = &s.catalog.Roadmaps[i]
			break
		}
	}

	featured := make([]*DesignProblem, 0, len(featuredDesignSlugs))
	for _, slug := range featuredDesignSlugs {
		if d, ok := s.catalog.Designs[slug]; ok {
			featured = append(featured, d)
		}
	}

	paths := make([]pathSummary, 0, len(s.catalog.Paths))
	for _, p := range s.catalog.Paths {
		ps := pathSummary{
			Slug:        p.Slug,
			Name:        p.Name,
			Description: p.Description,
			Count:       len(p.Challenges),
		}
		totalStages := 0
		var ids []string
		for i, ch := range p.Challenges {
			ids = append(ids, ch.Slug)
			totalStages += len(ch.Stages)
			if ch.IsCompose {
				ps.HasCompose = true
			}
			if i < 2 {
				ps.Samples = append(ps.Samples, ch)
			}
		}
		ps.ChallengeCSV = strings.Join(ids, ",")
		ps.TotalStages = totalStages
		paths = append(paths, ps)
	}

	s.render(w, indexTmpl, indexPageData{
		ChallengeCount:  len(s.catalog.Order),
		DesignCount:     len(s.catalog.DesignOrder),
		StartHere:       startHere,
		Roadmaps:        s.catalog.Roadmaps,
		FeaturedDesigns: featured,
		Paths:           paths,
		BlogPosts:       s.catalog.BlogPosts,
		SEO: SEO{
			Title: "open-crafters — learn | build your own infrastructure",
			Description: "Build-your-own-X challenges for production infrastructure primitives. " +
				"Read stages, study the protocol, then implement and grade over the wire.",
			Path:   "/",
			Type:   "website",
			JSONLD: websiteJSONLD(),
		},
	})
}

func (s *Server) handleRoadmapsIndex(w http.ResponseWriter, _ *http.Request) {
	s.render(w, roadmapsIndexTmpl, roadmapsIndexData{
		Roadmaps: s.catalog.Roadmaps,
		SEO: SEO{
			Title:       "Learning roadmaps — open-crafters",
			Description: "Curated journeys through durability, workflows, distributed systems, coordination, and compose capstones.",
			Path:        "/roadmaps",
		},
	})
}

type roadmapsIndexData struct {
	Roadmaps []RoadmapView
	SEO      SEO
}

type roadmapPageData struct {
	*RoadmapView
	SEO SEO
}

func (s *Server) handleRoadmap(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	rm, ok := s.catalog.GetRoadmap(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, roadmapTmpl, roadmapPageData{
		RoadmapView: rm,
		SEO: SEO{
			Title:       rm.Name + " — open-crafters roadmap",
			Description: rm.Tagline,
			Path:        "/roadmaps/" + rm.Slug,
		},
	})
}

func (s *Server) handleDesignIndex(w http.ResponseWriter, _ *http.Request) {
	designs := make([]*DesignProblem, 0, len(s.catalog.DesignOrder))
	for _, slug := range s.catalog.DesignOrder {
		designs = append(designs, s.catalog.Designs[slug])
	}
	s.render(w, designIndexTmpl, designIndexData{
		Roadmaps: s.catalog.DesignRoadmaps,
		Stacks:   s.catalog.DesignStacks,
		Designs:  designs,
		SEO: SEO{
			Title: "System design — open-crafters learn",
			Description: "Whiteboard system design scenarios with discussion prompts, hints, " +
				"and reference architectures tied to build-your-own challenges.",
			Path: "/design",
		},
	})
}

type designRoadmapsIndexData struct {
	Roadmaps []DesignRoadmapView
	SEO      SEO
}

type designRoadmapPageData struct {
	*DesignRoadmapView
	SEO SEO
}

type designStacksIndexData struct {
	Stacks []DesignStackView
	SEO    SEO
}

type designStackPageData struct {
	*DesignStackView
	SEO SEO
}

func (s *Server) handleDesignRoadmapsIndex(w http.ResponseWriter, _ *http.Request) {
	s.render(w, designRoadmapsIndexTmpl, designRoadmapsIndexData{
		Roadmaps: s.catalog.DesignRoadmaps,
		SEO: SEO{
			Title:       "Design roadmaps — open-crafters learn",
			Description: "Curated whiteboard journeys — interview classics, storage, scale, distributed core, and the full curriculum.",
			Path:        "/design/roadmaps",
		},
	})
}

func (s *Server) handleDesignRoadmap(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	rm, ok := s.catalog.GetDesignRoadmap(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, designRoadmapTmpl, designRoadmapPageData{
		DesignRoadmapView: rm,
		SEO: SEO{
			Title:       rm.Name + " — design roadmap — open-crafters",
			Description: rm.Tagline,
			Path:        "/design/roadmaps/" + rm.Slug,
		},
	})
}

func (s *Server) handleDesignStacksIndex(w http.ResponseWriter, _ *http.Request) {
	s.render(w, designStacksIndexTmpl, designStacksIndexData{
		Stacks: s.catalog.DesignStacks,
		SEO: SEO{
			Title:       "Design stacks — open-crafters learn",
			Description: "Whiteboard a system, then implement the graded primitives underneath — in dependency order.",
			Path:        "/design/stacks",
		},
	})
}

func (s *Server) handleDesignStack(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	st, ok := s.catalog.GetDesignStack(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, designStackTmpl, designStackPageData{
		DesignStackView: st,
		SEO: SEO{
			Title:       st.Name + " — design stack — open-crafters",
			Description: st.Tagline,
			Path:        "/design/stacks/" + st.Slug,
		},
	})
}

type designIndexData struct {
	Roadmaps []DesignRoadmapView
	Stacks   []DesignStackView
	Designs  []*DesignProblem
	SEO      SEO
}

type designProblemPageData struct {
	*DesignProblem
	SEO SEO
}

func (s *Server) handleDesignProblem(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	d, ok := s.catalog.GetDesign(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.render(w, designProblemTmpl, designProblemPageData{
		DesignProblem: d,
		SEO: SEO{
			Title:       d.Name + " — system design — open-crafters",
			Description: d.Tagline,
			Path:        "/design/" + d.Slug,
		},
	})
}

func (s *Server) handlePathRedirect(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	http.Redirect(w, r, "/roadmaps/"+slug, http.StatusMovedPermanently)
}

func (s *Server) handleChallenge(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	ch, ok := s.catalog.Get(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	path := "/challenges/" + ch.Slug
	s.render(w, challengeTmpl, challengePageData{
		Challenge:      ch,
		StageSlugs:     stageSlugs(ch),
		RoadmapSlug:    s.catalog.RoadmapForChallenge(slug),
		RoadmapName:    roadmapName(s.catalog, slug),
		RelatedDesigns: s.catalog.DesignsForChallenge(slug),
		DesignStacks:   s.catalog.StacksForChallenge(slug),
		SEO: SEO{
			Title:       ch.Name + " — open-crafters learn",
			Description: ch.Tagline,
			Path:        path,
			JSONLD:      learningResourceJSONLD(ch.Name, ch.Tagline, path),
		},
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
	path := "/challenges/" + ch.Slug + "/stages/" + stage.Slug
	s.render(w, stageTmpl, stagePageData{
		Challenge:  ch,
		Stage:      stage,
		StageSlugs: stageSlugs(ch),
		Prev:       neighborStage(ch, stage.Num-2),
		Next:       neighborStage(ch, stage.Num),
		SEO: SEO{
			Title:       stage.Name + " — " + ch.Name + " — open-crafters learn",
			Description: stage.Name + " — stage of " + ch.Name + ". " + ch.Tagline,
			Path:        path,
		},
	})
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
	SEO        SEO
}

type challengePageData struct {
	Challenge      *Challenge
	StageSlugs     string
	RoadmapSlug    string
	RoadmapName    string
	RelatedDesigns []*DesignProblem
	DesignStacks   []DesignStackLink
	SEO            SEO
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

func roadmapName(c *Catalog, challengeSlug string) string {
	if rm, ok := c.GetRoadmap(c.RoadmapForChallenge(challengeSlug)); ok {
		return rm.Name
	}
	return ""
}
