package learn

import (
	"html/template"
	"strings"
)

var tmplFuncs = template.FuncMap{
	"short":     shortSlug,
	"stagesCSV": stagesCSV,
	"add":       func(a, b int) int { return a + b },
	"siteNav":   func() template.HTML { return template.HTML(siteNavHTML) },
	"fontLinks": func() template.HTML { return template.HTML(fontLinksHTML) },
}

const fontLinksHTML = `<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,400;0,9..40,500;0,9..40,600;1,9..40,400&family=JetBrains+Mono:wght@400;500;600&family=Sora:wght@500;600;700&display=swap" rel="stylesheet">`

const siteNavHTML = `<nav class="site-nav" aria-label="Main">
  <div class="site-nav-inner">
    <a class="site-brand" href="/"><span class="brand-mark">$</span> open-crafters <span class="brand-badge">learn</span></a>
    <div class="site-links">
      <a href="/roadmaps">Roadmaps</a>
      <a href="https://runner.gilla.fun">Runner</a>
      <a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a>
    </div>
  </div>
</nav>`

func stagesCSV(stages []Stage) string {
	var parts []string
	for _, s := range stages {
		parts = append(parts, s.Slug)
	}
	return strings.Join(parts, ",")
}

var indexTmpl = template.Must(template.New("index").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>open-crafters — learn</title>
<meta name="description" content="Build-your-own-X challenges for production infrastructure primitives. Read stages, study the protocol, then implement and grade over the wire.">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
{{fontLinks}}
<link rel="stylesheet" href="/style.css">
</head><body>
{{siteNav}}
<div class="wrap">
<header class="hero">
  <div class="hero-grid">
  <div class="hero-main">
  <p class="eyebrow">14 challenges · 4 paths · graded black-box</p>
  <h1>Build the infrastructure<br>senior engineers actually ship.</h1>
  <p class="tag">Open-source <em>build-your-own-X</em> challenges for production primitives.
  Read each stage, implement in any language, and grade over the wire — crashes included.</p>
  <div class="install">
    <span class="install-label">Install</span>
    <code>curl -fsSL https://raw.githubusercontent.com/Rohithgilla12/open-crafters/main/install.sh | sh</code>
  </div>
  <p class="sub">then <code>crafters start wal</code> locally, or submit to the
  <a href="https://runner.gilla.fun">hosted runner</a></p>
  </div>
  <aside class="hero-aside">
    <a class="hero-link-card" href="/roadmaps">
      <span class="hero-link-kicker">Start here</span>
      <strong>Learning roadmaps</strong>
      <span class="hero-link-meta">Curated journeys with outcomes →</span>
    </a>
    <a class="hero-link-card" href="https://runner.gilla.fun">
      <span class="hero-link-kicker">Remote grading</span>
      <strong>Hosted runner</strong>
      <span class="hero-link-meta">Submit zips from the browser →</span>
    </a>
  </aside>
  </div>
  <details class="progress-sync">
    <summary class="progress-sync-title">Progress sync</summary>
    <p class="progress-sync-help">Sync with local <code>crafters test</code> via <code>progress.json</code> —
    export from the CLI (<code>crafters progress export</code>) and import here, or export to back up browser progress.</p>
    <div class="progress-sync-actions">
      <button type="button" class="btn btn-secondary" id="progress-export">Export progress.json</button>
      <label class="btn btn-secondary btn-file">Import progress.json
        <input type="file" id="progress-import" accept="application/json,.json" hidden>
      </label>
    </div>
    <p id="progress-sync-status" class="progress-sync-status" aria-live="polite"></p>
  </details>
</header>
<section class="roadmap-strip">
  <div class="roadmap-strip-head">
    <h2 class="section-label">Learning roadmaps</h2>
    <a class="text-link" href="/roadmaps">View all →</a>
  </div>
  <div class="roadmap-cards">
  {{range .Roadmaps}}
    <a class="roadmap-card" href="/roadmaps/{{.Slug}}" data-roadmap-card data-challenges="{{.ChallengeCSV}}" data-total-stages="{{.TotalStages}}">
      <h3>{{.Name}}</h3>
      <p>{{.Tagline}}</p>
      <span class="roadmap-meta"><span data-roadmap-progress-label></span>{{.TotalStages}} stages</span>
      <span class="roadmap-bar"><span class="roadmap-bar-fill"></span></span>
    </a>
  {{end}}
  </div>
</section>
{{range .Paths}}
<section class="path-section" id="path-{{.Slug}}" data-path="{{.Slug}}">
  <div class="path-head">
    <h2 class="path-title"><a href="/roadmaps/{{.Slug}}">{{.Name}}</a></h2>
    <p class="path-desc">{{.Description}}</p>
  </div>
  <main class="grid">
  {{range .Challenges}}
    <a class="card" href="/challenges/{{.Slug}}" data-challenge="{{.Slug}}" data-stages="{{stagesCSV .Stages}}">
      <div class="card-top">
        <h3>{{.Name}}</h3>
        <span class="badge diff-{{.Difficulty}}">{{.Difficulty}}</span>
      </div>
      <p>{{.Tagline}}</p>
      <div class="mix">{{.DiffMix}}</div>
      <span class="meta"><span data-progress-label></span>{{len .Stages}} stages →</span>
    </a>
  {{end}}
  </main>
</section>
{{end}}
<footer class="site-footer">
  <a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> ·
  <a href="https://runner.gilla.fun">hosted runner</a> ·
  graded black-box · any language with a TCP socket
</footer>
</div><script src="/learn.js"></script></body></html>`))

var roadmapsIndexTmpl = template.Must(template.New("roadmaps").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Roadmaps — open-crafters learn</title>
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
{{fontLinks}}
<link rel="stylesheet" href="/style.css">
</head><body>
{{siteNav}}
<div class="wrap">
<p class="back"><a href="/">← home</a></p>
<header class="page-header">
  <h1>Learning roadmaps</h1>
  <p class="tag">Curated journeys through the catalog — each with outcomes, milestones, and suggested order.</p>
</header>
<div class="roadmap-cards roadmap-cards-page">
{{range .}}
  <a class="roadmap-card" href="/roadmaps/{{.Slug}}" data-roadmap-card data-challenges="{{.ChallengeCSV}}" data-total-stages="{{.TotalStages}}">
    <h2>{{.Name}}</h2>
    <p>{{.Tagline}}</p>
    <p class="roadmap-desc">{{.Description}}</p>
    <span class="roadmap-meta"><span data-roadmap-progress-label></span>{{.TotalStages}} stages · {{len .Milestones}} milestones</span>
    <span class="roadmap-bar"><span class="roadmap-bar-fill"></span></span>
  </a>
{{end}}
</div>
<footer class="site-footer"><a href="/">← home</a></footer>
</div><script src="/learn.js"></script></body></html>`))

var roadmapTmpl = template.Must(template.New("roadmap").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} — open-crafters roadmap</title>
<meta name="description" content="{{.Tagline}}">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
{{fontLinks}}
<link rel="stylesheet" href="/style.css">
</head><body data-roadmap-page data-challenges="{{.ChallengeCSV}}">
{{siteNav}}
<div class="wrap">
<p class="back"><a href="/roadmaps">← all roadmaps</a></p>
<header class="page-header">
  <p class="eyebrow">Roadmap</p>
  <h1>{{.Name}}</h1>
  <p class="tag">{{.Description}}</p>
  {{if .StartCommand}}<div class="install"><span class="install-label">Start</span><code>{{.StartCommand}}</code></div>{{end}}
  <div class="roadmap-progress-head" data-roadmap-bar data-total-stages="{{.TotalStages}}">
    <span data-roadmap-progress-label class="roadmap-progress-label">0/{{.TotalStages}} stages</span>
    <span class="roadmap-bar roadmap-bar-lg"><span class="roadmap-bar-fill"></span></span>
  </div>
</header>
{{if .Outcomes}}
<h2 class="section-label">What you'll learn</h2>
<ul class="roadmap-outcomes">
{{range .Outcomes}}<li>{{.}}</li>{{end}}
</ul>
{{end}}
<h2 class="section-label">Milestones</h2>
<ol class="roadmap-timeline">
{{range .Milestones}}
  <li class="roadmap-milestone">
    {{if .PathSlug}}
    <a class="card path-card" href="/roadmaps/{{.PathSlug}}" data-roadmap-milestone>
      <span class="path-step">{{.Num}}</span>
      <div class="path-card-body">
        <h2>{{.PathName}}</h2>
        <p>{{.Blurb}}</p>
        <span class="meta">Open roadmap →</span>
      </div>
    </a>
    {{else}}
    <a class="card path-card" href="/challenges/{{.Challenge.Slug}}" data-challenge="{{.Challenge.Slug}}" data-stages="{{.StageCSV}}" data-roadmap-milestone>
      <span class="path-step">{{.Num}}</span>
      <div class="path-card-body">
        <h2>{{.Challenge.Name}} <span class="badge diff-{{.Challenge.Difficulty}}">{{.Challenge.Difficulty}}</span></h2>
        <p>{{.Blurb}}</p>
        <span class="meta"><span data-progress-label></span>{{len .Challenge.Stages}} stages →</span>
      </div>
    </a>
    {{end}}
  </li>
{{end}}
</ol>
<footer class="site-footer"><a href="/roadmaps">← all roadmaps</a></footer>
</div><script src="/learn.js"></script></body></html>`))

var pathTmpl = template.Must(template.New("path").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} — open-crafters learn</title>
<meta name="description" content="{{.Description}}">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
{{fontLinks}}
<link rel="stylesheet" href="/style.css">
</head><body>
{{siteNav}}
<div class="wrap">
<p class="back"><a href="/">← all paths</a></p>
<header class="page-header">
  <h1>{{.Name}}</h1>
  <p class="tag">{{.Description}}</p>
</header>
<ol class="path-challenges">
{{range $i, $ch := .Challenges}}
  <li>
    <a class="card path-card" href="/challenges/{{$ch.Slug}}" data-challenge="{{$ch.Slug}}" data-stages="{{stagesCSV $ch.Stages}}">
      <span class="path-step">{{add $i 1}}</span>
      <div class="path-card-body">
        <h2>{{$ch.Name}} <span class="badge diff-{{$ch.Difficulty}}">{{$ch.Difficulty}}</span></h2>
        <p>{{$ch.Tagline}}</p>
        <span class="meta"><span data-progress-label></span>{{len $ch.Stages}} stages →</span>
      </div>
    </a>
  </li>
{{end}}
</ol>
<footer class="site-footer"><a href="/">← all paths</a> · <a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a></footer>
</div><script src="/learn.js"></script></body></html>`))

var challengeTmpl = template.Must(template.New("challenge").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Challenge.Name}} — open-crafters learn</title>
<meta name="description" content="{{.Challenge.Tagline}}">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
{{fontLinks}}
<link rel="stylesheet" href="/style.css">
</head><body data-challenge="{{.Challenge.Slug}}" data-stages="{{.StageSlugs}}">
{{siteNav}}
<div class="wrap">
<p class="back"><a href="/">← all challenges</a>{{if .RoadmapSlug}} · <a href="/roadmaps/{{.RoadmapSlug}}">{{.RoadmapName}}</a>{{end}}</p>
<header class="page-header">
  <div class="card-top">
    <h1>{{.Challenge.Name}}</h1>
    <span class="badge diff-{{.Challenge.Difficulty}}">{{.Challenge.Difficulty}}</span>
  </div>
  {{if .RoadmapName}}<p class="path-crumb">Roadmap: <a href="/roadmaps/{{.RoadmapSlug}}">{{.RoadmapName}}</a></p>{{end}}
  <p class="tag">{{.Challenge.Tagline}}</p>
  <p class="progress-summary"><span data-progress-label></span></p>
  <div class="install"><span class="install-label">Local start</span><code>crafters start {{.Challenge.Slug | short}}</code></div>
</header>

<h2 class="section-label">Submit to hosted runner</h2>
<div class="panel submit-panel">
  <p class="panel-help">Zip your solution directory (must include <code>your_program.sh</code>) and grade remotely — same as <code>crafters submit</code>.</p>
  <form id="submit-form" data-challenge="{{.Challenge.Slug}}">
    <label class="field">Runner token <input type="password" name="token" autocomplete="off" placeholder="from your runner dashboard"></label>
    <label class="field">Solution zip <input type="file" name="file" accept=".zip,application/zip"></label>
    <label class="check"><input type="checkbox" name="all"> Grade all stages</label>
    <button type="submit" class="btn btn-primary">Submit for grading</button>
  </form>
  <p id="submit-status" class="submit-status" aria-live="polite"></p>
  <pre id="submit-log" class="submit-log"></pre>
</div>

<h2 class="section-label">Stages</h2>
<ol class="stages">
{{range .Challenge.Stages}}
  <li>
    <a class="stage-link" href="/challenges/{{$.Challenge.Slug}}/stages/{{.Slug}}" data-stage-slug="{{.Slug}}">
      <span class="num">{{.Num}}</span>
      <span class="stage-name">{{.Name}}</span>
      <span class="diff diff-{{.Difficulty}}">{{.Difficulty}}</span>
      <span class="slug">{{.Slug}}</span>
    </a>
  </li>
{{end}}
</ol>

<h2 class="section-label" id="protocol">Protocol</h2>
<div class="md protocol panel">{{.Challenge.ProtocolHTML}}</div>

<footer class="site-footer"><a href="/">← all challenges</a> · <a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> · <a href="https://runner.gilla.fun">hosted runner</a></footer>
</div><script src="/learn.js"></script></body></html>`))

var stageTmpl = template.Must(template.New("stage").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Stage.Name}} — {{.Challenge.Name}} — open-crafters learn</title>
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
{{fontLinks}}
<link rel="stylesheet" href="/style.css">
</head><body class="stage-layout" data-challenge="{{.Challenge.Slug}}" data-stage="{{.Stage.Slug}}" data-stages="{{.StageSlugs}}">
{{siteNav}}
<div class="wrap stage-layout">
<p class="back"><a href="/challenges/{{.Challenge.Slug}}">← {{.Challenge.Name}}</a></p>

<div class="stage-grid">
<aside class="sidebar">
  <h2 class="sidebar-title">Stages</h2>
  <nav class="sidebar-nav">
  {{range .Challenge.Stages}}
    <a class="sidebar-item{{if eq .Slug $.Stage.Slug}} active{{end}}" href="/challenges/{{$.Challenge.Slug}}/stages/{{.Slug}}" data-stage-slug="{{.Slug}}">
      <span class="num">{{.Num}}</span>
      <span>{{.Name}}</span>
      <span class="diff diff-{{.Difficulty}}">{{.Difficulty}}</span>
    </a>
  {{end}}
  </nav>
  <a class="sidebar-protocol" href="/challenges/{{.Challenge.Slug}}#protocol">Protocol spec ↓</a>
</aside>

<main class="stage-main">
  <header class="stage-head">
    <h1><span class="num">{{.Stage.Num}}</span> {{.Stage.Name}}</h1>
    <span class="diff diff-{{.Stage.Difficulty}}">{{.Stage.Difficulty}}</span>
    <span class="slug mono">{{.Stage.Slug}}</span>
  </header>
  {{if .Stage.Hint}}
  <details class="hint-box">
    <summary>Stuck? Here's a nudge</summary>
    <p>{{.Stage.Hint}}</p>
  </details>
  {{end}}
  <div class="md">{{.Stage.HTML}}</div>
  <nav class="stage-pager">
    {{if .Prev}}<a class="pager prev" href="/challenges/{{.Challenge.Slug}}/stages/{{.Prev.Slug}}">← {{.Prev.Name}}</a>{{else}}<span></span>{{end}}
    {{if .Next}}<a class="pager next" href="/challenges/{{.Challenge.Slug}}/stages/{{.Next.Slug}}">{{.Next.Name}} →</a>{{end}}
  </nav>
</main>
</div>

<footer class="site-footer"><a href="/">← all challenges</a> · <a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> · <a href="https://runner.gilla.fun">hosted runner</a></footer>
</div><script src="/learn.js"></script></body></html>`))

// siteCSS matches the aesthetic from cmd/crafters/site.go with learn-app additions.
const siteCSS = `:root{
  --bg:#0c0f16;
  --bg-elevated:#111622;
  --surface:#151b28;
  --surface-hover:#1a2233;
  --border:#273044;
  --border-soft:#1e2738;
  --fg:#e8ecf4;
  --muted:#9aa6bc;
  --accent:#6ee7b7;
  --accent-dim:#3d9e82;
  --link:#8eb4ff;
  --code:#e8d98a;
  --shadow:0 12px 40px rgba(0,0,0,.35);
  --radius:14px;
  --radius-sm:10px;
  --font-ui:"DM Sans",system-ui,sans-serif;
  --font-display:"Sora",var(--font-ui);
  --font-mono:"JetBrains Mono",ui-monospace,monospace;
  --path-durability:#5eead4;
  --path-workflow:#c4b5fd;
  --path-distributed:#7dd3fc;
  --path-coordination:#fcd34d;
}
*{box-sizing:border-box}
html{scroll-behavior:smooth}
body{
  margin:0;
  min-height:100vh;
  background:
    radial-gradient(ellipse 80% 50% at 50% -20%,rgba(110,231,183,.08),transparent 60%),
    radial-gradient(ellipse 60% 40% at 100% 0%,rgba(142,180,255,.06),transparent 55%),
    linear-gradient(180deg,#0a0d14 0%,var(--bg) 40%,#0a0d14 100%);
  color:var(--fg);
  font:16px/1.65 var(--font-ui);
  -webkit-font-smoothing:antialiased;
}
::selection{background:rgba(110,231,183,.25);color:var(--fg)}
a{color:var(--link);text-decoration:none;transition:color .15s}
a:hover{color:#b4cffd;text-decoration:none}
code,pre,.mono{font-family:var(--font-mono)}
h1,h2,h3{font-family:var(--font-display);letter-spacing:-.03em;font-weight:600}
.wrap{max-width:920px;margin:0 auto;padding:2rem 1.25rem 4rem}
.wrap.stage-layout{max-width:1120px}

/* —— Nav —— */
.site-nav{
  position:sticky;top:0;z-index:50;
  background:rgba(12,15,22,.82);
  backdrop-filter:blur(14px);
  border-bottom:1px solid var(--border-soft);
}
.site-nav-inner{
  max-width:1120px;margin:0 auto;padding:.85rem 1.25rem;
  display:flex;align-items:center;justify-content:space-between;gap:1rem;
}
.site-brand{
  display:inline-flex;align-items:center;gap:.35rem;
  font-family:var(--font-display);font-weight:600;font-size:.95rem;color:var(--fg);
}
.site-brand:hover{color:var(--accent);text-decoration:none}
.brand-mark{color:var(--accent);font-family:var(--font-mono);font-weight:500}
.brand-badge{
  font-size:.62rem;font-weight:700;text-transform:uppercase;letter-spacing:.14em;
  color:var(--accent-dim);background:rgba(110,231,183,.1);
  border:1px solid rgba(110,231,183,.2);border-radius:999px;padding:.12rem .45rem;
}
.site-links{display:flex;align-items:center;gap:1.1rem;font-size:.88rem}
.site-links a{color:var(--muted)}
.site-links a:hover{color:var(--fg)}

/* —— Hero —— */
.hero{padding:1.5rem 0 2rem;margin-bottom:2.5rem;border-bottom:1px solid var(--border-soft)}
.hero-grid{display:grid;grid-template-columns:1fr;gap:1.5rem}
@media(min-width:800px){.hero-grid{grid-template-columns:1.35fr .85fr;align-items:start}}
.eyebrow{
  margin:0 0 .75rem;font-size:.78rem;font-weight:600;text-transform:uppercase;
  letter-spacing:.14em;color:var(--accent-dim);font-family:var(--font-mono);
}
.hero h1{
  font-size:clamp(1.85rem,4vw,2.65rem);line-height:1.12;margin:0 0 1rem;
}
.hero .tag,.page-header .tag{color:var(--muted);font-size:1.02rem;max-width:58ch;margin:0}
.tag em{color:var(--fg);font-style:normal;font-weight:500}
.install{margin:1.25rem 0 .5rem}
.install-label{
  display:block;font-size:.72rem;text-transform:uppercase;letter-spacing:.12em;
  color:var(--muted);margin-bottom:.35rem;font-weight:600;
}
.install code{
  display:block;background:var(--surface);border:1px solid var(--border);
  border-radius:var(--radius-sm);padding:.85rem 1rem;color:var(--code);
  overflow-x:auto;font-size:.86rem;box-shadow:inset 0 1px 0 rgba(255,255,255,.03);
}
.sub{color:var(--muted);font-size:.9rem;margin:.5rem 0 0}
.sub code{color:var(--accent);background:rgba(110,231,183,.08);padding:.1rem .35rem;border-radius:4px}
.hero-aside{display:flex;flex-direction:column;gap:.75rem}
.hero-link-card{
  display:block;background:var(--surface);border:1px solid var(--border);
  border-radius:var(--radius);padding:1rem 1.1rem;color:var(--fg);
  transition:border-color .2s,transform .2s,box-shadow .2s;
}
.hero-link-card:hover{
  border-color:rgba(110,231,183,.45);transform:translateY(-2px);
  box-shadow:var(--shadow);text-decoration:none;
}
.hero-link-kicker{display:block;font-size:.72rem;text-transform:uppercase;letter-spacing:.1em;color:var(--accent-dim);margin-bottom:.25rem}
.hero-link-card strong{display:block;font-family:var(--font-display);font-size:1rem;margin-bottom:.2rem}
.hero-link-meta{font-size:.85rem;color:var(--muted)}

/* —— Progress sync —— */
.progress-sync{
  margin:1.5rem 0 0;padding:0;background:var(--surface);
  border:1px solid var(--border);border-radius:var(--radius);overflow:hidden;
}
.progress-sync summary{
  list-style:none;cursor:pointer;padding:1rem 1.15rem;
  font-family:var(--font-display);font-size:.88rem;font-weight:600;color:var(--fg);
}
.progress-sync summary::-webkit-details-marker{display:none}
.progress-sync summary::after{content:"+";float:right;color:var(--muted);font-weight:400}
.progress-sync[open] summary::after{content:"−"}
.progress-sync[open] summary{border-bottom:1px solid var(--border-soft)}
.progress-sync-help,.progress-sync-actions,.progress-sync-status{padding:0 1.15rem}
.progress-sync-help{margin:.85rem 0 1rem;color:var(--muted);font-size:.9rem;line-height:1.55}
.progress-sync-actions{display:flex;flex-wrap:wrap;gap:.55rem;padding-bottom:1rem}
.progress-sync-status{min-height:1.1rem;padding-bottom:1rem;color:var(--link);font-size:.85rem}

/* —— Buttons —— */
.btn{
  display:inline-flex;align-items:center;justify-content:center;
  border-radius:var(--radius-sm);padding:.5rem 1rem;font-size:.88rem;
  font-weight:600;font-family:var(--font-ui);cursor:pointer;transition:all .15s;
  border:1px solid transparent;
}
.btn-primary{background:var(--accent);color:#0a1210;border-color:transparent}
.btn-primary:hover{filter:brightness(1.06);box-shadow:0 4px 20px rgba(110,231,183,.25)}
.btn-secondary{background:var(--bg-elevated);color:var(--fg);border-color:var(--border)}
.btn-secondary:hover{border-color:var(--link);background:var(--surface-hover)}
.btn-file{display:inline-flex}

/* —— Section labels —— */
.section-label{
  font-family:var(--font-display);font-size:.78rem;font-weight:600;
  text-transform:uppercase;letter-spacing:.14em;color:var(--accent-dim);
  margin:2.5rem 0 1rem;
}
.text-link{font-size:.88rem;color:var(--link)}

/* —— Cards & grid —— */
.grid{display:grid;grid-template-columns:1fr;gap:1rem}
@media(min-width:640px){.grid{grid-template-columns:1fr 1fr}}
.card{
  display:block;background:var(--surface);border:1px solid var(--border);
  border-radius:var(--radius);padding:1.15rem 1.25rem;
  transition:border-color .2s,transform .2s,box-shadow .2s,background .2s;
  box-shadow:0 1px 0 rgba(255,255,255,.02) inset;
}
.card:hover{
  border-color:rgba(142,180,255,.4);transform:translateY(-3px);
  background:var(--surface-hover);box-shadow:var(--shadow);text-decoration:none;
}
.card-top{display:flex;align-items:flex-start;justify-content:space-between;gap:.75rem;margin-bottom:.45rem}
.card h2,.card h3{margin:0;font-size:1.08rem;color:var(--fg);line-height:1.3}
.card p{margin:0 0 .85rem;color:var(--muted);font-size:.92rem;line-height:1.5}
.card .meta{color:var(--accent);font-size:.82rem;font-family:var(--font-mono)}
.card .meta [data-progress-label]:not(:empty)::after{content:" · ";color:var(--muted)}

/* —— Path sections —— */
.path-section{margin-bottom:3rem;padding-top:.5rem}
.path-section[data-path="durability"] .path-title a{color:var(--path-durability)}
.path-section[data-path="workflow"] .path-title a{color:var(--path-workflow)}
.path-section[data-path="distributed"] .path-title a{color:var(--path-distributed)}
.path-section[data-path="coordination"] .path-title a{color:var(--path-coordination)}
.path-head{margin-bottom:1.1rem;padding-left:.85rem;border-left:3px solid var(--border)}
.path-section[data-path="durability"] .path-head{border-left-color:var(--path-durability)}
.path-section[data-path="workflow"] .path-head{border-left-color:var(--path-workflow)}
.path-section[data-path="distributed"] .path-head{border-left-color:var(--path-distributed)}
.path-section[data-path="coordination"] .path-head{border-left-color:var(--path-coordination)}
.path-title{font-size:1.3rem;margin:0 0 .35rem}
.path-title a{color:var(--fg)}
.path-desc{margin:0;color:var(--muted);font-size:.94rem;max-width:58ch}
.path-crumb{color:var(--muted);font-size:.88rem;margin:.15rem 0 .65rem}
.path-challenges{list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:.75rem}
.path-card{display:flex;align-items:flex-start;gap:1rem}
.path-step{
  display:inline-flex;align-items:center;justify-content:center;
  min-width:2.1rem;height:2.1rem;background:var(--bg-elevated);
  border:1px solid var(--border);border-radius:50%;color:var(--accent);
  font-size:.88rem;font-family:var(--font-mono);flex-shrink:0;margin-top:.1rem;
}
.path-card-body{flex:1}

/* —— Roadmaps —— */
.roadmap-strip{margin-bottom:3rem}
.roadmap-strip-head{display:flex;align-items:baseline;justify-content:space-between;gap:1rem;margin-bottom:1rem}
.roadmap-cards{display:grid;grid-template-columns:1fr;gap:1rem}
@media(min-width:640px){.roadmap-cards{grid-template-columns:1fr 1fr}}
.roadmap-cards-page{grid-template-columns:1fr}
@media(min-width:720px){.roadmap-cards-page{grid-template-columns:1fr 1fr}}
.roadmap-card{
  display:block;background:linear-gradient(165deg,var(--surface) 0%,var(--bg-elevated) 100%);
  border:1px solid var(--border);border-radius:var(--radius);
  padding:1.15rem 1.2rem;color:var(--fg);
  transition:border-color .2s,transform .2s,box-shadow .2s;
}
.roadmap-card:hover{
  border-color:rgba(110,231,183,.35);transform:translateY(-3px);
  box-shadow:var(--shadow);text-decoration:none;
}
.roadmap-card h2,.roadmap-card h3{margin:.1rem 0 .45rem;font-size:1.05rem;color:var(--fg)}
.roadmap-card p{margin:0 0 .65rem;color:var(--muted);font-size:.9rem;line-height:1.45}
.roadmap-desc{font-size:.86rem}
.roadmap-meta{display:block;font-size:.8rem;color:var(--accent);font-family:var(--font-mono);margin-bottom:.55rem}
.roadmap-bar{display:block;height:5px;background:rgba(255,255,255,.06);border-radius:999px;overflow:hidden}
.roadmap-bar-lg{height:7px;margin-top:.4rem}
.roadmap-bar-fill{
  display:block;height:100%;width:0;
  background:linear-gradient(90deg,var(--accent),var(--link));
  border-radius:999px;transition:width .35s ease;
}
.roadmap-progress-head{margin-top:1.15rem;max-width:28rem}
.roadmap-progress-label{display:block;font-size:.86rem;color:var(--accent);font-family:var(--font-mono);margin-bottom:.35rem}
.roadmap-outcomes{
  margin:0 0 1.5rem;padding:1rem 1.15rem 1rem 2.5rem;
  background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);
  color:var(--muted);
}
.roadmap-outcomes li{margin:.4rem 0;line-height:1.5}
.roadmap-timeline{
  list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:.85rem;
  position:relative;
}
.roadmap-timeline::before{
  content:"";position:absolute;left:1.05rem;top:.5rem;bottom:.5rem;width:2px;
  background:linear-gradient(180deg,var(--accent-dim),var(--border));
  border-radius:999px;
}
.roadmap-milestone{position:relative;padding-left:0}

/* —— Page headers —— */
.back{margin:0 0 1.25rem;font-size:.88rem;color:var(--muted)}
.page-header{
  padding-bottom:1.5rem;margin-bottom:1.5rem;border-bottom:1px solid var(--border-soft);
}
.page-header h1{font-size:clamp(1.6rem,3vw,2.2rem);margin:.2rem 0 .65rem;line-height:1.15}
.page-header .card-top{align-items:center;margin-bottom:.35rem}
.page-header h1:only-child{margin-bottom:.65rem}

/* —— Stages list —— */
.stages{list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:.55rem}
.stage-link{
  display:flex;align-items:center;gap:.75rem;background:var(--surface);
  border:1px solid var(--border);border-radius:var(--radius-sm);
  padding:.75rem 1rem;color:var(--fg);
  transition:border-color .15s,background .15s,transform .15s;
}
.stage-link:hover{border-color:rgba(142,180,255,.45);background:var(--surface-hover);transform:translateX(3px);text-decoration:none}
.stage-name{flex:1;font-weight:600;font-size:.95rem}
.num{
  display:inline-flex;align-items:center;justify-content:center;
  min-width:1.75rem;height:1.75rem;background:var(--bg-elevated);
  border:1px solid var(--border);border-radius:50%;color:var(--accent);
  font-size:.8rem;font-family:var(--font-mono);flex-shrink:0;
}
.slug{margin-left:auto;color:var(--muted);font-size:.78rem;font-family:var(--font-mono)}
.diff{
  font-size:.68rem;font-weight:700;text-transform:uppercase;letter-spacing:.06em;
  padding:.15rem .5rem;border-radius:999px;border:1px solid currentColor;flex-shrink:0;
}
.diff-easy{color:#6ee7b7}.diff-medium{color:#fbbf24}.diff-hard{color:#f87171}
.badge{
  font-size:.62rem;font-weight:700;text-transform:uppercase;letter-spacing:.08em;
  padding:.2rem .55rem;border-radius:999px;flex-shrink:0;color:#0c0f16;
}
.badge.diff-easy{background:#6ee7b7}.badge.diff-medium{background:#fbbf24}.badge.diff-hard{background:#f87171}
.mix{display:flex;gap:.45rem;flex-wrap:wrap;margin:0 0 .75rem}
.mix .diff{border:none;padding:0;font-size:.72rem;background:none}

/* —— Markdown —— */
.md{padding:.2rem 0 1.1rem}
.md h1,.md h2,.md h3{font-family:var(--font-display);letter-spacing:-.02em;margin:1.4rem 0 .6rem}
.md h1{font-size:1.35rem}.md h2{font-size:1.12rem}.md h3{font-size:1rem}
.md p,.md li{color:var(--fg);line-height:1.65}
.md a{color:var(--link)}
.md code{background:rgba(255,255,255,.06);padding:.14em .4em;border-radius:5px;color:var(--code);font-size:.86em}
.md pre{background:#080b12;border:1px solid var(--border);border-radius:var(--radius-sm);padding:1rem;overflow-x:auto}
.md pre code{background:none;padding:0;color:var(--fg)}
.md table{border-collapse:collapse;width:100%;margin:1rem 0;font-size:.9rem}
.md th,.md td{border:1px solid var(--border);padding:.55rem .75rem;text-align:left}
.md th{background:var(--bg-elevated);color:var(--accent);font-family:var(--font-display);font-size:.82rem}
.md blockquote{border-left:3px solid var(--link);margin:1rem 0;padding:.2rem 1rem;color:var(--muted)}
.panel{background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);padding:1.1rem 1.25rem}
.protocol{padding:1rem 1.35rem 1.25rem}

/* —— Footer —— */
.site-footer{
  margin-top:3.5rem;padding-top:1.5rem;border-top:1px solid var(--border-soft);
  color:var(--muted);font-size:.86rem;
}

/* —— Stage layout —— */
.stage-grid{display:grid;grid-template-columns:1fr;gap:1.5rem}
@media(min-width:860px){.stage-grid{grid-template-columns:260px 1fr}}
.sidebar{
  background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);
  padding:1rem;position:sticky;top:4.5rem;align-self:start;
}
.sidebar-title{
  font-family:var(--font-display);font-size:.72rem;text-transform:uppercase;
  letter-spacing:.14em;color:var(--accent-dim);margin:0 0 .85rem;font-weight:600;
}
.sidebar-nav{display:flex;flex-direction:column;gap:.2rem}
.sidebar-item{
  display:flex;align-items:center;gap:.5rem;padding:.5rem .55rem;border-radius:var(--radius-sm);
  color:var(--muted);font-size:.86rem;transition:background .12s,color .12s;
}
.sidebar-item:hover{background:var(--surface-hover);color:var(--fg);text-decoration:none}
.sidebar-item.active{background:rgba(110,231,183,.08);color:var(--fg);border-left:2px solid var(--accent);padding-left:.4rem}
.sidebar-item .num{min-width:1.45rem;height:1.45rem;font-size:.72rem}
.sidebar-item .diff{font-size:.58rem;padding:0 .3rem}
.sidebar-protocol{display:block;margin-top:1rem;font-size:.84rem;color:var(--link)}
.stage-head{
  display:flex;align-items:center;gap:.7rem;flex-wrap:wrap;margin-bottom:1.1rem;
  padding-bottom:1rem;border-bottom:1px solid var(--border-soft);
}
.stage-head h1{font-size:1.45rem;margin:0;flex:1 1 100%;line-height:1.2}
.hint-box{
  background:rgba(142,180,255,.06);border:1px solid rgba(142,180,255,.2);
  border-left:3px solid var(--link);border-radius:var(--radius-sm);
  padding:.65rem 1rem;margin-bottom:1.2rem;
}
.hint-box summary{cursor:pointer;font-weight:600;color:var(--link);font-size:.9rem}
.hint-box p{margin:.55rem 0 0;color:var(--muted);font-size:.92rem}
.stage-pager{
  display:flex;justify-content:space-between;gap:1rem;margin-top:2rem;
  padding-top:1.25rem;border-top:1px solid var(--border-soft);
}
.pager{font-size:.9rem;font-weight:600;color:var(--link)}
.pager.next{margin-left:auto;text-align:right}

/* —— Progress states —— */
.progress-summary{min-height:1.2rem;color:var(--accent);font-size:.88rem;font-family:var(--font-mono);margin:.35rem 0}
.progress-read .num{border-color:var(--link);color:var(--link)}
.progress-passed .num{background:rgba(110,231,183,.15);border-color:var(--accent);color:var(--accent)}
.progress-passed.stage-link,.progress-passed.sidebar-item{border-color:rgba(110,231,183,.3);background:rgba(110,231,183,.04)}

/* —— Submit panel —— */
.submit-panel{margin-bottom:.5rem}
.panel-help{color:var(--muted);font-size:.9rem;margin:0 0 1rem;line-height:1.5}
.submit-panel .field{display:block;margin:.65rem 0;font-size:.88rem;color:var(--muted);font-weight:500}
.submit-panel input[type=file],.submit-panel input[type=password]{
  display:block;width:100%;margin-top:.35rem;background:var(--bg-elevated);
  border:1px solid var(--border);border-radius:var(--radius-sm);
  padding:.55rem .75rem;color:var(--fg);font-family:var(--font-ui);font-size:.9rem;
}
.submit-panel input:focus{outline:none;border-color:var(--link);box-shadow:0 0 0 3px rgba(142,180,255,.15)}
.submit-panel .check{display:flex;align-items:center;gap:.5rem;margin:.85rem 0;font-size:.88rem;color:var(--muted)}
.submit-status{color:var(--link);font-size:.88rem;margin:.75rem 0 .35rem;min-height:1.2rem}
.submit-log{
  background:#080b12;border:1px solid var(--border);border-radius:var(--radius-sm);
  padding:.85rem 1rem;max-height:16rem;overflow:auto;font-size:.76rem;
  color:var(--muted);white-space:pre-wrap;margin:0;display:none;
}
.submit-log:not(:empty){display:block}

@media(max-width:639px){
  .site-links{gap:.75rem;font-size:.82rem}
  .hero-aside{display:none}
}
`
