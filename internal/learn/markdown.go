package learn

import (
	"bytes"
	"html/template"
	"io/fs"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	ghtml "github.com/yuin/goldmark/renderer/html"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

type renderer struct {
	fs fs.FS
	md goldmark.Markdown
}

func newRenderer() *renderer {
	return &renderer{
		fs: opencrafters.ChallengesFS(),
		md: goldmark.New(
			goldmark.WithExtensions(extension.GFM),
			goldmark.WithRendererOptions(ghtml.WithUnsafe()),
		),
	}
}

func (r *renderer) render(rel string) (template.HTML, error) {
	src, err := fs.ReadFile(r.fs, rel)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := r.md.Convert(src, &buf); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec // trusted, embedded content
}
