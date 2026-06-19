package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	ghtml "github.com/yuin/goldmark/renderer/html"

	opencrafters "github.com/Rohithgilla12/open-crafters"
)

// cmdSite renders the embedded challenge content into a static website.
//
//	crafters site [--out dir]   (default: ./site)
func cmdSite(args []string) {
	out := "site"
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--out":
			i++
			if i < len(args) {
				out = args[i]
			}
		case strings.HasPrefix(a, "--out="):
			out = strings.TrimPrefix(a, "--out=")
		}
	}

	cfs := opencrafters.ChallengesFS()
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(ghtml.WithUnsafe()),
	)
	render := func(rel string) (template.HTML, error) {
		src, err := fs.ReadFile(cfs, rel)
		if err != nil {
			return "", err
		}
		var buf bytes.Buffer
		if err := md.Convert(src, &buf); err != nil {
			return "", err
		}
		return template.HTML(buf.String()), nil //nolint:gosec // trusted, embedded content
	}

	var site []siteChallenge
	for _, slug := range orderedSlugs() {
		ch := challenges[slug]
		sc := siteChallenge{Slug: slug, Name: ch.Name}

		if y, err := fs.ReadFile(cfs, path.Join(slug, "challenge.yaml")); err == nil {
			sc.Tagline = tagline(y)
			sc.Difficulty = yamlField(y, "difficulty")
		}
		if html, err := render(path.Join(slug, "PROTOCOL.md")); err == nil {
			sc.ProtocolHTML = html
		}
		for i := range ch.Stages {
			s := ch.Stages[i]
			base := path.Base(s.Instructions)
			html, err := render(path.Join(slug, "stages", base))
			if err != nil {
				die("rendering %s: %v", s.Instructions, err)
			}
			sc.Stages = append(sc.Stages, siteStage{Num: i + 1, Slug: s.Slug, Name: s.Name, Difficulty: s.Difficulty, HTML: html})
		}
		sc.DiffMix = difficultyMix(sc.Stages)
		site = append(site, sc)
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		die("%v", err)
	}
	write := func(name string, tmpl *template.Template, data any) {
		f, err := os.Create(filepath.Join(out, name))
		if err != nil {
			die("%v", err)
		}
		defer f.Close()
		if err := tmpl.Execute(f, data); err != nil {
			die("rendering %s: %v", name, err)
		}
	}

	write("index.html", indexTmpl, site)
	for _, sc := range site {
		write(sc.Slug+".html", challengeTmpl, sc)
	}
	if err := os.WriteFile(filepath.Join(out, "style.css"), []byte(siteCSS), 0o644); err != nil {
		die("%v", err)
	}
	// Copy every static asset (favicon, OG image, apple-touch-icon) to the root.
	afs := opencrafters.AssetsFS()
	if err := fs.WalkDir(afs, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(afs, p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(out, filepath.FromSlash(p)), b, 0o644)
	}); err != nil {
		die("copying assets: %v", err)
	}
	fmt.Printf("wrote %d pages + style.css to %s/\n", len(site)+1, out)
}

type siteStage struct {
	Num        int
	Slug, Name string
	Difficulty string
	HTML       template.HTML
}

type siteChallenge struct {
	Slug, Name, Tagline string
	Difficulty          string
	Stages              []siteStage
	DiffMix             template.HTML
	ProtocolHTML        template.HTML
}

// yamlField returns a top-level (non-indented) scalar field from a
// challenge.yaml, e.g. yamlField(y, "difficulty") → "hard".
func yamlField(y []byte, key string) string {
	for _, ln := range strings.Split(string(y), "\n") {
		if strings.HasPrefix(ln, key+":") {
			return strings.TrimSpace(strings.TrimPrefix(ln, key+":"))
		}
	}
	return ""
}

// difficultyMix renders a small colored breakdown of stage difficulties.
func difficultyMix(stages []siteStage) template.HTML {
	n := map[string]int{}
	for _, s := range stages {
		n[s.Difficulty]++
	}
	var parts []string
	for _, d := range []string{"easy", "medium", "hard"} {
		if n[d] > 0 {
			parts = append(parts, fmt.Sprintf(`<span class="diff diff-%s">%d %s</span>`, d, n[d], d))
		}
	}
	return template.HTML(strings.Join(parts, " ")) //nolint:gosec // fixed, internal strings
}

// tagline pulls the folded `short_description:` block out of a challenge.yaml
// without a YAML dependency.
func tagline(y []byte) string {
	var out []string
	capturing := false
	for _, ln := range strings.Split(string(y), "\n") {
		if strings.HasPrefix(ln, "short_description:") {
			capturing = true
			continue
		}
		if !capturing {
			continue
		}
		if strings.TrimSpace(ln) == "" || strings.HasPrefix(ln, " ") || strings.HasPrefix(ln, "\t") {
			if t := strings.TrimSpace(ln); t != "" {
				out = append(out, t)
			}
			continue
		}
		break
	}
	return strings.Join(out, " ")
}

var tmplFuncs = template.FuncMap{
	"short": func(slug string) string { return strings.TrimPrefix(slug, "build-your-own-") },
}

var indexTmpl = template.Must(template.New("index").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>open-crafters — build your own X for serious infrastructure</title>
<meta name="description" content="Open-source build-your-own-X challenges for the production-infrastructure primitives senior engineers actually wrestle with — workflow engines, write-ahead logs, message queues, MVCC, Kafka-style logs. Implement in any language; graded black-box over the wire, crashes included.">
<link rel="icon" href="favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="apple-touch-icon.png">
<link rel="canonical" href="https://rohithgilla12.github.io/open-crafters/">
<meta property="og:type" content="website">
<meta property="og:title" content="open-crafters — build your own X for serious infrastructure">
<meta property="og:description" content="Build-your-own-X challenges for real infra primitives: workflow engines, WALs, message queues, MVCC, logs. Any language, graded black-box over the wire — crashes included.">
<meta property="og:url" content="https://rohithgilla12.github.io/open-crafters/">
<meta property="og:image" content="https://rohithgilla12.github.io/open-crafters/og.png">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="open-crafters — build your own X for serious infrastructure">
<meta name="twitter:description" content="Build-your-own-X challenges for real infra primitives, in any language. Graded black-box over the wire, crashes included.">
<meta name="twitter:image" content="https://rohithgilla12.github.io/open-crafters/og.png">
<link rel="stylesheet" href="style.css">
</head><body><div class="wrap">
<header class="hero">
  <h1><span class="prompt">$</span> open-crafters</h1>
  <p class="tag">Open-source <em>build-your-own-X</em> challenges for the production-infrastructure
  primitives senior engineers actually wrestle with. Implement the system in any language;
  a test harness grades you stage by stage, entirely over the wire — crashes included.</p>
  <div class="install">
    <code>curl -fsSL https://raw.githubusercontent.com/Rohithgilla12/open-crafters/main/install.sh | sh</code>
  </div>
  <p class="sub">then <code>crafters</code> for the dashboard, or <code>crafters start {{(index . 0).Slug | short}}</code></p>
</header>
<main class="grid">
{{range .}}
  <a class="card" href="{{.Slug}}.html">
    <h2>{{.Name}} <span class="badge diff-{{.Difficulty}}">{{.Difficulty}}</span></h2>
    <p>{{.Tagline}}</p>
    <div class="mix">{{.DiffMix}}</div>
    <span class="meta">{{len .Stages}} stages →</span>
  </a>
{{end}}
</main>
<footer><a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> · graded black-box · any language with a TCP socket</footer>
</div></body></html>`))

var challengeTmpl = template.Must(template.New("challenge").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} — open-crafters</title>
<meta name="description" content="{{.Tagline}}">
<link rel="icon" href="favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="apple-touch-icon.png">
<link rel="canonical" href="https://rohithgilla12.github.io/open-crafters/{{.Slug}}.html">
<meta property="og:type" content="article">
<meta property="og:title" content="{{.Name}} — open-crafters">
<meta property="og:description" content="{{.Tagline}}">
<meta property="og:url" content="https://rohithgilla12.github.io/open-crafters/{{.Slug}}.html">
<meta property="og:image" content="https://rohithgilla12.github.io/open-crafters/og.png">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="{{.Name}} — open-crafters">
<meta name="twitter:description" content="{{.Tagline}}">
<meta name="twitter:image" content="https://rohithgilla12.github.io/open-crafters/og.png">
<link rel="stylesheet" href="style.css">
</head><body><div class="wrap">
<p class="back"><a href="index.html">← all challenges</a></p>
<header class="chead">
  <h1>{{.Name}} <span class="badge diff-{{.Difficulty}}">{{.Difficulty}}</span></h1>
  <p class="tag">{{.Tagline}}</p>
  <div class="install"><code>crafters start {{.Slug | short}}</code></div>
</header>

<h2 class="section">Stages</h2>
<ol class="stages">
{{range .Stages}}
  <li><details>
    <summary><span class="num">{{.Num}}</span> {{.Name}} <span class="diff diff-{{.Difficulty}}">{{.Difficulty}}</span> <span class="slug">{{.Slug}}</span></summary>
    <div class="md">{{.HTML}}</div>
  </details></li>
{{end}}
</ol>

<h2 class="section">Protocol</h2>
<div class="md protocol">{{.ProtocolHTML}}</div>

<footer><a href="index.html">← all challenges</a> · <a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a></footer>
</div></body></html>`))

const siteCSS = `:root{
  --bg:#0b0e14; --panel:#11151f; --panel2:#161b27; --border:#222a39;
  --fg:#d7dce5; --dim:#8b96a8; --accent:#5ad1b3; --accent2:#7aa2f7; --code:#e6db9a;
}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);
  font:16px/1.65 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;}
.wrap{max-width:860px;margin:0 auto;padding:2.5rem 1.25rem 4rem;}
a{color:var(--accent2);text-decoration:none}
a:hover{text-decoration:underline}
code,pre,.mono{font-family:"SF Mono",ui-monospace,Menlo,Consolas,monospace;}
h1{font-size:2.1rem;margin:.2rem 0;letter-spacing:-.02em}
.prompt{color:var(--accent)}
.hero{padding:2rem 0 1rem;border-bottom:1px solid var(--border);margin-bottom:2rem}
.hero .tag{color:var(--dim);font-size:1.05rem;max-width:62ch}
.tag em{color:var(--fg);font-style:italic}
.install{margin:1.2rem 0 .4rem}
.install code{display:block;background:var(--panel);border:1px solid var(--border);
  border-left:3px solid var(--accent);border-radius:8px;padding:.8rem 1rem;
  color:var(--code);overflow-x:auto;font-size:.92rem}
.sub{color:var(--dim);font-size:.92rem}
.sub code{color:var(--accent)}
.grid{display:grid;grid-template-columns:1fr;gap:1rem}
@media(min-width:640px){.grid{grid-template-columns:1fr 1fr}}
.card{display:block;background:var(--panel);border:1px solid var(--border);border-radius:12px;
  padding:1.2rem 1.3rem;transition:border-color .15s,transform .15s,background .15s}
.card:hover{border-color:var(--accent);transform:translateY(-2px);background:var(--panel2);text-decoration:none}
.card h2{margin:.1rem 0 .5rem;font-size:1.2rem;color:var(--fg)}
.card p{margin:0 0 .9rem;color:var(--dim);font-size:.93rem}
.card .meta{color:var(--accent);font-size:.85rem;font-family:ui-monospace,monospace}
.back{margin:0 0 1rem;font-size:.9rem}
.chead{border-bottom:1px solid var(--border);padding-bottom:1.4rem;margin-bottom:1.6rem}
.chead .tag{color:var(--dim);max-width:64ch}
.section{font-size:.8rem;text-transform:uppercase;letter-spacing:.12em;color:var(--accent);
  margin:2.2rem 0 .8rem;border-bottom:1px solid var(--border);padding-bottom:.4rem}
.stages{list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:.5rem}
details{background:var(--panel);border:1px solid var(--border);border-radius:10px;overflow:hidden}
details[open]{border-color:var(--accent2)}
summary{cursor:pointer;padding:.8rem 1rem;font-weight:600;list-style:none;display:flex;align-items:center;gap:.7rem}
summary::-webkit-details-marker{display:none}
summary:hover{background:var(--panel2)}
.num{display:inline-flex;align-items:center;justify-content:center;min-width:1.7rem;height:1.7rem;
  background:var(--panel2);border:1px solid var(--border);border-radius:50%;color:var(--accent);
  font-size:.82rem;font-family:ui-monospace,monospace}
.slug{margin-left:auto;color:var(--dim);font-size:.8rem;font-family:ui-monospace,monospace}
.diff{font-size:.7rem;font-weight:600;text-transform:uppercase;letter-spacing:.05em;
  padding:.1rem .45rem;border-radius:999px;border:1px solid currentColor}
.diff-easy{color:#5ad1b3}.diff-medium{color:#e0c45a}.diff-hard{color:#e57a86}
.badge{font-size:.62rem;font-weight:700;text-transform:uppercase;letter-spacing:.08em;
  padding:.18rem .5rem;border-radius:999px;vertical-align:middle;color:#0b0e14}
.badge.diff-easy{background:#5ad1b3}.badge.diff-medium{background:#e0c45a}.badge.diff-hard{background:#e57a86}
.mix{display:flex;gap:.4rem;flex-wrap:wrap;margin:0 0 .9rem}
.mix .diff{border:none;padding:0;font-size:.72rem}
.md{padding:.2rem 1.2rem 1.1rem}
.md h1,.md h2,.md h3{letter-spacing:-.01em;margin:1.3rem 0 .6rem}
.md h1{font-size:1.4rem}.md h2{font-size:1.15rem}.md h3{font-size:1rem}
.md p,.md li{color:var(--fg)}
.md a{color:var(--accent2)}
.md code{background:var(--panel2);padding:.12em .4em;border-radius:5px;color:var(--code);font-size:.88em}
.md pre{background:#0d1119;border:1px solid var(--border);border-radius:8px;padding:1rem;overflow-x:auto}
.md pre code{background:none;padding:0;color:var(--fg)}
.md table{border-collapse:collapse;width:100%;margin:1rem 0;font-size:.9rem}
.md th,.md td{border:1px solid var(--border);padding:.5rem .7rem;text-align:left}
.md th{background:var(--panel2);color:var(--accent)}
.md blockquote{border-left:3px solid var(--accent2);margin:1rem 0;padding:.2rem 1rem;color:var(--dim)}
.protocol{background:var(--panel);border:1px solid var(--border);border-radius:10px;padding:.2rem 1.4rem 1.2rem}
footer{margin-top:3rem;padding-top:1.4rem;border-top:1px solid var(--border);color:var(--dim);font-size:.88rem}
`
