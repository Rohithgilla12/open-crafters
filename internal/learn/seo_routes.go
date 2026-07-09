package learn

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) handleRobots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w, "User-agent: *\nAllow: /\n\nSitemap: %s/sitemap.xml\n", learnBaseURL)
}

func (s *Server) handleSitemap(w http.ResponseWriter, _ *http.Request) {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	add := func(path string) {
		if path == "" {
			path = "/"
		}
		fmt.Fprintf(&b, "  <url><loc>%s%s</loc></url>\n", learnBaseURL, path)
	}
	add("/")
	add("/roadmaps")
	add("/design")
	add("/design/roadmaps")
	add("/design/stacks")
	add("/blog")
	for _, rm := range s.catalog.Roadmaps {
		add("/roadmaps/" + rm.Slug)
	}
	for _, slug := range s.catalog.DesignOrder {
		add("/design/" + slug)
	}
	for _, rm := range s.catalog.DesignRoadmaps {
		add("/design/roadmaps/" + rm.Slug)
	}
	for _, st := range s.catalog.DesignStacks {
		add("/design/stacks/" + st.Slug)
	}
	for _, slug := range s.catalog.Order {
		add("/challenges/" + slug)
	}
	for _, p := range s.catalog.BlogPosts {
		add("/blog/" + p.Slug)
	}
	b.WriteString("</urlset>\n")
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func (s *Server) handleBlogIndex(w http.ResponseWriter, _ *http.Request) {
	s.render(w, blogIndexTmpl, blogIndexData{
		Posts: s.catalog.BlogPosts,
		SEO: SEO{
			Title:       "Blog — open-crafters learn",
			Description: "Essays on durability, workflows, consensus, and the primitives behind production infrastructure — tied to graded build-your-own challenges.",
			Path:        "/blog",
			Type:        "website",
		},
	})
}

func (s *Server) handleBlogPost(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	post, ok := s.catalog.GetBlog(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	var related []*Challenge
	var relatedDesigns []*DesignProblem
	for _, id := range post.Related {
		if ch, ok := s.catalog.Get(id); ok {
			related = append(related, ch)
			continue
		}
		if d, ok := s.catalog.GetDesign(id); ok {
			relatedDesigns = append(relatedDesigns, d)
		}
	}
	path := "/blog/" + post.Slug
	date := ""
	if !post.Date.IsZero() {
		date = post.Date.Format("2006-01-02")
	}
	s.render(w, blogPostTmpl, blogPostData{
		Post:           post,
		Related:        related,
		RelatedDesigns: relatedDesigns,
		SEO: SEO{
			Title:       post.Title + " — open-crafters",
			Description: post.Description,
			Path:        path,
			Type:        "article",
			JSONLD:      articleJSONLD(post.Title, post.Description, path, date),
		},
	})
}

type blogIndexData struct {
	Posts []BlogPost
	SEO   SEO
}

type blogPostData struct {
	Post           *BlogPost
	Related        []*Challenge
	RelatedDesigns []*DesignProblem
	SEO            SEO
}
