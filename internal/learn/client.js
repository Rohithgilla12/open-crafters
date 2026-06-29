// Browser-side progress (localStorage) and hosted-runner submit UI.
(function () {
  const STORAGE_KEY = "open-crafters-progress";
  const TOKEN_KEY = "crafters-runner-token";

  function loadProgress() {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (!raw) return { challenges: {} };
      const p = JSON.parse(raw);
      if (!p.challenges) p.challenges = {};
      return p;
    } catch {
      return { challenges: {} };
    }
  }

  function saveProgress(p) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(p));
  }

  function ensureChallenge(p, slug) {
    if (!p.challenges[slug]) p.challenges[slug] = { read: {}, passed: {} };
    const c = p.challenges[slug];
    if (!c.read) c.read = {};
    if (!c.passed) c.passed = {};
    return c;
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
        if (statusEl) statusEl.textContent = "Passed";
        return;
      }
      if (st === "failed" || st === "error") {
        if (statusEl) statusEl.textContent = st === "failed" ? "Stage failed — see log" : "Error";
        const passed = passedStagesFromLog(job.log);
        if (passed.length) {
          markPassed(slug, passed);
          applyProgressUI();
        }
        return;
      }
    }
    if (statusEl) statusEl.textContent = "Timed out waiting for job";
  }

  function sleep(ms) {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  document.addEventListener("DOMContentLoaded", () => {
    initStagePage();
    applyProgressUI();
    initSubmitForm();
  });
})();
