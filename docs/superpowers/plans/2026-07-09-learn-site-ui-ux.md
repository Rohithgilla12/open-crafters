# Learn Site UI/UX — Implementation Plan

> **For agentic workers:** Implement task-by-task. Spec: `docs/superpowers/specs/2026-07-09-learn-site-ui-ux-design.md`. Design context: `.impeccable.md`.

**Goal:** Ship catalog-cockpit IA + terminal-craft visual system across the learn site.

**Architecture:** Curate homepage data in `server.go`; restructure Go HTML templates; refresh Tailwind tokens in `styles/input.css` and rebuild `dist/style.css`; light `client.js` for install CTA scroll.

**Tech Stack:** Go `html/template`, Tailwind v4 (`npm run build:learn-css`), vanilla JS.

## Global Constraints

- Dual CTAs on home; no four-card hero promo stack; no full catalog dump on `/`
- No thick `border-left`/`border-right` >1px accents; no gradient text; no glow theater
- Fonts: not Inter/DM Sans/Sora/Space Grotesk/IBM Plex/Syne — use Hanken Grotesk + Martian Mono
- Dark theme only; progress sync must keep working
- Do not ask the user for further decisions; resolve open items with defaults from the spec

---

### Task 1: Visual tokens + CSS rebuild

**Files:** `internal/learn/styles/input.css`, `internal/learn/dist/style.css`

- Replace theme tokens (OKLCH-tinted neutrals, scarce accent, Hanken Grotesk + Martian Mono)
- Flatten components: inset panels, hairlines, quieter pills; remove lift-shadow hover patterns
- Replace path accent thick left borders with 1px rules or text color only
- Run `npm run build:learn-css`

### Task 2: Homepage IA + server data

**Files:** `internal/learn/server.go`, `internal/learn/templates.go` (`indexTmpl`)

- Curate: StartHere roadmap (durability), FeaturedDesigns (scheduler, workflow, url-shortener, distributed-cache), path summaries (count + first 2 challenges)
- Hero: dual CTAs; install strip below; journeys compact; compose banner; featured design; path samples
- Move progress sync to footer

### Task 3: Shared chrome + remaining templates

**Files:** `internal/learn/templates.go` (nav, roadmaps, design, challenge, stage, stacks)

- Shared nav active states; denser list/row patterns; compose stamps; long-read design pages; challenge stage lists as rows

### Task 4: Client JS + tests

**Files:** `internal/learn/client.js`, `internal/learn/design_routes_test.go`

- Scroll/focus `#install` for Build locally CTA
- Update route assertions for new home copy; keep progress sync
- `go test ./internal/learn/ -count=1`
