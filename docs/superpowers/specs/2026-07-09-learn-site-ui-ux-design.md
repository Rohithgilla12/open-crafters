# Learn site UI/UX redesign — Catalog cockpit

**Date:** 2026-07-09  
**Site:** https://learn.gilla.fun/  
**Code:** `internal/learn/` (Go templates + Tailwind `styles/input.css` + `client.js`)  
**Approach:** A — Catalog cockpit (approved)  
**Scope:** Full site pass (home, roadmaps, design, challenge/design detail)  
**Design context:** `.impeccable.md`

## Problem

The homepage dumps almost the entire catalog: hero, install, four promo cards, all roadmaps, all 16 design problems, then every challenge under every path. First-visit path selection is slow. Visually it reads as generic dark-dev SaaS (card soup, mint glow, overused Sora/DM Sans) rather than a serious infra lab.

## Goals

1. **Clearer first visit** — dual equal CTAs; curated home; full catalogs on dedicated pages.
2. **Terminal craft identity** — dense, precise, industrial; hairline rules; scarce accent; mono for commands only.
3. **Consistent chrome** across roadmaps, design, and challenge detail pages.

## Non-goals

- Light mode / theme toggle (this pass).
- New product features (auth, accounts, search backend).
- Changing challenge content, grading, or protocols.
- Rewriting the hosted runner product UI.

## Users & success

| Persona | Success |
|---|---|
| First-time visitor | Picks “Build locally” or “Browse journeys” in &lt;10s without scrolling the whole catalog |
| Returning learner | Finds roadmap/challenge quickly; progress sync still works |
| Design interviewer prep | Browses compact design index; opens long-read problem pages |

## Information architecture

### Homepage (`/`)

**First viewport (one composition)**

- Brand: `$ open-crafters` + `learn`
- One headline + one short supporting sentence
- **Two equal CTAs:** `Build locally` (focus/scroll to install strip) · `Browse journeys` → `/roadmaps`
- No four stacked promo cards in the hero

**Below the fold (curated)**

1. **Install strip** — curl + `crafters start wal` (supporting, not competing with CTAs)
2. **Start here** — Durability roadmap featured + short “why WAL first”
3. **Journeys** — compact grid/strip of roadmaps + “View all”
4. **Compose callout** — single banner (workflow note + link)
5. **System design** — 3–4 featured problems + “View all 16”
6. **Challenge paths** — path headers with counts / samples; not every challenge card

**Progress sync** — leave hero; place in footer utility or on roadmap/challenge pages.

### Other surfaces

| Route | Pattern |
|---|---|
| `/roadmaps` | Dense journey list/rows (name, tagline, stages, progress); compose stamps |
| `/roadmaps/:slug` | Numbered milestone rows + hairlines; compose stamps; progress near top |
| `/design` | Compact problem index (theme · difficulty · time); optional featured strip |
| `/design/:slug` | Long-read: problem → prompts → build bridge; spoilers gated; less nested cards |
| `/challenges/:slug` | Stages as tight list/rail; start command prominent; compose as inset panel |
| Nav | Existing links + clearer active state; Challenges index only if we add a real `/challenges` list (optional; not required if path pages cover it) |

## Visual system (terminal craft)

### Tone

Dense, precise, industrial lab. Dark theme (late-night coding context). Confidence over cheerfulness.

### Surfaces

- Flatten: fewer floating cards; more hairline dividers and inset panels
- Cards only for clickable destinations
- Soften mint atmospheric glow; accent for status/labels (~10%), not page atmosphere

### Typography

- Brand voice words: **dense · precise · industrial**
- Reflex fonts to reject (do not use): Inter, DM Sans, Sora, Space Grotesk, IBM Plex*, Syne, Instrument*, etc. (see impeccable skill)
- Pair: distinctive utilitarian display/UI sans + mono **only** for commands/labels
- Fluid type for marketing headings on home; fixed rem scale for dense app lists
- Fewer sizes, ≥1.25 step between hierarchy levels; body ~65–75ch

### Color

- OKLCH tokens; neutrals tinted toward brand hue (current mint/teal family, de-glowed)
- Path accents as thin signals (dots / 1px rules / text color) — **not** thick side-stripe borders (`border-left` &gt; 1px banned)
- Difficulty pills quieter; compose badge as small stamp

### Motion

- Hover: border/underline brighten only
- No lift-and-shadow card theater
- Honor `prefers-reduced-motion`

### Absolute bans (impeccable)

- Thick colored side-stripe accents on cards/list items
- Gradient text
- Purple/cyan glow stacks, pill clusters, nested cards

## Technical approach

Primary touchpoints:

- `internal/learn/templates.go` — restructure `indexTmpl` and align other templates to shared chrome/patterns
- `internal/learn/styles/input.css` — token refresh, component classes, rebuild embedded `dist/style.css`
- `internal/learn/client.js` — CTA scroll-to-install; progress sync placement; no behavior regressions
- `internal/learn/server.go` — only if homepage needs curated subsets (featured design slugs, start-here roadmap); prefer template-side filtering when data already in catalog
- Tests: `internal/learn/design_routes_test.go` (+ any new route/content assertions for curated home)

Build CSS via existing learn Tailwind pipeline (document exact command in implementation plan).

## Acceptance criteria

1. Home first viewport has dual CTAs and no four-card promo stack.
2. Home does not render the full challenge catalog or all 16 design cards.
3. `/roadmaps`, `/design`, challenge and design detail pages share the new visual system.
4. Progress sync still works (export/import).
5. Existing learn route tests pass; update assertions for new copy/structure.
6. Mobile: CTAs stack, install readable, lists usable.
7. Passes AI-slop check: no banned patterns; feels like terminal craft, not generic dark SaaS.

## Open decisions (resolve in plan if needed)

- Exact font pair (chosen during implementation via impeccable font procedure)
- Whether to add `/challenges` index or keep challenges reachable only via roadmaps/paths
- Which 3–4 design problems are “featured” on home (default: scheduler, workflow, URL shortener, distributed cache — or first N by catalog order)

## Out of scope follow-ups

- Light mode
- Full-text search
- Personalization beyond local progress.json
