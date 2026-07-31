# Portal visual refresh and multi-agent demo seed

Status: approved design, 2026-07-31

Two changes to `esc serve`: a full visual redesign of the admin portal (refined enterprise light, sidebar shell, restyled charts), and a demo seed that exercises all three instruction-file targets (CLAUDE.md, AGENTS.md, GEMINI.md).

## Goals

- The portal should look like a polished admin console, not an unstyled prototype.
- The demo should show rule packs landing in every instruction file the major agents read: Claude Code (CLAUDE.md), Gemini CLI and Google Antigravity (GEMINI.md), and the cross-tool AGENTS.md family (Antigravity, Cursor, Kimi, Grok).
- No architecture changes: server-rendered Go templates, no client-side JavaScript, no new dependencies, no new routes or handlers.

## Non-goals

- Dark mode or `prefers-color-scheme` support.
- New pages, auth changes, or behavior changes.
- Configurable target filenames (Gemini's `context.fileName` setting).
- Webfonts. The system font stack stays.

## 1. Visual system (`internal/portal/web/static/style.css`)

Replace the current 11 `:root` variables with a small token system:

- Palette: near-white page background (`#f8f9fb` range), white card surfaces, deeper ink, one accent blue. Status colors keep their semantics (in-sync green, drifted amber, stale red, ungoverned gray) but get recalibrated fill/text pairs meeting WCAG AA contrast.
- Spacing on a 4px scale; radius and two shadow-level tokens.
- Type scale: 13/14/15/18/24px on the existing `system-ui` stack. `font-variant-numeric: tabular-nums` on metrics and table cells.
- Rules for every class the templates reference but the stylesheet currently lacks: `.num`, `.warn`, `.chips`, `.filters`. Styled `select` and `button` elements.

## 2. Layout shell and page composition

`layout.html` becomes a sidebar shell:

- Fixed left sidebar: brand mark and name at top, nav (Overview, Fleet, Rule Packs, Usage) with active-page highlight, org name pinned at the bottom.
- Scrolling content column with a max-width, page header from the existing `title`/`explainer` blocks.
- Active-nav state requires the layout to know the current page: add a field to the shared template data in `server.go`.

Per page (markup and CSS only, same handlers and routes):

- **Overview**: stat hero row with large status-tinted numbers, then posture and recent-activity sections.
- **Fleet**: styled filter row; table with row hover, status pills, monospace repo paths.
- **Packs**: cards with version badges. Pack view/edit keep their structure; the editor textarea gets proper monospace treatment and the diff view gets add/remove coloring.
- **Usage**: styled filter row; charts restyled per section 3.

## 3. Charts (`internal/portal/charts/charts.go`)

- Series palette and text styles updated to match the new tokens.
- Axis text muted, 11px; one or two more gridlines; slightly softer strokes.
- Charts remain deterministic server-rendered SVG. Golden files in `testdata/` are regenerated once and reviewed by eye before committing.

## 4. Demo seed (`internal/portal/seed/repos.go`)

`buildDemoRepo` currently writes a two-line CLAUDE.md placeholder. Instead, seed three files, each with 2-3 lines of realistic pre-existing user content:

- `CLAUDE.md`
- `AGENTS.md`
- `GEMINI.md`

The pre-existing content demonstrates the core renderer invariant during the publish-then-sync demo: the managed block lands alongside bytes esc never modifies.

Demo pack fragment front-matter changes from `targets: [claude, agents]` to `targets: [claude, agents, gemini]` so published rules reach all three files. No renderer changes: all three targets already exist in `internal/render/render.go`.

Also add a 2026-07-31 Google Antigravity entry to `docs/roadmap/vendor-guidance-tracking.md`: Antigravity reads project-level GEMINI.md (tool-specific, higher priority) and AGENTS.md (cross-tool), plus global copies under `~/.gemini/`; `.agent/rules/` holds additional workspace rules. Covering CLAUDE.md, AGENTS.md, and GEMINI.md therefore covers Antigravity with no new target.

Seeding stays create-if-missing: an existing `~/.escapement/server/demo-repo` keeps its old files until deleted. Document this in the changelog rather than adding refresh logic.

## Error handling

No new failure modes. Chart rendering, template execution, and seeding keep their existing error paths and exit-code mapping.

## Testing

- `go test ./...` passes; chart golden files regenerated deliberately, not by reflex.
- Existing template and e2e serve tests still pass; extend the seed test (or e2e assertions) to check all three instruction files exist in the demo repo and that synced output lands in each.
- Visual verification: `esc serve --demo`, load all pages plus pack view/edit/diff, confirm layout, active-nav state, pills, and chart styling.
