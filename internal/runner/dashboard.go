package runner

import (
	"html/template"
	"net/http"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = dashboardTmpl.Execute(w, map[string]any{
		"Authed": s.authorize(r),
		"Token":  r.URL.Query().Get("token"),
	})
}

var dashboardTmpl = template.Must(template.New("dashboard").Parse(dashboardHTML))

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>open-crafters runner</title>
<style>
:root{color-scheme:dark;--bg:#0d1117;--panel:#161b22;--border:#30363d;--text:#e6edf3;--muted:#8b949e;--green:#3fb950;--red:#f85149;--yellow:#d29922;--blue:#58a6ff}
*{box-sizing:border-box}body{margin:0;font:15px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Helvetica,Arial,sans-serif;background:var(--bg);color:var(--text)}
.wrap{max-width:960px;margin:0 auto;padding:24px 16px 48px}
header{display:flex;align-items:baseline;justify-content:space-between;gap:16px;margin-bottom:24px}
h1{font-size:1.4rem;margin:0}h1 span{color:var(--muted);font-weight:400}
.badge{font-size:.75rem;padding:2px 8px;border-radius:999px;background:var(--panel);border:1px solid var(--border);color:var(--muted)}
.panel{background:var(--panel);border:1px solid var(--border);border-radius:8px;padding:16px;margin-bottom:16px}
.panel h2{font-size:.95rem;margin:0 0 12px;color:var(--muted);font-weight:600;text-transform:uppercase;letter-spacing:.04em}
code,pre{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:.85rem}
pre{background:#010409;border:1px solid var(--border);border-radius:6px;padding:12px;overflow:auto;margin:0}
table{width:100%;border-collapse:collapse;font-size:.9rem}
th,td{text-align:left;padding:8px 10px;border-bottom:1px solid var(--border)}
th{color:var(--muted);font-weight:600;font-size:.75rem;text-transform:uppercase}
.status{font-weight:600}.status.passed{color:var(--green)}.status.failed,.status.error{color:var(--red)}.status.running{color:var(--blue)}.status.queued{color:var(--yellow)}
a{color:var(--blue);text-decoration:none}a:hover{text-decoration:underline}
#logbox{white-space:pre-wrap;max-height:360px}
.muted{color:var(--muted)}
#login{margin-top:8px}
input[type=password]{width:100%;max-width:420px;background:#010409;border:1px solid var(--border);color:var(--text);padding:8px 10px;border-radius:6px}
button{background:var(--blue);color:#fff;border:0;border-radius:6px;padding:8px 14px;cursor:pointer;font:inherit;margin-top:8px}
button:hover{filter:brightness(1.1)}
</style>
</head>
<body>
<div class="wrap">
<header>
  <h1>open-crafters <span>runner</span></h1>
  <span class="badge" id="health">checking…</span>
</header>

<div class="panel">
  <h2>Submit from your machine</h2>
  <pre>export CRAFTERS_RUNNER_URL=https://runner.gilla.fun
export CRAFTERS_RUNNER_TOKEN=&lt;token&gt;
cd my-wal && crafters submit
crafters submit --watch   # re-submit on save</pre>
</div>

{{if .Authed}}
<div class="panel">
  <h2>Recent jobs</h2>
  <table>
    <thead><tr><th>When</th><th>Challenge</th><th>Source</th><th>Status</th><th></th></tr></thead>
    <tbody id="jobs"><tr><td colspan="5" class="muted">Loading…</td></tr></tbody>
  </table>
</div>
<div class="panel" id="logpanel" style="display:none">
  <h2>Log <span id="logtitle" class="muted"></span></h2>
  <pre id="logbox"></pre>
</div>
{{else}}
<div class="panel">
  <h2>Job history</h2>
  <p class="muted">Enter your runner token to view recent grading jobs.</p>
  <div id="login">
    <input type="password" id="token" placeholder="RUNNER_TOKEN" autocomplete="off">
    <br><button type="button" onclick="login()">View jobs</button>
  </div>
</div>
{{end}}
</div>
<script>
const token = {{printf "%q" .Token}};
const authed = {{if .Authed}}true{{else}}false{{end}};

async function health(){
  try{
    const r=await fetch('/health');
    const j=await r.json();
    document.getElementById('health').textContent=j.status==='ok'?'healthy':'degraded';
    document.getElementById('health').style.color='var(--green)';
  }catch(e){
    document.getElementById('health').textContent='unreachable';
    document.getElementById('health').style.color='var(--red)';
  }
}

function login(){
  const t=document.getElementById('token').value.trim();
  if(!t)return;
  location.search='?token='+encodeURIComponent(t);
}

function authHeaders(){
  const h={'Accept':'application/json'};
  if(token) h['Authorization']='Bearer '+token;
  return h;
}

async function loadJobs(){
  if(!authed&&!token)return;
  const r=await fetch('/v1/jobs?limit=50',{headers:authHeaders()});
  if(!r.ok){document.getElementById('jobs').innerHTML='<tr><td colspan="5" class="muted">Unauthorized</td></tr>';return;}
  const data=await r.json();
  const rows=(data.jobs||[]).map(j=>{
    const src=j.github_repo||(j.source||'upload');
    const when=(j.finished_at||j.created_at||'').replace('T',' ').replace('Z',' UTC');
    return '<tr><td>'+when+'</td><td>'+j.challenge+'</td><td>'+src+'</td>'+
      '<td class="status '+j.status+'">'+j.status+'</td>'+
      '<td><a href="#" data-id="'+j.id+'">log</a></td></tr>';
  }).join('');
  document.getElementById('jobs').innerHTML=rows||'<tr><td colspan="5" class="muted">No jobs yet</td></tr>';
  document.querySelectorAll('[data-id]').forEach(a=>a.onclick=async e=>{
    e.preventDefault();
    const id=e.target.dataset.id;
    const r=await fetch('/v1/jobs/'+id,{headers:authHeaders()});
    const j=await r.json();
    document.getElementById('logpanel').style.display='block';
    document.getElementById('logtitle').textContent='#'+id.slice(0,8);
    document.getElementById('logbox').textContent=j.log||j.error||'(empty)';
  });
}

health();
if(authed) loadJobs();
setInterval(()=>{if(authed)loadJobs();},5000);
</script>
</body>
</html>`
