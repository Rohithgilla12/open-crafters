# Cmd+K Jump Palette — Design Spec

**Date:** 2026-07-15  
**Site:** https://learn.gilla.fun  
**Approach:** Vanilla overlay in `client.js` (approved)

## Goals

1. ⌘K / Ctrl+K opens a jump-anywhere palette on every learn page.
2. Fuzzy-filter Challenges, Roadmaps, Design problems, Blog posts, and top-level Pages.
3. Match existing craft UI (surface panel, mono hints, accent focus).

## Non-goals

- Continue / streak actions
- Progress mutation
- Stage-level entries (challenge overview only)
- New JS frameworks

## Behavior

| Input | Action |
|---|---|
| ⌘K / Ctrl+K | Toggle palette (ignore when focus in input/textarea/select outside palette) |
| Esc | Close |
| Type | Filter by name + slug (case-insensitive substring) |
| ↑↓ | Move selection |
| Enter / click | Navigate to `href` |

## Data

- `/api/challenges` → `/challenges/{slug}`
- `/api/roadmaps` → `/roadmaps/{slug}`
- `/api/design` → `/design/{slug}`
- `/api/blog` (new) → `/blog/{slug}`
- Static pages: Home, Roadmaps, Design, Design roadmaps, Design stacks, Blog

Catalog loads once per page session (lazy on first open).

## UI

- Centered dialog, dimmed backdrop, `role="dialog"` + `aria-modal`
- Search input autofocus; results grouped with mono section labels
- Nav hint button showing platform-appropriate `⌘K` / `Ctrl+K`

## Acceptance

1. Palette opens on ⌘K/Ctrl+K from home and a challenge page
2. Filtering finds a challenge by slug fragment
3. Enter navigates; Esc closes
4. `go test ./internal/learn/` passes; CSS rebuild committed if needed
