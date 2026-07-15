// Browser-side progress (localStorage) and hosted-runner submit UI.
(function () {
  const STORAGE_KEY = "open-crafters-progress";
  const TOKEN_KEY = "crafters-runner-token";
  const FORMAT_VERSION = 1;

  function loadProgress() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return { version: FORMAT_VERSION, challenges: {}, design: {} };
      const p = JSON.parse(raw);
      if (!p.challenges) p.challenges = {};
      if (!p.design) p.design = {};
      if (!p.version) p.version = FORMAT_VERSION;
      return p;
    } catch {
      return { version: FORMAT_VERSION, challenges: {}, design: {} };
    }
  }

  function saveProgress(p) {
    p.version = FORMAT_VERSION;
    localStorage.setItem(STORAGE_KEY, JSON.stringify(p));
  }

  function mergeTimestamps(dst, src) {
    if (!dst || !src) return;
    for (const [k, v] of Object.entries(src)) {
      if (!v) continue;
      if (!dst[k] || v < dst[k]) dst[k] = v;
    }
  }

  function mergeProgress(incoming) {
    if (!incoming) return loadProgress();
    const p = loadProgress();
    if (incoming.challenges) {
      for (const [slug, sc] of Object.entries(incoming.challenges)) {
        const c = ensureChallenge(p, slug);
        mergeTimestamps(c.passed, sc.passed || {});
        mergeTimestamps(c.read, sc.read || {});
      }
    }
    if (incoming.design) {
      for (const [slug, sd] of Object.entries(incoming.design)) {
        const d = ensureDesign(p, slug);
        if (sd.completed_at && (!d.completed_at || sd.completed_at < d.completed_at)) {
          d.completed_at = sd.completed_at;
        }
        mergeTimestamps(d.prompts, sd.prompts || {});
      }
    }
    saveProgress(p);
    return p;
  }

  function exportProgress() {
    const p = loadProgress();
    const blob = new Blob([JSON.stringify(p, null, 2) + "\n"], {
      type: "application/json",
    });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "progress.json";
    a.click();
    URL.revokeObjectURL(a.href);
  }

  function ensureDesign(p, slug) {
    if (!p.design[slug]) p.design[slug] = { prompts: {} };
    const d = p.design[slug];
    if (!d.prompts) d.prompts = {};
    return d;
  }

  function ensureChallenge(p, slug) {
    if (!p.challenges[slug]) p.challenges[slug] = { read: {}, passed: {} };
    const c = p.challenges[slug];
    if (!c.read) c.read = {};
    if (!c.passed) c.passed = {};
    return c;
  }

  function markDesignPrompt(slug, idx) {
    const p = loadProgress();
    const d = ensureDesign(p, slug);
    const key = String(idx);
    if (!d.prompts[key]) {
      d.prompts[key] = new Date().toISOString();
      saveProgress(p);
    }
  }

  function unmarkDesignPrompt(slug, idx) {
    const p = loadProgress();
    const d = p.design[slug];
    if (!d || !d.prompts) return;
    delete d.prompts[String(idx)];
    saveProgress(p);
  }

  function markDesignComplete(slug) {
    const p = loadProgress();
    const d = ensureDesign(p, slug);
    d.completed_at = new Date().toISOString();
    saveProgress(p);
  }

  function applyDesignProgressUI() {
    const p = loadProgress();

    document.querySelectorAll("[data-design]").forEach((root) => {
      const slug = root.dataset.design;
      const d = p.design[slug] || { prompts: {} };
      root.querySelectorAll("[data-design-progress-label]").forEach((el) => {
        if (d.completed_at) el.textContent = "complete · ";
        else if (d.prompts && Object.keys(d.prompts).length > 0) {
          el.textContent = Object.keys(d.prompts).length + " prompts done · ";
        } else el.textContent = "";
      });
    });

    const slug = document.body.dataset.design;
    if (!slug) return;
    const d = p.design[slug] || { prompts: {} };
    document.querySelectorAll(".design-prompt-check").forEach((cb) => {
      const idx = cb.dataset.promptIdx;
      cb.checked = !!(d.prompts && d.prompts[idx]);
    });
    const btn = document.getElementById("design-complete-btn");
    if (btn && d.completed_at) {
      btn.textContent = "Completed ✓";
      btn.disabled = true;
    }
    document.querySelectorAll("[data-design-progress-label]").forEach((el) => {
      if (d.completed_at) el.textContent = "complete · ";
    });
    applyDesignRoadmapProgress();
  }

  function designRoadmapCompleted(p, designsCSV) {
    let total = 0;
    let done = 0;
    for (const slug of designsCSV.split(",").filter(Boolean)) {
      total++;
      const d = p.design[slug];
      if (d && d.completed_at) done++;
    }
    return { total, done };
  }

  function applyDesignRoadmapProgress() {
    const p = loadProgress();
    document.querySelectorAll("[data-design-roadmap-card]").forEach((card) => {
      const csv = card.dataset.designs || "";
      const total = parseInt(card.dataset.totalProblems || "0", 10);
      const { done } = designRoadmapCompleted(p, csv);
      const pct = total > 0 ? Math.round((done / total) * 100) : 0;
      const fill = card.querySelector(".roadmap-bar-fill");
      if (fill) fill.style.width = pct + "%";
      card.querySelectorAll("[data-design-roadmap-progress-label]").forEach((el) => {
        if (done === total && total > 0) el.textContent = "complete · ";
        else if (done > 0) el.textContent = done + "/" + total + " complete · ";
        else el.textContent = "";
      });
    });
    const bar = document.querySelector("[data-design-roadmap-bar]");
    if (bar) {
      const csv = document.body.dataset.designs || "";
      const total = parseInt(bar.dataset.totalProblems || "0", 10);
      const { done } = designRoadmapCompleted(p, csv);
      const pct = total > 0 ? Math.round((done / total) * 100) : 0;
      const fill = bar.querySelector(".roadmap-bar-fill");
      if (fill) fill.style.width = pct + "%";
      document.querySelectorAll("[data-design-roadmap-progress-label]").forEach((el) => {
        if (done === total && total > 0) el.textContent = "complete — " + total + " problems";
        else el.textContent = done + "/" + total + " complete";
      });
    }
  }

  function initDesignPage() {
    const slug = document.body.dataset.design;
    if (!slug) return;

    document.querySelectorAll(".design-prompt-check").forEach((cb) => {
      cb.addEventListener("change", () => {
        const idx = cb.dataset.promptIdx;
        if (cb.checked) markDesignPrompt(slug, idx);
        else unmarkDesignPrompt(slug, idx);
        applyDesignProgressUI();
      });
    });

    const btn = document.getElementById("design-complete-btn");
    if (btn) {
      btn.addEventListener("click", () => {
        markDesignComplete(slug);
        applyDesignProgressUI();
        applyDesignRoadmapProgress();
      });
    }
    applyDesignProgressUI();
  }


  function markRead(slug, stageSlug) {
    if (!slug || !stageSlug) return;
    const p = loadProgress();
    const c = ensureChallenge(p, slug);
    if (!c.read[stageSlug]) {
      c.read[stageSlug] = new Date().toISOString();
      saveProgress(p);
    }
  }

  function markPassed(slug, stageSlugs) {
    if (!slug || !stageSlugs.length) return;
    const p = loadProgress();
    const c = ensureChallenge(p, slug);
    const now = new Date().toISOString();
    for (const s of stageSlugs) {
      if (!c.passed[s]) c.passed[s] = now;
    }
    saveProgress(p);
  }

  function stageList() {
    const body = document.body;
    const raw = body.dataset.stages;
    if (!raw) return [];
    return raw.split(",").filter(Boolean);
  }

  function roadmapPassedStages(p, challengesCSV) {
    let total = 0;
    let passed = 0;
    for (const slug of challengesCSV.split(",").filter(Boolean)) {
      const el = document.querySelector('[data-challenge="' + slug + '"]');
      const stages = el
        ? (el.dataset.stages || "").split(",").filter(Boolean)
        : [];
      const c = p.challenges[slug] || { read: {}, passed: {} };
      total += stages.length;
      passed += stages.filter((s) => c.passed && c.passed[s]).length;
    }
    return { total, passed };
  }

  function applyRoadmapProgress() {
    const p = loadProgress();

    document.querySelectorAll("[data-roadmap-card]").forEach((card) => {
      const csv = card.dataset.challenges || "";
      const total = parseInt(card.dataset.totalStages || "0", 10);
      const { passed } = roadmapPassedStages(p, csv);
      const pct = total > 0 ? Math.round((passed / total) * 100) : 0;
      const fill = card.querySelector(".roadmap-bar-fill");
      if (fill) fill.style.width = pct + "%";
      card.querySelectorAll("[data-roadmap-progress-label]").forEach((el) => {
        if (passed === total && total > 0) el.textContent = "complete · ";
        else if (passed > 0) el.textContent = passed + "/" + total + " stages · ";
        else el.textContent = "";
      });
    });

    const bar = document.querySelector("[data-roadmap-bar]");
    if (bar) {
      const csv = document.body.dataset.challenges || "";
      const total = parseInt(bar.dataset.totalStages || "0", 10);
      const { passed } = roadmapPassedStages(p, csv);
      const pct = total > 0 ? Math.round((passed / total) * 100) : 0;
      const fill = bar.querySelector(".roadmap-bar-fill");
      if (fill) fill.style.width = pct + "%";
      document.querySelectorAll("[data-roadmap-progress-label]").forEach((el) => {
        if (passed === total && total > 0) el.textContent = "complete — " + total + " stages";
        else el.textContent = passed + "/" + total + " stages passed";
      });
    }
  }

  function applyProgressUI() {
    const p = loadProgress();

    document.querySelectorAll("[data-stage-slug]").forEach((el) => {
      const root = el.closest("[data-challenge]") || document.body;
      const slug = root.dataset.challenge || document.body.dataset.challenge;
      if (!slug) return;
      const c = p.challenges[slug] || { read: {}, passed: {} };
      const st = el.dataset.stageSlug;
      el.classList.remove("progress-read", "progress-passed");
      if (c.passed && c.passed[st]) el.classList.add("progress-passed");
      else if (c.read && c.read[st]) el.classList.add("progress-read");
    });

    document.querySelectorAll("[data-challenge]").forEach((root) => {
      const slug = root.dataset.challenge;
      const stages = (root.dataset.stages || "").split(",").filter(Boolean);
      const c = p.challenges[slug] || { read: {}, passed: {} };
      const readN = stages.filter((s) => c.read && c.read[s]).length;
      const passN = stages.filter((s) => c.passed && c.passed[s]).length;
      const total = stages.length;
      root.querySelectorAll("[data-progress-label]").forEach((el) => {
        if (passN === total && total > 0) el.textContent = "complete";
        else if (passN > 0) el.textContent = passN + "/" + total + " passed";
        else if (readN > 0) el.textContent = readN + "/" + total + " read";
        else el.textContent = "";
      });
    });

    if (document.body.dataset.challenge && !document.querySelector("[data-challenge]")) {
      const slug = document.body.dataset.challenge;
      const stages = stageList();
      const c = p.challenges[slug] || { read: {}, passed: {} };
      const readN = stages.filter((s) => c.read && c.read[s]).length;
      const passN = stages.filter((s) => c.passed && c.passed[s]).length;
      const total = stages.length;
      document.querySelectorAll("[data-progress-label]").forEach((el) => {
        if (passN === total && total > 0) el.textContent = "complete";
        else if (passN > 0) el.textContent = passN + "/" + total + " passed";
        else if (readN > 0) el.textContent = readN + "/" + total + " read";
        else el.textContent = "";
      });
    }
  }

  function passedStagesFromLog(log) {
    const out = [];
    if (!log) return out;
    const re = /✓\s+(\S+)\s+passed/g;
    let m;
    while ((m = re.exec(log)) !== null) out.push(m[1]);
    return out;
  }

  function saveToken(token) {
    if (token) localStorage.setItem(TOKEN_KEY, token);
    else localStorage.removeItem(TOKEN_KEY);
  }

  function loadToken() {
    return localStorage.getItem(TOKEN_KEY) || "";
  }

  function initStagePage() {
    const slug = document.body.dataset.challenge;
    const stage = document.body.dataset.stage;
    if (slug && stage) markRead(slug, stage);
    applyProgressUI();
  }

  function initSubmitForm() {
    const form = document.getElementById("submit-form");
    if (!form) return;

    const tokenInput = form.querySelector('[name="token"]');
    const fileInput = form.querySelector('[name="file"]');
    const allInput = form.querySelector('[name="all"]');
    const statusEl = document.getElementById("submit-status");
    const logEl = document.getElementById("submit-log");
    const slug = form.dataset.challenge;

    if (tokenInput) tokenInput.value = loadToken();

    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      const token = tokenInput ? tokenInput.value.trim() : "";
      if (!token) {
        if (statusEl) statusEl.textContent = "Enter your runner token.";
        return;
      }
      saveToken(token);
      const file = fileInput && fileInput.files && fileInput.files[0];
      if (!file) {
        if (statusEl) statusEl.textContent = "Choose a zip of your solution directory.";
        return;
      }
      if (statusEl) statusEl.textContent = "Uploading…";
      if (logEl) logEl.textContent = "";

      const fd = new FormData();
      fd.append("challenge", slug);
      fd.append("file", file);
      if (allInput && allInput.checked) fd.append("all", "true");

      try {
        const r = await fetch("/api/submit", {
          method: "POST",
          headers: { Authorization: "Bearer " + token },
          body: fd,
        });
        const text = await r.text();
        if (!r.ok) {
          if (statusEl) statusEl.textContent = "Submit failed: " + text;
          return;
        }
        const job = JSON.parse(text);
        if (statusEl) statusEl.textContent = "Queued job " + job.id + "…";
        await pollJob(token, job.id, statusEl, logEl, slug);
      } catch (err) {
        if (statusEl) statusEl.textContent = "Error: " + err.message;
      }
    });
  }

  async function pollJob(token, id, statusEl, logEl, slug) {
    const deadline = Date.now() + 15 * 60 * 1000;
    while (Date.now() < deadline) {
      await sleep(1500);
      const r = await fetch("/api/jobs/" + id, {
        headers: { Authorization: "Bearer " + token },
      });
      if (!r.ok) {
        if (statusEl) statusEl.textContent = "Poll failed (" + r.status + ")";
        return;
      }
      const job = await r.json();
      const st = job.status || "unknown";
      if (statusEl) statusEl.textContent = "Status: " + st;
      if (logEl && job.log) logEl.textContent = job.log;

      if (st === "passed") {
        const passed = passedStagesFromLog(job.log);
        if (passed.length) markPassed(slug, passed);
        else if (job.all) markPassed(slug, stageList());
        applyProgressUI();
        applyRoadmapProgress();
        if (statusEl) statusEl.textContent = "Passed";
        return;
      }
      if (st === "failed" || st === "error") {
        if (statusEl) statusEl.textContent = st === "failed" ? "Stage failed — see log" : "Error";
        const passed = passedStagesFromLog(job.log);
        if (passed.length) {
          markPassed(slug, passed);
          applyProgressUI();
          applyRoadmapProgress();
        }
        return;
      }
    }
    if (statusEl) statusEl.textContent = "Timed out waiting for job";
  }

  function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  function initInstallCTA() {
    document.querySelectorAll('[data-scroll-to-install]').forEach((btn) => {
      btn.addEventListener('click', (e) => {
        const el = document.getElementById('install');
        if (!el) return;
        e.preventDefault();
        el.scrollIntoView({ behavior: 'smooth', block: 'start' });
        const code = el.querySelector('code');
        if (code) code.focus?.();
      });
    });
  }

  // --- Cmd+K jump palette -------------------------------------------------

  const PAGES = [
    { group: "Pages", label: "Home", href: "/", keywords: "index challenges" },
    { group: "Pages", label: "Roadmaps", href: "/roadmaps", keywords: "paths learning" },
    { group: "Pages", label: "System design", href: "/design", keywords: "whiteboard interview" },
    { group: "Pages", label: "Design roadmaps", href: "/design/roadmaps", keywords: "" },
    { group: "Pages", label: "Design stacks", href: "/design/stacks", keywords: "compose" },
    { group: "Pages", label: "Blog", href: "/blog", keywords: "posts articles" },
  ];

  let cmdkItems = null;
  let cmdkLoaded = false;
  let cmdkIndexPromise = null;
  let cmdkOpenGen = 0;
  let cmdkState = { open: false, query: "", active: 0, filtered: [] };
  let cmdkEls = null;

  function isEditableTarget(el) {
    if (!el || el === document.body) return false;
    const tag = (el.tagName || "").toLowerCase();
    if (tag === "input" || tag === "textarea" || tag === "select") return true;
    if (el.isContentEditable) return true;
    return false;
  }

  function cmdkShortcutLabel() {
    const mac = /Mac|iPhone|iPad|iPod/i.test(navigator.platform || navigator.userAgent || "");
    return mac ? "⌘K" : "Ctrl+K";
  }

  function scoreItem(item, q) {
    if (!q) return 1;
    const hay = (item.label + " " + item.slug + " " + (item.keywords || "")).toLowerCase();
    if (hay.includes(q)) return q.length / hay.length + (item.label.toLowerCase().startsWith(q) ? 1 : 0);
    const parts = q.split(/\s+/).filter(Boolean);
    if (parts.every((p) => hay.includes(p))) return 0.5;
    return 0;
  }

  function filterCmdk(query) {
    const q = query.trim().toLowerCase();
    const scored = [];
    for (const item of cmdkItems || []) {
      const s = scoreItem(item, q);
      if (s > 0) scored.push({ item, s });
    }
    scored.sort((a, b) => b.s - a.s || a.item.group.localeCompare(b.item.group) || a.item.label.localeCompare(b.item.label));
    return scored.map((x) => x.item);
  }

  async function loadCmdkIndex() {
    if (cmdkLoaded && cmdkItems) return cmdkItems;
    if (cmdkIndexPromise) return cmdkIndexPromise;
    cmdkIndexPromise = (async () => {
      const items = PAGES.map((p) => ({ ...p, slug: "" }));
      try {
        const [chRes, rmRes, desRes, blogRes] = await Promise.all([
          fetch("/api/challenges"),
          fetch("/api/roadmaps"),
          fetch("/api/design"),
          fetch("/api/blog"),
        ]);
        if (chRes.ok) {
          const data = await chRes.json();
          for (const c of data.challenges || []) {
            items.push({
              group: "Challenges",
              label: c.name || c.slug,
              slug: c.slug || "",
              href: "/challenges/" + c.slug,
              keywords: (c.tagline || "") + " " + (c.difficulty || ""),
            });
          }
        }
        if (rmRes.ok) {
          const data = await rmRes.json();
          for (const r of data.roadmaps || []) {
            items.push({
              group: "Roadmaps",
              label: r.name || r.slug,
              slug: r.slug || "",
              href: "/roadmaps/" + r.slug,
              keywords: r.tagline || "",
            });
          }
        }
        if (desRes.ok) {
          const data = await desRes.json();
          for (const d of data.design || []) {
            items.push({
              group: "Design",
              label: d.name || d.slug,
              slug: d.slug || "",
              href: "/design/" + d.slug,
              keywords: (d.tagline || "") + " " + (d.category || ""),
            });
          }
        }
        if (blogRes.ok) {
          const data = await blogRes.json();
          for (const p of data.posts || []) {
            items.push({
              group: "Blog",
              label: p.title || p.slug,
              slug: p.slug || "",
              href: "/blog/" + p.slug,
              keywords: p.description || "",
            });
          }
        }
      } catch {
        /* pages-only fallback */
      }
      cmdkItems = items;
      cmdkLoaded = true;
      return items;
    })();
    return cmdkIndexPromise;
  }

  function ensureCmdkDOM() {
    if (cmdkEls) return cmdkEls;
    const root = document.createElement("div");
    root.id = "cmdk-root";
    root.className = "fixed inset-0 z-76 hidden";
    root.setAttribute("aria-hidden", "true");
    root.innerHTML =
      '<div data-cmdk-backdrop class="absolute inset-0 bg-canvas/90 backdrop-blur-sm"></div>' +
      '<div class="relative mx-auto mt-[12vh] flex max-h-[70vh] w-full max-w-[580px] flex-col overflow-hidden rounded-[14px] border border-border bg-surface shadow-[0_4px_20px_rgba(110,231,183,0.08)]" role="dialog" aria-modal="true" aria-label="Jump anywhere">' +
      '<div class="flex shrink-0 items-center gap-2 border-b border-border-soft px-4 py-3">' +
      '<span class="font-mono text-accent">$</span>' +
      '<input data-cmdk-input type="text" autocomplete="off" spellcheck="false" placeholder="Jump to challenge, roadmap, design, blog…" class="min-w-0 flex-1 bg-transparent font-mono text-sm text-ink outline-none placeholder:text-muted" />' +
      '<kbd class="hidden rounded border border-border-soft bg-canvas-elevated px-1.5 py-0.5 font-mono text-[0.58rem] text-muted sm:inline">esc</kbd>' +
      "</div>" +
      '<div data-cmdk-list class="min-h-0 flex-1 overflow-auto py-2" role="listbox"></div>' +
      '<div data-cmdk-empty class="hidden shrink-0 px-4 py-6 text-center font-mono text-sm text-muted">No matches</div>' +
      "</div>";
    document.body.appendChild(root);
    cmdkEls = {
      root,
      backdrop: root.querySelector("[data-cmdk-backdrop]"),
      input: root.querySelector("[data-cmdk-input]"),
      list: root.querySelector("[data-cmdk-list]"),
      empty: root.querySelector("[data-cmdk-empty]"),
    };
    cmdkEls.backdrop.addEventListener("click", closeCmdk);
    cmdkEls.input.addEventListener("input", () => {
      cmdkState.query = cmdkEls.input.value;
      cmdkState.active = 0;
      renderCmdkList();
    });
    cmdkEls.input.addEventListener("keydown", (e) => {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        moveCmdk(1);
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        moveCmdk(-1);
      } else if (e.key === "Enter") {
        e.preventDefault();
        activateCmdk();
      } else if (e.key === "Escape") {
        e.preventDefault();
        closeCmdk();
      }
    });
    return cmdkEls;
  }

  function renderCmdkList() {
    const els = ensureCmdkDOM();
    const filtered = filterCmdk(cmdkState.query);
    cmdkState.filtered = filtered;
    if (cmdkState.active >= filtered.length) cmdkState.active = Math.max(0, filtered.length - 1);
    els.list.innerHTML = "";
    if (!filtered.length) {
      els.empty.classList.remove("hidden");
      els.list.classList.add("hidden");
      return;
    }
    els.empty.classList.add("hidden");
    els.list.classList.remove("hidden");
    let lastGroup = "";
    filtered.forEach((item, i) => {
      if (item.group !== lastGroup) {
        lastGroup = item.group;
        const h = document.createElement("div");
        h.className =
          "px-4 pb-1 pt-2 font-mono text-[0.62rem] font-semibold uppercase tracking-widest text-muted";
        h.textContent = item.group;
        els.list.appendChild(h);
      }
      const btn = document.createElement("button");
      btn.type = "button";
      btn.role = "option";
      btn.setAttribute("aria-selected", i === cmdkState.active ? "true" : "false");
      btn.dataset.index = String(i);
      btn.className =
        "flex w-full items-baseline gap-2 px-4 py-2 text-left text-sm transition-[background-color] duration-150 " +
        (i === cmdkState.active ? "bg-accent/10 text-ink" : "text-ink hover:bg-surface-hover");
      const label = document.createElement("span");
      label.className = "min-w-0 flex-1 truncate font-display font-medium";
      label.textContent = item.label;
      btn.appendChild(label);
      if (item.slug) {
        const slug = document.createElement("span");
        slug.className = "shrink-0 font-mono text-[0.72rem] text-muted";
        slug.textContent = item.slug;
        btn.appendChild(slug);
      }
      btn.addEventListener("mousemove", () => {
        if (cmdkState.active !== i) {
          cmdkState.active = i;
          renderCmdkList();
        }
      });
      btn.addEventListener("click", () => {
        cmdkState.active = i;
        activateCmdk();
      });
      els.list.appendChild(btn);
    });
    const activeBtn = els.list.querySelector('[aria-selected="true"]');
    if (activeBtn) activeBtn.scrollIntoView({ block: "nearest" });
  }

  function moveCmdk(delta) {
    const n = cmdkState.filtered.length;
    if (!n) return;
    cmdkState.active = (cmdkState.active + delta + n) % n;
    renderCmdkList();
  }

  function activateCmdk() {
    const item = cmdkState.filtered[cmdkState.active];
    if (!item || !item.href) return;
    closeCmdk();
    window.location.href = item.href;
  }

  async function openCmdk() {
    const els = ensureCmdkDOM();
    const openId = ++cmdkOpenGen;
    cmdkState.open = true;
    cmdkState.query = "";
    cmdkState.active = 0;
    els.input.value = "";
    els.root.classList.remove("hidden");
    els.root.setAttribute("aria-hidden", "false");
    document.documentElement.classList.add("overflow-hidden");
    if (!cmdkItems) {
      cmdkItems = PAGES.map((p) => ({ ...p, slug: "" }));
    }
    renderCmdkList();
    requestAnimationFrame(() => els.input.focus());
    await loadCmdkIndex();
    if (!cmdkState.open || openId !== cmdkOpenGen) return;
    renderCmdkList();
  }

  function closeCmdk() {
    if (!cmdkEls || !cmdkState.open) return;
    cmdkState.open = false;
    cmdkOpenGen++;
    cmdkEls.root.classList.add("hidden");
    cmdkEls.root.setAttribute("aria-hidden", "true");
    document.documentElement.classList.remove("overflow-hidden");
  }

  function toggleCmdk() {
    if (cmdkState.open) closeCmdk();
    else openCmdk();
  }

  function initCmdk() {
    document.querySelectorAll("[data-cmdk-hint]").forEach((el) => {
      el.textContent = cmdkShortcutLabel();
    });
    document.querySelectorAll("[data-cmdk-open]").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.preventDefault();
        openCmdk();
      });
    });
    document.addEventListener("keydown", (e) => {
      const key = e.key && e.key.toLowerCase();
      if ((e.metaKey || e.ctrlKey) && key === "k") {
        if (isEditableTarget(e.target) && !cmdkState.open) return;
        e.preventDefault();
        toggleCmdk();
        return;
      }
      if (key === "escape" && cmdkState.open) {
        e.preventDefault();
        closeCmdk();
      }
    });
  }

  function initProgressSync() {
    const exportBtn = document.getElementById("progress-export");
    const importInput = document.getElementById("progress-import");
    const statusEl = document.getElementById("progress-sync-status");

    if (exportBtn) {
      exportBtn.addEventListener("click", () => {
        exportProgress();
        if (statusEl) statusEl.textContent = "Downloaded progress.json";
      });
    }
    if (importInput) {
      importInput.addEventListener("change", async () => {
        const file = importInput.files && importInput.files[0];
        importInput.value = "";
        if (!file) return;
        try {
          const text = await file.text();
          const incoming = JSON.parse(text);
          mergeProgress(incoming);
          applyProgressUI();
          applyDesignProgressUI();
          applyDesignRoadmapProgress();
          applyRoadmapProgress();
          if (statusEl) statusEl.textContent = "Imported " + file.name;
        } catch (err) {
          if (statusEl) statusEl.textContent = "Import failed: " + err.message;
        }
      });
    }
  }

  document.addEventListener("DOMContentLoaded", () => {
    initStagePage();
    initDesignPage();
    applyProgressUI();
    applyDesignProgressUI();
    applyDesignRoadmapProgress();
    applyRoadmapProgress();
    initSubmitForm();
    initProgressSync();
    initInstallCTA();
    initCmdk();
  });
})();
