package learn

import (
	"bytes"
	"html/template"
	"io/fs"
	"path"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	ghtml "github.com/yuin/goldmark/renderer/html"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

type renderer struct {
	chFS     fs.FS
	designFS fs.FS
	md       goldmark.Markdown
}

func newRenderer() *renderer {
	return &renderer{
		chFS:     opencrafters.ChallengesFS(),
		designFS: opencrafters.DesignFS(),
		md: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithRendererOptions(ghtml.WithUnsafe()),
		),
	}
}

func (r *renderer) render(rel string) (template.HTML, error) {
	return r.renderFS(r.chFS, rel)
}

func (r *renderer) renderDesign(slug, file string) (template.HTML, error) {
	return r.renderFS(r.designFS, path.Join(slug, file))
}

func (r *renderer) renderFS(fsys fs.FS, rel string) (template.HTML, error) {
	src, err := fs.ReadFile(fsys, rel)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := r.md.Convert(src, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // trusted, embedded content
}
