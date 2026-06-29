package learn

import "html/template"

var tmplFuncs = template.FuncMap{
	"short": shortSlug,
}

var indexTmpl = template.Must(template.New("index").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>open-crafters — learn</title>
<meta name="description" content="Build-your-own-X challenges for production infrastructure primitives. Read stages, study the protocol, then implement and grade over the wire.">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<link rel="stylesheet" href="/style.css">
</head><body><div class="wrap">
<header class="hero">
  <h1><span class="prompt">$</span> open-crafters <span class="learn-badge">learn</span></h1>
  <p class="tag">Open-source <em>build-your-own-X</em> challenges for the production-infrastructure
  primitives senior engineers actually wrestle with. Read each stage, implement in any language,
  and grade black-box over the wire — crashes included.</p>
  <div class="install">
    <code>curl -fsSL https://raw.githubusercontent.com/Rohithgilla12/open-crafters/main/install.sh | sh</code>
  </div>
  <p class="sub">then <code>crafters start {{(index . 0).Slug | short}}</code> locally, or submit to the
  <a href="https://runner.gilla.fun">hosted runner</a></p>
</header>
<main class="grid">
{{range .}}
  <a class="card" href="/challenges/{{.Slug}}">
    <h2>{{.Name}} <span class="badge diff-{{.Difficulty}}">{{.Difficulty}}</span></h2>
    <p>{{.Tagline}}</p>
    <div class="mix">{{.DiffMix}}</div>
    <span class="meta">{{len .Stages}} stages →</span>
  </a>
{{end}}
</main>
<footer>
  <a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> ·
  <a href="https://runner.gilla.fun">hosted runner</a> ·
  graded black-box · any language with a TCP socket
</footer>
</div></body></html>`))

var challengeTmpl = template.Must(template.New("challenge").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Name}} — open-crafters learn</title>
<meta name="description" content="{{.Tagline}}">
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<link rel="stylesheet" href="/style.css">
</head><body><div class="wrap">
<p class="back"><a href="/">← all challenges</a></p>
<header class="chead">
  <h1>{{.Name}} <span class="badge diff-{{.Difficulty}}">{{.Difficulty}}</span></h1>
  <p class="tag">{{.Tagline}}</p>
  <div class="install"><code>crafters start {{.Slug | short}}</code></div>
  <p class="sub">Submit remotely via <code>crafters submit --url https://runner.gilla.fun</code></p>
</header>

<h2 class="section">Stages</h2>
<ol class="stages">
{{range .Stages}}
  <li>
    <a class="stage-link" href="/challenges/{{$.Slug}}/stages/{{.Slug}}">
      <span class="num">{{.Num}}</span>
      <span class="stage-name">{{.Name}}</span>
      <span class="diff diff-{{.Difficulty}}">{{.Difficulty}}</span>
      <span class="slug">{{.Slug}}</span>
    </a>
  </li>
{{end}}
</ol>

<h2 class="section">Protocol</h2>
<div class="md protocol">{{.ProtocolHTML}}</div>

<footer><a href="/">← all challenges</a> · <a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> · <a href="https://runner.gilla.fun">hosted runner</a></footer>
</div></body></html>`))

var stageTmpl = template.Must(template.New("stage").Funcs(tmplFuncs).Parse(`<!doctype html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Stage.Name}} — {{.Challenge.Name}} — open-crafters learn</title>
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="apple-touch-icon" href="/apple-touch-icon.png">
<link rel="stylesheet" href="/style.css">
</head><body><div class="wrap stage-layout">
<p class="back"><a href="/challenges/{{.Challenge.Slug}}">← {{.Challenge.Name}}</a></p>

<div class="stage-grid">
<aside class="sidebar">
  <h2 class="sidebar-title">Stages</h2>
  <nav class="sidebar-nav">
  {{range .Challenge.Stages}}
    <a class="sidebar-item{{if eq .Slug $.Stage.Slug}} active{{end}}" href="/challenges/{{$.Challenge.Slug}}/stages/{{.Slug}}">
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

<footer><a href="/">← all challenges</a> · <a href="https://github.com/Rohithgilla12/open-crafters">GitHub</a> · <a href="https://runner.gilla.fun">hosted runner</a></footer>
</div></body></html>`))

// siteCSS matches the aesthetic from cmd/crafters/site.go with learn-app additions.
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
.learn-badge{font-size:.55em;font-weight:700;text-transform:uppercase;letter-spacing:.14em;
  color:var(--accent);vertical-align:middle;margin-left:.4rem}
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
.stage-link{display:flex;align-items:center;gap:.7rem;background:var(--panel);border:1px solid var(--border);
  border-radius:10px;padding:.8rem 1rem;color:var(--fg);transition:border-color .15s,background .15s}
.stage-link:hover{border-color:var(--accent2);background:var(--panel2);text-decoration:none}
.stage-name{flex:1;font-weight:600}
.num{display:inline-flex;align-items:center;justify-content:center;min-width:1.7rem;height:1.7rem;
  background:var(--panel2);border:1px solid var(--border);border-radius:50%;color:var(--accent);
  font-size:.82rem;font-family:ui-monospace,monospace;flex-shrink:0}
.slug{margin-left:auto;color:var(--dim);font-size:.8rem;font-family:ui-monospace,monospace}
.diff{font-size:.7rem;font-weight:600;text-transform:uppercase;letter-spacing:.05em;
  padding:.1rem .45rem;border-radius:999px;border:1px solid currentColor;flex-shrink:0}
.diff-easy{color:#5ad1b3}.diff-medium{color:#e0c45a}.diff-hard{color:#e57a86}
.badge{font-size:.62rem;font-weight:700;text-transform:uppercase;letter-spacing:.08em;
  padding:.18rem .5rem;border-radius:999px;vertical-align:middle;color:#0b0e14}
.badge.diff-easy{background:#5ad1b3}.badge.diff-medium{background:#e0c45a}.badge.diff-hard{background:#e57a86}
.mix{display:flex;gap:.4rem;flex-wrap:wrap;margin:0 0 .9rem}
.mix .diff{border:none;padding:0;font-size:.72rem}
.md{padding:.2rem 0 1.1rem}
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
.wrap.stage-layout{max-width:1100px}
.stage-grid{display:grid;grid-template-columns:1fr;gap:1.5rem}
@media(min-width:800px){.stage-grid{grid-template-columns:240px 1fr}}
.sidebar{background:var(--panel);border:1px solid var(--border);border-radius:12px;padding:1rem;
  position:sticky;top:1rem;align-self:start}
.sidebar-title{font-size:.75rem;text-transform:uppercase;letter-spacing:.12em;color:var(--accent);
  margin:0 0 .8rem}
.sidebar-nav{display:flex;flex-direction:column;gap:.25rem}
.sidebar-item{display:flex;align-items:center;gap:.5rem;padding:.45rem .5rem;border-radius:8px;
  color:var(--dim);font-size:.88rem;transition:background .12s,color .12s}
.sidebar-item:hover{background:var(--panel2);color:var(--fg);text-decoration:none}
.sidebar-item.active{background:var(--panel2);color:var(--fg);border-left:2px solid var(--accent)}
.sidebar-item .num{min-width:1.4rem;height:1.4rem;font-size:.75rem}
.sidebar-item .diff{font-size:.6rem;padding:0 .3rem}
.sidebar-protocol{display:block;margin-top:1rem;font-size:.85rem;color:var(--accent)}
.stage-head{display:flex;align-items:center;gap:.7rem;flex-wrap:wrap;margin-bottom:1rem;
  padding-bottom:1rem;border-bottom:1px solid var(--border)}
.stage-head h1{font-size:1.5rem;margin:0;flex:1 1 100%}
.hint-box{background:var(--panel);border:1px solid var(--border);border-left:3px solid var(--accent2);
  border-radius:10px;padding:.6rem 1rem;margin-bottom:1.2rem}
.hint-box summary{cursor:pointer;font-weight:600;color:var(--accent2);font-size:.92rem}
.hint-box p{margin:.6rem 0 0;color:var(--dim);font-size:.93rem}
.stage-pager{display:flex;justify-content:space-between;gap:1rem;margin-top:2rem;
  padding-top:1.2rem;border-top:1px solid var(--border)}
.pager{font-size:.92rem;font-weight:600}
.pager.next{margin-left:auto;text-align:right}
`
