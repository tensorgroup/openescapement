# OpenEscapement

Deterministic governance for AI usage: the `esc` CLI syncs versioned, signed rule packs into the instruction files coding agents natively read (CLAUDE.md, AGENTS.md, GEMINI.md, skills dirs, MCP config). v0.1 of the CLI is implemented; the registry, dashboard, MCP server, and telemetry pillars are future work.

These are the canonical instructions for any coding agent working in this repo. `CLAUDE.md` imports this file; edit here, not there.

## Build, test, lint

- Go 1.24, module `github.com/tensorgroup/openescapement`, binary `esc` (`cmd/esc`)
- `go test ./...` — integration tests build real temp git repos, no mocks. `ESC_CACHE_DIR` overrides the pack cache; tests rely on it.
- `go vet ./...` and `gofmt -w .` before claiming work done

## Gotchas and invariants

- **Single external dependency policy:** `gopkg.in/yaml.v3` only. Everything else is stdlib; system `git` via `os/exec`. This is a security posture (this tool writes instructions agents execute), not a preference. Adding a dependency needs explicit justification.
  - Exception (display-only): `github.com/yuin/goldmark` renders markdown for the admin portal's browser views. Scope is strictly display: it never runs in the renderer or engine that writes instruction files. It carries zero transitive dependencies (empty require graph), is pinned by `go.sum`, and runs with raw HTML disabled (no `html.WithUnsafe`). Any wider use, or any second display dependency, needs the same explicit justification.
- Sentinel errors in `internal/esc` map to exit codes: 0 ok, 1 drift/constraint, 2 usage, 3 integrity/signature, 4 other. New failure modes go through them.
- Renderer invariants, all golden-testable: bytes outside a managed block are never modified; nothing is written after a verification or constraint failure; all writes are atomic; renderer output is deterministic.

## Deeper context, read on demand

- `ai-governance-product-spec.md` — canonical product spec (Draft v2). Read before any product or design discussion; it also defines the product boundaries (what this is *not*). Open questions are in §11.
- `docs/superpowers/specs/` — design docs from the spec → plan → implementation cycle
- `docs/roadmap/`, `docs/strategy/` — future pillars, licensing and open-core reasoning
- `docs/roadmap/vendor-guidance-tracking.md` — cadence for checking model vendors' instruction-file and context guidance (the format of this very file is a moving target)

## Working conventions

- Spec-driven: substantive product or architecture decisions get captured in the spec or a design doc, not left in conversation.
- Open-core: the pack format, renderer, CLI, and base MCP server must stand alone without the SaaS control plane.
- YAGNI ruthlessly. Lightweight over complete, privacy-respecting by default (no prompt-content collection), self-hostable, boring and fast.
