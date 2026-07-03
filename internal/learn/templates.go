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
	"diffBadge": difficultyBadgeClass,
	"diffPill":  difficultyPillClass,
}

func difficultyBadgeClass(d string) string {
	switch d {
	case "easy":
		return "shrink-0 rounded-full bg-emerald-300 px-2 py-0.5 text-[0.62rem] font-bold uppercase tracking-wider text-canvas"
	case "medium":
		return "shrink-0 rounded-full bg-amber-300 px-2 py-0.5 text-[0.62rem] font-bold uppercase tracking-wider text-canvas"
	case "hard":
		return "shrink-0 rounded-full bg-red-400 px-2 py-0.5 text-[0.62rem] font-bold uppercase tracking-wider text-canvas"
	default:
		return "shrink-0 rounded-full bg-muted px-2 py-0.5 text-[0.62rem] font-bold uppercase text-canvas"
	}
}

func difficultyPillClass(d string) string {
	switch d {
	case "easy":
		return "shrink-0 rounded-full border border-emerald-300 px-2 py-0.5 text-[0.68rem] font-bold uppercase tracking-wide text-emerald-300"
	case "medium":
		return "shrink-0 rounded-full border border-amber-300 px-2 py-0.5 text-[0.68rem] font-bold uppercase tracking-wide text-amber-300"
	case "hard":
		return "shrink-0 rounded-full border border-red-400 px-2 py-0.5 text-[0.68rem] font-bold uppercase tracking-wide text-red-400"
	default:
		return "shrink-0 rounded-full border border-muted px-2 py-0.5 text-[0.68rem] font-bold uppercase text-muted"
	}
}

const fontLinksHTML = `<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,400;0,9..40,500;0,9..40,600;1,9..40,400&family=JetBrains+Mono:wght@400;500;600&family=Sora:wght@500;600;700&display=swap" rel="stylesheet">`

const siteNavHTML = `<nav class="sticky top-0 z-50 border-b border-border-soft bg-canvas/80 backdrop-blur-md" aria-label="Main">
  <div class="mx-auto flex max-w-[1120px] items-center justify-between gap-4 px-5 py-3.5">
    <a class="inline-flex items-center gap-1.5 font-display text-[0.95rem] font-semibold text-ink hover:text-accent" href="/">
      <span class="font-mono font-medium text-accent">$</span> open-crafters
      <span class="rounded-full border border-accent/20 bg-accent/10 px-1.5 py-0.5 text-[0.62rem] font-bold uppercase tracking-widest text-accent-dim">learn</span>
    </a>
    <div class="flex items-center gap-4 text-sm text-muted">
      <a class="hover:text-ink" href="/roadmaps">Roadmaps</a>
      <a class="hover:text-ink" href="https://runner.gilla.fun">Runner</a>
      <a class="hover:text-ink" href="https://github.com/Rohithgilla12/open-crafters">GitHub</a>
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
<div class="mx-auto max-w-[920px] px-5 py-8 pb-16">
<header class="mb-10 border-b border-border-soft pb-8">
  <div class="grid grid-cols-1 gap-6 lg:grid-cols-[1.35fr_0.85fr] lg:items-start">
  <div>
  <p class="mb-3 font-mono text-xs font-semibold uppercase tracking-widest text-accent-dim">14 challenges · 4 paths · graded black-box</p>
  <h1 class="mb-4 text-[clamp(1.85rem,4vw,2.65rem)] leading-[1.12]">Build the infrastructure<br>senior engineers actually ship.</h1>
  <p class="max-w-[58ch] text-[1.02rem] text-muted">Open-source <em class="font-medium text-ink not-italic">build-your-own-X</em> challenges for production primitives.
  Read each stage, implement in any language, and grade over the wire — crashes included.</p>
  <div class="mt-5 mb-2">
    <span class="mb-1.5 block text-xs font-semibold uppercase tracking-widest text-muted">Install</span>
    <code class="block overflow-x-auto rounded-[10px] border border-border bg-surface px-4 py-3.5 font-mono text-[0.86rem] text-code shadow-[inset_0_1px_0_rgba(255,255,255,0.03)]">curl -fsSL https://raw.githubusercontent.com/Rohithgilla12/open-crafters/main/install.sh | sh</code>
  </div>
  <p class="mt-2 text-sm text-muted">then <code class="rounded bg-accent/10 px-1.5 py-0.5 text-accent">crafters start wal</code> locally, or submit to the
  <a class="text-link hover:text-link/80" href="https://runner.gilla.fun">hosted runner</a></p>
  </div>
  <aside class="hidden flex-col gap-3 sm:flex">
    <a class="block rounded-[14px] border border-border bg-surface p-4 text-ink transition hover:-translate-y-0.5 hover:border-accent/45 hover:shadow-[0_12px_40px_rgba(0,0,0,0.35)]" href="/roadmaps">
      <span class="mb-1 block text-xs uppercase tracking-widest text-accent-dim">Start here</span>
      <strong class="block font-display text-base">Learning roadmaps</strong>
      <span class="text-sm text-muted">Curated journeys with outcomes →</span>
    </a>
    <a class="block rounded-[14px] border border-border bg-surface p-4 text-ink transition hover:-translate-y-0.5 hover:border-accent/45 hover:shadow-[0_12px_40px_rgba(0,0,0,0.35)]" href="https://runner.gilla.fun">
      <span class="mb-1 block text-xs uppercase tracking-widest text-accent-dim">Remote grading</span>
      <strong class="block font-display text-base">Hosted runner</strong>
      <span class="text-sm text-muted">Submit zips from the browser →</span>
    </a>
  </aside>
  </div>
  <details class="progress-sync mt-6 overflow-hidden rounded-[14px] border border-border bg-surface">
    <summary class="cursor-pointer px-4 py-4 font-display text-sm font-semibold text-ink">Progress sync</summary>
    <p class="progress-sync-help px-4 text-sm leading-relaxed text-muted">Sync with local <code class="rounded bg-white/6 px-1 text-code">crafters test</code> via <code class="rounded bg-white/6 px-1 text-code">progress.json</code> —
    export from the CLI (<code class="rounded bg-white/6 px-1 text-code">crafters progress export</code>) and import here, or export to back up browser progress.</p>
    <div class="flex flex-wrap gap-2 px-4 pb-4">
      <button type="button" class="inline-flex cursor-pointer items-center justify-center rounded-[10px] border border-border bg-canvas-elevated px-4 py-2 text-sm font-semibold text-ink transition hover:border-link hover:bg-surface-hover" id="progress-export">Export progress.json</button>
      <label class="inline-flex cursor-pointer items-center justify-center rounded-[10px] border border-border bg-canvas-elevated px-4 py-2 text-sm font-semibold text-ink transition hover:border-link hover:bg-surface-hover">Import progress.json
        <input type="file" id="progress-import" accept="application/json,.json" hidden>
      </label>
    </div>
    <p id="progress-sync-status" class="min-h-[1.1rem] px-4 pb-4 text-sm text-link" aria-live="polite"></p>
  </details>
</header>
<section class="mb-12">
  <div class="mb-4 flex items-baseline justify-between gap-4">
    <h2 class="font-display text-xs font-semibold uppercase tracking-widest text-accent-dim">Learning roadmaps</h2>
    <a class="text-sm text-link" href="/roadmaps">View all →</a>
  </div>
  <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
  {{range .Roadmaps}}
    <a class="block rounded-[14px] border border-border bg-gradient-to-br from-surface to-canvas-elevated p-5 text-ink transition hover:-translate-y-1 hover:border-accent/35 hover:shadow-[0_12px_40px_rgba(0,0,0,0.35)]" href="/roadmaps/{{.Slug}}" data-roadmap-card data-challenges="{{.ChallengeCSV}}" data-total-stages="{{.TotalStages}}">
      <h3 class="mb-1.5 text-[1.05rem] font-semibold">{{.Name}}</h3>
      <p class="mb-2.5 text-sm leading-snug text-muted">{{.Tagline}}</p>
      <span class="mb-1.5 block font-mono text-xs text-accent"><span data-roadmap-progress-label></span>{{.TotalStages}} stages</span>
      <span class="roadmap-bar block h-1.5 overflow-hidden rounded-full bg-white/6"><span class="roadmap-bar-fill"></span></span>
    </a>
  {{end}}
  </div>
</section>
{{range .Paths}}
<section class="path-section mb-12 pt-2" id="path-{{.Slug}}" data-path="{{.Slug}}">
  <div class="path-accent mb-4 border-l-[3px] border-border pl-3.5">
    <h2 class="mb-1 text-xl"><a class="path-title-link hover:underline" href="/roadmaps/{{.Slug}}">{{.Name}}</a></h2>
    <p class="max-w-[58ch] text-[0.94rem] text-muted">{{.Description}}</p>
  </div>
  <main class="grid grid-cols-1 gap-4 sm:grid-cols-2">
  {{range .Challenges}}
    <a class="block rounded-[14px] border border-border bg-surface p-5 text-ink shadow-[inset_0_1px_0_rgba(255,255,255,0.02)] transition hover:-translate-y-1 hover:border-link/40 hover:bg-surface-hover hover:shadow-[0_12px_40px_rgba(0,0,0,0.35)]" href="/challenges/{{.Slug}}" data-challenge="{{.Slug}}" data-stages="{{stagesCSV .Stages}}">
      <div class="mb-1 flex items-start justify-between gap-3">
        <h3 class="text-[1.08rem] leading-snug font-semibold">{{.Name}}</h3>
        <span class="{{diffBadge .Difficulty}}">{{.Difficulty}}</span>
      </div>
      <p class="mb-3 text-sm leading-relaxed text-muted">{{.Tagline}}</p>
      <div class="mb-3 flex flex-wrap gap-1.5">{{.DiffMix}}</div>
      <span class="challenge-meta font-mono text-xs text-accent"><span data-progress-label></span>{{len .Stages}} stages →</span>
    </a>
  {{end}}
  </main>
</section>
{{end}}
<footer class="mt-14 border-t border-border-soft pt-6 text-sm text-muted">
  <a class="text-link" href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> ·
  <a class="text-link" href="https://runner.gilla.fun">hosted runner</a> ·
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
<div class="mx-auto max-w-[920px] px-5 py-8 pb-16">
<p class="mb-5 text-sm text-muted"><a class="text-link" href="/">← home</a></p>
<header class="mb-6 border-b border-border-soft pb-6">
  <h1 class="mb-2 text-[clamp(1.6rem,3vw,2.2rem)] leading-tight">Learning roadmaps</h1>
  <p class="max-w-[58ch] text-muted">Curated journeys through the catalog — each with outcomes, milestones, and suggested order.</p>
</header>
<div class="grid grid-cols-1 gap-4 md:grid-cols-2">
{{range .}}
  <a class="block rounded-[14px] border border-border bg-gradient-to-br from-surface to-canvas-elevated p-5 text-ink transition hover:-translate-y-1 hover:border-accent/35 hover:shadow-[0_12px_40px_rgba(0,0,0,0.35)]" href="/roadmaps/{{.Slug}}" data-roadmap-card data-challenges="{{.ChallengeCSV}}" data-total-stages="{{.TotalStages}}">
    <h2 class="mb-1.5 text-[1.05rem] font-semibold">{{.Name}}</h2>
    <p class="mb-2 text-sm text-muted">{{.Tagline}}</p>
    <p class="mb-2 text-[0.86rem] text-muted">{{.Description}}</p>
    <span class="mb-1.5 block font-mono text-xs text-accent"><span data-roadmap-progress-label></span>{{.TotalStages}} stages · {{len .Milestones}} milestones</span>
    <span class="roadmap-bar block h-1.5 overflow-hidden rounded-full bg-white/6"><span class="roadmap-bar-fill"></span></span>
  </a>
{{end}}
</div>
<footer class="mt-14 border-t border-border-soft pt-6 text-sm text-muted"><a class="text-link" href="/">← home</a></footer>
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
<div class="mx-auto max-w-[920px] px-5 py-8 pb-16">
<p class="mb-5 text-sm text-muted"><a class="text-link" href="/roadmaps">← all roadmaps</a></p>
<header class="mb-6 border-b border-border-soft pb-6">
  <p class="mb-2 font-mono text-xs font-semibold uppercase tracking-widest text-accent-dim">Roadmap</p>
  <h1 class="mb-2 text-[clamp(1.6rem,3vw,2.2rem)] leading-tight">{{.Name}}</h1>
  <p class="max-w-[58ch] text-muted">{{.Description}}</p>
  {{if .StartCommand}}<div class="mt-5 mb-2">
    <span class="mb-1.5 block text-xs font-semibold uppercase tracking-widest text-muted">Start</span>
    <code class="block overflow-x-auto rounded-[10px] border border-border bg-surface px-4 py-3.5 font-mono text-[0.86rem] text-code">{{.StartCommand}}</code>
  </div>{{end}}
  <div class="mt-4 max-w-md" data-roadmap-bar data-total-stages="{{.TotalStages}}">
    <span data-roadmap-progress-label class="mb-1 block font-mono text-sm text-accent">0/{{.TotalStages}} stages</span>
    <span class="roadmap-bar block h-1.5 overflow-hidden rounded-full bg-white/6"><span class="roadmap-bar-fill"></span></span>
  </div>
</header>
{{if .Outcomes}}
<h2 class="mb-4 font-display text-xs font-semibold uppercase tracking-widest text-accent-dim">What you'll learn</h2>
<ul class="mb-6 rounded-[14px] border border-border bg-surface py-3 pl-10 pr-4 text-muted">
{{range .Outcomes}}<li class="my-1.5 leading-relaxed">{{.}}</li>{{end}}
</ul>
{{end}}
<h2 class="mb-4 font-display text-xs font-semibold uppercase tracking-widest text-accent-dim">Milestones</h2>
<ol class="roadmap-timeline relative flex flex-col gap-3.5">
{{range .Milestones}}
  <li class="roadmap-milestone">
    {{if .PathSlug}}
    <a class="flex items-start gap-4 rounded-[14px] border border-border bg-surface p-5 text-ink transition hover:-translate-y-0.5 hover:border-link/40 hover:bg-surface-hover hover:shadow-[0_12px_40px_rgba(0,0,0,0.35)]" href="/roadmaps/{{.PathSlug}}" data-roadmap-milestone>
      <span class="num flex h-8 min-w-8 shrink-0 items-center justify-center rounded-full border border-border bg-canvas-elevated font-mono text-sm text-accent">{{.Num}}</span>
      <div class="flex-1">
        <h2 class="mb-1 text-[1.08rem] font-semibold">{{.PathName}}</h2>
        <p class="mb-2 text-sm text-muted">{{.Blurb}}</p>
        <span class="font-mono text-xs text-accent">Open roadmap →</span>
      </div>
    </a>
    {{else}}
    <a class="flex items-start gap-4 rounded-[14px] border border-border bg-surface p-5 text-ink transition hover:-translate-y-0.5 hover:border-link/40 hover:bg-surface-hover hover:shadow-[0_12px_40px_rgba(0,0,0,0.35)]" href="/challenges/{{.Challenge.Slug}}" data-challenge="{{.Challenge.Slug}}" data-stages="{{.StageCSV}}" data-roadmap-milestone>
      <span class="num flex h-8 min-w-8 shrink-0 items-center justify-center rounded-full border border-border bg-canvas-elevated font-mono text-sm text-accent">{{.Num}}</span>
      <div class="flex-1">
        <h2 class="mb-1 flex flex-wrap items-center gap-2 text-[1.08rem] font-semibold">{{.Challenge.Name}} <span class="{{diffBadge .Challenge.Difficulty}}">{{.Challenge.Difficulty}}</span></h2>
        <p class="mb-2 text-sm text-muted">{{.Blurb}}</p>
        <span class="challenge-meta font-mono text-xs text-accent"><span data-progress-label></span>{{len .Challenge.Stages}} stages →</span>
      </div>
    </a>
    {{end}}
  </li>
{{end}}
</ol>
<footer class="mt-14 border-t border-border-soft pt-6 text-sm text-muted"><a class="text-link" href="/roadmaps">← all roadmaps</a></footer>
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
<div class="mx-auto max-w-[920px] px-5 py-8 pb-16">
<p class="mb-5 text-sm text-muted"><a class="text-link" href="/">← all paths</a></p>
<header class="mb-6 border-b border-border-soft pb-6">
  <h1 class="mb-2 text-[clamp(1.6rem,3vw,2.2rem)] leading-tight">{{.Name}}</h1>
  <p class="max-w-[58ch] text-muted">{{.Description}}</p>
</header>
<ol class="flex flex-col gap-3">
{{range $i, $ch := .Challenges}}
  <li>
    <a class="flex items-start gap-4 rounded-[14px] border border-border bg-surface p-5 text-ink transition hover:-translate-y-0.5 hover:border-link/40 hover:bg-surface-hover" href="/challenges/{{$ch.Slug}}" data-challenge="{{$ch.Slug}}" data-stages="{{stagesCSV $ch.Stages}}">
      <span class="num flex h-8 min-w-8 shrink-0 items-center justify-center rounded-full border border-border bg-canvas-elevated font-mono text-sm text-accent">{{add $i 1}}</span>
      <div class="flex-1">
        <h2 class="mb-1 flex flex-wrap items-center gap-2 text-[1.08rem] font-semibold">{{$ch.Name}} <span class="{{diffBadge $ch.Difficulty}}">{{$ch.Difficulty}}</span></h2>
        <p class="mb-2 text-sm text-muted">{{$ch.Tagline}}</p>
        <span class="challenge-meta font-mono text-xs text-accent"><span data-progress-label></span>{{len $ch.Stages}} stages →</span>
      </div>
    </a>
  </li>
{{end}}
</ol>
<footer class="mt-14 border-t border-border-soft pt-6 text-sm text-muted"><a class="text-link" href="/">← all paths</a> · <a class="text-link" href="https://github.com/Rohithgilla12/open-crafters">GitHub</a></footer>
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
<div class="mx-auto max-w-[920px] px-5 py-8 pb-16">
<p class="mb-5 text-sm text-muted"><a class="text-link" href="/">← all challenges</a>{{if .RoadmapSlug}} · <a class="text-link" href="/roadmaps/{{.RoadmapSlug}}">{{.RoadmapName}}</a>{{end}}</p>
<header class="mb-6 border-b border-border-soft pb-6">
  <div class="mb-1 flex items-center gap-3">
    <h1 class="text-[clamp(1.6rem,3vw,2.2rem)] leading-tight">{{.Challenge.Name}}</h1>
    <span class="{{diffBadge .Challenge.Difficulty}}">{{.Challenge.Difficulty}}</span>
  </div>
  {{if .RoadmapName}}<p class="mb-2 text-sm text-muted">Roadmap: <a class="text-link" href="/roadmaps/{{.RoadmapSlug}}">{{.RoadmapName}}</a></p>{{end}}
  <p class="max-w-[58ch] text-muted">{{.Challenge.Tagline}}</p>
  <p class="my-1 min-h-[1.2rem] font-mono text-sm text-accent"><span data-progress-label></span></p>
  <div class="mt-4">
    <span class="mb-1.5 block text-xs font-semibold uppercase tracking-widest text-muted">Local start</span>
    <code class="block overflow-x-auto rounded-[10px] border border-border bg-surface px-4 py-3.5 font-mono text-[0.86rem] text-code">crafters start {{.Challenge.Slug | short}}</code>
  </div>
</header>

<h2 class="mb-4 font-display text-xs font-semibold uppercase tracking-widest text-accent-dim">Submit to hosted runner</h2>
<div class="submit-panel mb-2 rounded-[14px] border border-border bg-surface p-5">
  <p class="mb-4 text-sm leading-relaxed text-muted">Zip your solution directory (must include <code class="rounded bg-white/6 px-1 text-code">your_program.sh</code>) and grade remotely — same as <code class="rounded bg-white/6 px-1 text-code">crafters submit</code>.</p>
  <form id="submit-form" data-challenge="{{.Challenge.Slug}}">
    <label class="mb-2.5 block text-sm font-medium text-muted">Runner token <input class="mt-1.5 block w-full rounded-[10px] border border-border bg-canvas-elevated px-3 py-2.5 text-ink focus:border-link focus:outline-none focus:ring-[3px] focus:ring-link/15" type="password" name="token" autocomplete="off" placeholder="from your runner dashboard"></label>
    <label class="mb-2.5 block text-sm font-medium text-muted">Solution zip <input class="mt-1.5 block w-full rounded-[10px] border border-border bg-canvas-elevated px-3 py-2.5 text-ink focus:border-link focus:outline-none focus:ring-[3px] focus:ring-link/15" type="file" name="file" accept=".zip,application/zip"></label>
    <label class="my-3 flex items-center gap-2 text-sm text-muted"><input type="checkbox" name="all"> Grade all stages</label>
    <button type="submit" class="inline-flex cursor-pointer items-center justify-center rounded-[10px] bg-accent px-4 py-2.5 text-sm font-bold text-[#0a1210] transition hover:brightness-105 hover:shadow-[0_4px_20px_rgba(110,231,183,0.25)]">Submit for grading</button>
  </form>
  <p id="submit-status" class="submit-status mt-3 min-h-[1.2rem] text-sm text-link" aria-live="polite"></p>
  <pre id="submit-log" class="submit-log mt-2 max-h-64 overflow-auto rounded-[10px] border border-border bg-[#080b12] p-4 font-mono text-xs whitespace-pre-wrap text-muted"></pre>
</div>

<h2 class="mb-4 font-display text-xs font-semibold uppercase tracking-widest text-accent-dim">Stages</h2>
<ol class="flex flex-col gap-2">
{{range .Challenge.Stages}}
  <li>
    <a class="stage-link flex items-center gap-3 rounded-[10px] border border-border bg-surface px-4 py-3 text-ink transition hover:translate-x-0.5 hover:border-link/45 hover:bg-surface-hover" href="/challenges/{{$.Challenge.Slug}}/stages/{{.Slug}}" data-stage-slug="{{.Slug}}">
      <span class="num flex h-7 min-w-7 shrink-0 items-center justify-center rounded-full border border-border bg-canvas-elevated font-mono text-xs text-accent">{{.Num}}</span>
      <span class="stage-name flex-1 text-[0.95rem] font-semibold">{{.Name}}</span>
      <span class="{{diffPill .Difficulty}}">{{.Difficulty}}</span>
      <span class="slug ml-auto font-mono text-xs text-muted">{{.Slug}}</span>
    </a>
  </li>
{{end}}
</ol>

<h2 class="mb-4 mt-10 font-display text-xs font-semibold uppercase tracking-widest text-accent-dim" id="protocol">Protocol</h2>
<div class="md protocol rounded-[14px] border border-border bg-surface px-5 py-4">{{.Challenge.ProtocolHTML}}</div>

<footer class="mt-14 border-t border-border-soft pt-6 text-sm text-muted"><a class="text-link" href="/">← all challenges</a> · <a class="text-link" href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> · <a class="text-link" href="https://runner.gilla.fun">hosted runner</a></footer>
</div><script src="/learn.js"></script></body></html>`))

var stageTmpl = template.Must(template.New("stage").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Stage.Name}} — {{.Challenge.Name}} — open-crafters learn</title>
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
{{fontLinks}}
<link rel="stylesheet" href="/style.css">
</head><body data-challenge="{{.Challenge.Slug}}" data-stage="{{.Stage.Slug}}" data-stages="{{.StageSlugs}}">
{{siteNav}}
<div class="mx-auto max-w-[1120px] px-5 py-8 pb-16">
<p class="mb-5 text-sm text-muted"><a class="text-link" href="/challenges/{{.Challenge.Slug}}">← {{.Challenge.Name}}</a></p>

<div class="grid grid-cols-1 gap-6 lg:grid-cols-[260px_1fr]">
<aside class="sticky top-[4.5rem] self-start rounded-[14px] border border-border bg-surface p-4">
  <h2 class="mb-3 font-display text-xs font-semibold uppercase tracking-widest text-accent-dim">Stages</h2>
  <nav class="flex flex-col gap-0.5">
  {{range .Challenge.Stages}}
    <a class="sidebar-item flex items-center gap-2 rounded-[10px] px-2 py-2 text-sm text-muted transition hover:bg-surface-hover hover:text-ink{{if eq .Slug $.Stage.Slug}} active border-l-2 border-accent bg-accent/8 pl-1.5 text-ink{{end}}" href="/challenges/{{$.Challenge.Slug}}/stages/{{.Slug}}" data-stage-slug="{{.Slug}}">
      <span class="num flex h-[1.45rem] min-w-[1.45rem] items-center justify-center rounded-full border border-border bg-canvas-elevated font-mono text-[0.72rem] text-accent">{{.Num}}</span>
      <span class="min-w-0 flex-1 truncate">{{.Name}}</span>
      <span class="{{diffPill .Difficulty}} !px-1 !text-[0.58rem]">{{.Difficulty}}</span>
    </a>
  {{end}}
  </nav>
  <a class="mt-4 block text-sm text-link" href="/challenges/{{.Challenge.Slug}}#protocol">Protocol spec ↓</a>
</aside>

<main>
  <header class="mb-4 flex flex-wrap items-center gap-2.5 border-b border-border-soft pb-4">
    <h1 class="w-full text-2xl leading-tight"><span class="num mr-2 inline-flex h-7 min-w-7 items-center justify-center rounded-full border border-border bg-canvas-elevated font-mono text-xs text-accent">{{.Stage.Num}}</span>{{.Stage.Name}}</h1>
    <span class="{{diffPill .Stage.Difficulty}}">{{.Stage.Difficulty}}</span>
    <span class="slug mono font-mono text-xs text-muted">{{.Stage.Slug}}</span>
  </header>
  {{if .Stage.Hint}}
  <details class="mb-5 rounded-[10px] border border-link/20 border-l-[3px] border-l-link bg-link/6 px-4 py-3">
    <summary class="cursor-pointer text-sm font-semibold text-link">Stuck? Here's a nudge</summary>
    <p class="mt-2 text-sm text-muted">{{.Stage.Hint}}</p>
  </details>
  {{end}}
  <div class="md">{{.Stage.HTML}}</div>
  <nav class="mt-8 flex justify-between gap-4 border-t border-border-soft pt-5">
    {{if .Prev}}<a class="text-sm font-semibold text-link" href="/challenges/{{.Challenge.Slug}}/stages/{{.Prev.Slug}}">← {{.Prev.Name}}</a>{{else}}<span></span>{{end}}
    {{if .Next}}<a class="ml-auto text-right text-sm font-semibold text-link" href="/challenges/{{.Challenge.Slug}}/stages/{{.Next.Slug}}">{{.Next.Name}} →</a>{{end}}
  </nav>
</main>
</div>

<footer class="mt-14 border-t border-border-soft pt-6 text-sm text-muted"><a class="text-link" href="/">← all challenges</a> · <a class="text-link" href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> · <a class="text-link" href="https://runner.gilla.fun">hosted runner</a></footer>
</div><script src="/learn.js"></script></body></html>`))
