# Learn SEO + Blog — Design Spec

**Date:** 2026-07-09  
**Site:** https://learn.gilla.fun/  
**Approach:** A — Technical SEO + markdown blog in learn (approved)

## Goals

1. Make existing catalog pages crawlable and shareable (sitemap, robots, canonical, OG/Twitter, JSON-LD).
2. Add `/blog` content that ranks for infra topics and funnels into challenges/design.

## Technical SEO

| Asset | Behavior |
|---|---|
| Canonical | `https://learn.gilla.fun{path}` on every page |
| Meta description | Unique per page from tagline/description |
| OG + Twitter | title, description, url, type; image → existing `og.png` on GitHub Pages or learn asset if present |
| JSON-LD | `WebSite` (home), `LearningResource` (challenges), `Article` (blog) |
| `/sitemap.xml` | home, roadmaps, design, challenges, blog index + posts |
| `/robots.txt` | `Allow: /` + `Sitemap: https://learn.gilla.fun/sitemap.xml` |

## Blog

- **Routes:** `GET /blog`, `GET /blog/{slug}`
- **Content:** `content/blog/*.md` with YAML frontmatter (`title`, `description`, `date`, `tags`, `related` challenge/design slugs)
- **Embed:** via `go:embed` (extend `content.go` or learn-local embed)
- **UI:** terminal-craft list + long-read article; CTA to related challenge
- **Seed posts (4):** WAL durability, Temporal workflows, Raft consensus, workflow SDK replay

## Non-goals

- CMS / admin UI
- Comments, newsletter, analytics product
- Translating all challenge docs into blog posts

## Acceptance

1. `/robots.txt` and `/sitemap.xml` return 200 with expected URLs
2. Challenge/home pages include canonical + og:title
3. `/blog` lists seeded posts; `/blog/{slug}` renders markdown
4. Nav includes Blog; learn tests cover new routes
5. Deployable via existing VPS learn image
