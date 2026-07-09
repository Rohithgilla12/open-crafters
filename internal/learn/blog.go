package learn

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	ghtml "github.com/yuin/goldmark/renderer/html"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

// BlogPost is a rendered blog article.
type BlogPost struct {
	Slug        string
	Title       string
	Description string
	Date        time.Time
	DateDisplay string
	Tags        []string
	Related     []string // challenge or design slugs
	HTML        template.HTML
}

// LoadBlogPosts reads embedded markdown posts from content/blog.
func LoadBlogPosts() ([]BlogPost, error) {
	bfs := opencrafters.BlogFS()
	entries, err := fs.ReadDir(bfs, ".")
	if err != nil {
		return nil, err
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(ghtml.WithUnsafe()),
	)

	var posts []BlogPost
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := fs.ReadFile(bfs, e.Name())
		if err != nil {
			return nil, err
		}
		meta, body, err := parseFrontmatter(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		slug := strings.TrimSuffix(e.Name(), ".md")
		if v := meta["slug"]; v != "" {
			slug = v
		}
		title := meta["title"]
		if title == "" {
			title = slug
		}
		desc := meta["description"]
		dateStr := meta["date"]
		var date time.Time
		if dateStr != "" {
			date, err = time.Parse("2006-01-02", dateStr)
			if err != nil {
				return nil, fmt.Errorf("%s: bad date %q", e.Name(), dateStr)
			}
		}
		var tags []string
		if meta["tags"] != "" {
			for _, t := range strings.Split(meta["tags"], ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}
		var related []string
		if meta["related"] != "" {
			for _, r := range strings.Split(meta["related"], ",") {
				r = strings.TrimSpace(r)
				if r != "" {
					related = append(related, r)
				}
			}
		}
		var buf bytes.Buffer
		if err := md.Convert(body, &buf); err != nil {
			return nil, fmt.Errorf("%s: markdown: %w", e.Name(), err)
		}
		posts = append(posts, BlogPost{
			Slug:        slug,
			Title:       title,
			Description: desc,
			Date:        date,
			DateDisplay: date.Format("Jan 2, 2006"),
			Tags:        tags,
			Related:     related,
			HTML:        template.HTML(buf.String()), //nolint:gosec // trusted embedded content
		})
	}

	sort.Slice(posts, func(i, j int) bool {
		if posts[i].Date.Equal(posts[j].Date) {
			return posts[i].Slug > posts[j].Slug
		}
		return posts[i].Date.After(posts[j].Date)
	})
	return posts, nil
}

// parseFrontmatter splits optional YAML-like --- frontmatter (simple key: value lines).
func parseFrontmatter(src []byte) (map[string]string, []byte, error) {
	text := string(src)
	meta := map[string]string{}
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return meta, src, nil
	}
	rest := text[3:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	end := strings.Index(rest, "\n---\n")
	crlfEnd := strings.Index(rest, "\r\n---\r\n")
	sepLen := 5
	if crlfEnd >= 0 && (end < 0 || crlfEnd < end) {
		end = crlfEnd
		sepLen = 7
	}
	if end < 0 {
		alt := strings.Index(rest, "\n---")
		if alt >= 0 && alt+4 == len(rest) {
			end = alt
			sepLen = 4
		} else {
			return nil, nil, fmt.Errorf("unclosed frontmatter")
		}
	}
	fm := rest[:end]
	body := rest[end+sepLen:]
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		meta[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return meta, []byte(body), nil
}
