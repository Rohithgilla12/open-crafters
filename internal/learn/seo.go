package learn

import (
	"encoding/json"
	"fmt"
	"html/template"
	"strings"
)

const learnBaseURL = "https://learn.gilla.fun"

const defaultOGImage = "https://learn.gilla.fun/og.png"

// SEO holds per-page search / social metadata.
type SEO struct {
	Title       string
	Description string
	Path        string // absolute path, e.g. /challenges/build-your-own-wal
	Type        string // website | article
	JSONLD      any    // optional structured data object
}

func (s SEO) Canonical() string {
	path := s.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return learnBaseURL + path
}

func (s SEO) OGType() string {
	if s.Type == "article" {
		return "article"
	}
	return "website"
}

func seoHeadHTML(s SEO) template.HTML {
	title := htmlEsc(s.Title)
	desc := htmlEsc(s.Description)
	canon := htmlEsc(s.Canonical())
	ogType := htmlEsc(s.OGType())
	img := htmlEsc(defaultOGImage)

	var b strings.Builder
	fmt.Fprintf(&b, `<title>%s</title>
<meta name="description" content="%s">
<link rel="canonical" href="%s">
<meta property="og:type" content="%s">
<meta property="og:title" content="%s">
<meta property="og:description" content="%s">
<meta property="og:url" content="%s">
<meta property="og:image" content="%s">
<meta property="og:site_name" content="open-crafters learn">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="%s">
<meta name="twitter:description" content="%s">
<meta name="twitter:image" content="%s">
`, title, desc, canon, ogType, title, desc, canon, img, title, desc, img)

	if s.JSONLD != nil {
		raw, err := json.Marshal(s.JSONLD)
		if err == nil {
			fmt.Fprintf(&b, `<script type="application/ld+json">%s</script>
`, string(raw))
		}
	}
	return template.HTML(b.String()) //nolint:gosec // values escaped / JSON marshaled
}

func htmlEsc(s string) string {
	replacer := strings.NewReplacer(
		`&`, "&amp;",
		`"`, "&quot;",
		`<`, "&lt;",
		`>`, "&gt;",
	)
	return replacer.Replace(s)
}

func websiteJSONLD() map[string]any {
	return map[string]any{
		"@context": "https://schema.org",
		"@type":    "WebSite",
		"name":     "open-crafters learn",
		"url":      learnBaseURL + "/",
		"description": "Build-your-own-X challenges for production infrastructure primitives, " +
			"plus system design scenarios — graded black-box over the wire.",
	}
}

func learningResourceJSONLD(name, description, path string) map[string]any {
	return map[string]any{
		"@context":    "https://schema.org",
		"@type":       "LearningResource",
		"name":        name,
		"description": description,
		"url":         learnBaseURL + path,
		"provider": map[string]any{
			"@type": "Organization",
			"name":  "open-crafters",
			"url":   learnBaseURL + "/",
		},
		"isAccessibleForFree":  true,
		"learningResourceType": "challenge",
	}
}

func articleJSONLD(title, description, path, datePublished string) map[string]any {
	m := map[string]any{
		"@context":    "https://schema.org",
		"@type":       "Article",
		"headline":    title,
		"description": description,
		"url":         learnBaseURL + path,
		"author": map[string]any{
			"@type": "Organization",
			"name":  "open-crafters",
		},
		"publisher": map[string]any{
			"@type": "Organization",
			"name":  "open-crafters",
			"url":   learnBaseURL + "/",
		},
		"mainEntityOfPage": learnBaseURL + path,
	}
	if datePublished != "" {
		m["datePublished"] = datePublished
	}
	return m
}
