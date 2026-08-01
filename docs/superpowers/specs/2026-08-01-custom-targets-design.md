# Data-driven targets: pack-defined custom targets and a portal Targets page

Status: revised draft, 2026-08-01. Incorporates architecture review, security review, and design-team review findings (see §10).

Makes the renderer's target list data-driven so packs can define custom instruction-file targets, and adds a portal Targets page that documents every target (built-in and custom), links each to its official vendor docs, offers per-target suggestions, and lets an admin add or edit custom targets through the existing publish flow.

## Goals

- Surface what each target is: which file, which agent tools read it, official docs, guidance.
- Let organizations define new targets in a pack (for example `.github/copilot-instructions.md` for GitHub Copilot, `QWEN.md` for Qwen Code) without waiting for an esc release.
- Support layering: an organization publishes a base pack (its enforced floor), and teams or projects add their own packs on top. Any pack in a repo's config can define targets; the effective set is a deterministic, ownership-respecting union.
- Keep the open core standing alone: definitions live in `pack.yaml` in the pack's git repo. The portal is an editing UI over that file, never the system of record.

## Non-goals

- Renaming or reconfiguring the six built-in targets (claude, agents, gemini, governance, skills, mcp). Their files and semantics stay compiled in.
- Custom whole-file, skills-dir, or MCP-config targets. Custom targets are managed-block markdown files only.
- Per-repo target definitions in `.escapement/config.yaml`. Repos acknowledge custom targets (§2.3) but never define them.
- Portal-side enforcement of which packs a repo must include. A compliance view of "repos missing the org base pack" is future fleet work (§9).

## 1. Data model

### 1.1 Built-in target metadata (compiled in)

A new `internal/targets` package owns target metadata, replacing the bare name-to-file map in `internal/render`:

```go
type Info struct {
    Name        string   // "claude"
    File        string   // "CLAUDE.md" (empty for skills/mcp)
    Kind        string   // "managed-block" | "whole-file" | "skills-dir" | "mcp-config"
    Readers     []string // agent tools that read it: "claude-code", "antigravity", ...
    DocURLs     []string // official vendor documentation
    Description string   // one paragraph: what this file is
    Suggestion  string   // esc's guidance: when to target it, what belongs in it
    BuiltIn     bool
    OwnerPack   string   // defining pack name; empty for built-ins
}
```

Reader and doc content is sourced from `docs/roadmap/vendor-guidance-tracking.md` and maintained on that doc's cadence. Initial table:

| Target | File | Readers | Docs |
|---|---|---|---|
| claude | CLAUDE.md | Claude Code | Anthropic CLAUDE.md docs |
| agents | AGENTS.md | Antigravity, Cursor, Codex, Gemini CLI, Kimi, Grok (AAIF standard) | agents.md / AAIF |
| gemini | GEMINI.md | Gemini CLI, Antigravity | google-gemini/gemini-cli gemini-md docs |
| governance | GOVERNANCE.md | humans, audit | (esc docs) |
| skills | .claude/skills/ | Claude Code | Anthropic skills docs |
| mcp | MCP config | MCP-capable tools | modelcontextprotocol.io |

### 1.2 Pack-defined custom targets

`pack.yaml` grows an optional `custom_targets` section. The key is deliberately not `targets`, which already means a selector in fragment front-matter and a filter in repo config; a third meaning under the same name invites misconfiguration.

```yaml
schema: 1
name: acme-org
version: 1.5.0
custom_targets:
  - name: copilot                       # usable in THIS pack's fragment front-matter
    file: .github/copilot-instructions.md
    doc: https://docs.github.com/en/copilot/customizing-copilot
    description: Repository custom instructions for GitHub Copilot.
rules: [rules/secrets.md]
```

- `name`, `file` required; `doc`, `description` optional.
- Custom targets are always `Kind: managed-block`. All renderer invariants apply unchanged: bytes outside the managed block are never touched, writes are atomic, nothing writes after a verification or constraint failure, output is deterministic.
- **A custom target is owned by its defining pack.** Only fragments of the owning pack can render into it, and only when they name it explicitly (§3).
- Fragments name custom targets explicitly: `targets: [claude, agents, copilot]`. A fragment with no `targets:` front-matter means **all built-in targets only** - never custom targets, not even the owning pack's. This keeps adding a custom target from silently dumping every existing fragment into a new file, and makes custom-target content an enumerable, auditable set.

Schema stays 1. `pack.yaml` decoding uses `KnownFields(true)` (pack.go:99), so an older esc rejects a pack using `custom_targets:` with an explicit parse error. For a governance tool, failing closed and loudly is correct; the error message and changelog tell admins to upgrade esc before adopting custom targets.

## 2. Validation and security

Custom targets turn pack content into a file-placement decision. Marginal risk: a pack could already inject text into built-in files via fragments; custom targets newly let a pack choose *which path* an esc-managed block lands in. Markdown is precisely the surface agents execute, and markdown transcludes (this repo's own CLAUDE.md is `@AGENTS.md`), so filename rules alone cannot bound the blast radius. Defense is layered: strict path rules (2.1), ownership and collision rules (2.2), and repo-side acknowledgment (2.3).

Validation is centralized in `targets.ValidateCustom`, shared verbatim by the CLI and the portal, and runs **after `verifyTrust` and before any write** - custom target definitions are parsed from verified pack content only, never during fetch.

### 2.1 Name and file rules

Name:
- matches `^[a-z][a-z0-9-]{0,31}$`
- must not equal any built-in target name

File:
- ASCII only, from `[A-Za-z0-9._/-]`; no segment ends with a dot or space (Windows normalization)
- a clean, relative, slash-separated path: no `..` segments, no leading `/`, no `\`, no empty segments; `filepath.Clean` must be a no-op; the joined path must remain inside the repo root by `filepath.Rel` check
- must end in `.md`
- at most 2 path segments (`QWEN.md`, `.github/copilot-instructions.md`; not `a/b/c.md`)
- **denylist of control directories** as the first segment: `.git`, `.escapement`, `.claude`, `.gemini`, `.cursor`, `.codex`, `.agent`, `.agents`, `.vscode`, `.idea`, `.github/workflows` - except the explicit carve-out `.github/*.md`. Rationale: `.claude/commands/foo.md` is executed as a slash command; agent-control dot-dirs are instruction surfaces with semantics beyond "a markdown file".
- **all file comparisons (built-in collision, cross-pack collision, denylist) are performed on the NFC-normalized, case-folded path.** `claude.md`, `Claude.MD`, and decomposed-Unicode variants all collide with `CLAUDE.md`. Since `file` is ASCII-only, NFC is a no-op on valid input but the comparison rule is stated for the validator anyway.
- must not equal (case-folded) a built-in target's file

Write-time (engine `apply` phase, not plan phase - a plan-time check is stale by apply):
- immediately before each write, every path component is checked with `Lstat`; a symlinked parent directory or target file is a constraint failure (exit 1), never followed. A residual local race between Lstat and rename remains on shared checkouts; documented as accepted (same class as any local tooling).
- rendered custom files respect `constraints.max_file_bytes` and `forbidden_patterns` exactly like built-ins.

### 2.2 Ownership and collision rules (deterministic, order-independent)

- Each custom target belongs to exactly one pack. **There is no silent dedup**: any two packs in a repo's effective set declaring the same `name` (case-folded) or the same `file` (case-folded) is a constraint failure naming both packs, sync refuses (exit 1). A shared target would otherwise let a team pack gain write access to an org file by re-declaring it.
- If an org and a team genuinely want to co-write one file, they put the fragments in the same pack. Co-writable targets across packs are explicitly out of scope.

### 2.3 Repo-side acknowledgment

`.escapement/config.yaml` gains:

```yaml
allow_custom_target_files:
  - .github/copilot-instructions.md
```

A custom target renders only if its `file` appears (case-folded match) in the repo's `allow_custom_target_files` list. Otherwise sync fails with a constraint error naming the pack, target, and the config line to add - fail closed, not skip, so governance drift is visible, and `esc status` reports it.

Rationale: pinning a pack today grants it write access to a *known, fixed* set of files. Custom targets would silently expand that grant to pack-chosen paths; the acknowledgment list restores the property that a repo's write surface is enumerable from its own config. This matches the product's consent posture (no standing write path; repo owners run esc). Fleet rollout cost is one config line per repo per new file, which is the point.

### 2.4 Trust

Target definitions are part of pack content, covered by existing signature verification; `trust: unsigned` packs may define custom targets, because 2.3 makes the repo, not the pack, the authority on write paths. Sentinel errors: all 2.1-2.3 violations are constraint failures (exit 1); signature problems stay exit 3.

## 3. Engine and renderer changes

- Effective target set per repo = built-ins ∪ custom targets from every configured pack, after 2.2 collision checks and 2.3 acknowledgment.
- **Pack-scoped compose.** `render.Compose` and `fragmentApplies` (render.go:42) are currently pack-agnostic; both must learn target ownership. For a custom target, compose consumes only the owning pack's fragments, and only those that name the target explicitly. Fragment `targets:` names are validated against built-ins plus the *defining pack's own* custom targets; referencing an unknown or foreign name is a constraint failure naming fragment and pack.
- **Catalog carve-out.** `composeCatalog` merges every pack's catalog into rendered blocks. Custom target files receive **no catalog section** - only the owning pack's explicitly-targeted fragments plus the standard managed-block header. The org-wide catalog stays an org-wide concern in built-in files.
- Repo config `targets:` (the existing filter) applies to the effective set by name; filtering out a custom target skips rendering it.
- **Orphan block removal is new code, not existing behavior.** Today apply removes only orphaned `esc-` skills dirs (apply.go:94-110); stale managed *blocks* are never removed. New behavior: when a target leaves the effective set (pack version drops it, repo un-acknowledges it, filter excludes it), sync removes the stale managed block from that file, leaving all other bytes; if removal leaves the file byte-empty, the file is deleted. Block removal is a write and carries the full 2.1 write-time protections. `esc status` reports files carrying esc blocks for targets no longer in the effective set.
- Diff/status/verify iterate the effective set everywhere `allTargets` is used today (engine.go:54, diff.go:49).

## 4. Portal Targets page

New sidebar entry **Targets** between Rule Packs and Usage. Server-rendered, no JS, same shell and tokens as the refreshed portal.

- **Built-in targets** (orientation layer, first): one full-width row card per target - the 180px card grid cannot hold this content. Left: name, file in monospace, reader chips (reuses `.chips`). Right: description, suggestion, doc links. Doc links show their host (`docs.github.com`, `agents.md`), not a bare "docs".
- **Custom targets** below, grouped by defining pack: name, file, description, doc link, fragment-usage count, per-row Edit link. Usage line on every target ("targeted by N fragments across M packs") ties the learn job to the manage job.
- **One primary action**: a single Add custom target button in the custom-section header.
- **Add/Edit form** reuses the fragment publish flow verbatim: fields (pack selector over writable packs, name, file, doc URL, description), then Preview diff → Publish with a version bump, same error banner. UI copy states explicitly that adding a target publishes a new pack version - that is the governance story, not friction. Validation errors come from the shared `targets.ValidateCustom` with the exact CLI message. The page also surfaces, per custom target, which fleet repos have acknowledged its file (from registry sync data) so an admin sees rollout progress.
- **States**: empty custom section ("No custom targets yet" + one-line explainer + Add button); read-only state when no writable pack is configured (form hidden, hint shown); form error banner.
- No portal-only state anywhere; every edit lands in the pack repo.

## 5. Layering story (org base + team additions)

No new mechanism: a repo's `.escapement/config.yaml` lists the org base pack plus team or project packs. Each pack may define custom targets; §2.2 keeps the union deterministic and ownership-bound, §2.3 keeps the repo's write surface explicit. The org base's targets reach every repo that pins the base pack and acknowledges the files; a team pack's targets reach only that team's repos. Recommended layout, documented in README:

```yaml
packs:
  - {source: git@github.com:acme/esc-org-base, ref: v2.1.0}      # org floor
  - {source: git@github.com:acme-payments/esc-team, ref: v0.3.0} # team additions
targets: []                                                       # optional filter
allow_custom_target_files:
  - .github/copilot-instructions.md                               # org base's target
  - PAYMENTS-AGENTS.md                                            # team's target
```

## 6. Demo seed

The demo pack gains one custom target (`copilot` → `.github/copilot-instructions.md`) with one fragment naming it; the demo repo config acknowledges the file and seeds a pre-existing `.github/copilot-instructions.md` with user content, so the Targets page, the acknowledgment mechanism, and the publish-to-sync loop all show end to end.

## 7. Error handling

All new failures route through existing sentinels: constraint (exit 1) for name/file/collision/acknowledgment/unknown-reference violations, integrity (exit 3) unchanged, usage (exit 2) unchanged. The portal maps validation errors to form banners; sync stays all-or-nothing (existing verification step covers the expanded set, and validation ordering per §2 preamble).

## 8. Testing

- Table-driven unit tests for `targets.ValidateCustom`: every 2.1 rule, including traversal (`../x.md`, `a/../../x.md`), absolute paths, backslashes, control-dir denials (`.claude/commands/x.md`, `.github/workflows/x.md`), the `.github/*.md` carve-out, depth 3, non-md, trailing dot/space, case-fold collisions (`claude.md`, `Claude.MD`), non-ASCII rejection.
- Engine integration (real temp git repos): multi-pack union, both collision classes, ownership (foreign fragment naming another pack's target fails; empty front-matter never reaches custom targets), catalog carve-out, acknowledgment gate (missing entry fails closed with the right message), repo-filter interaction, orphan block removal incl. byte-empty deletion, symlinked-parent refusal, deterministic output.
- Renderer golden test for a custom target file (no catalog section).
- Portal handler tests: page renders built-ins, customs, empty and read-only states; add/edit round-trip produces the expected `pack.yaml` diff; invalid input surfaces the validator's message verbatim.
- e2e: demo publish flow adds a custom target; `esc sync` fails closed before acknowledgment, succeeds after, landing the managed block beside seeded content.

## 9. Phasing and future notes

- Plan 1 (core): §1.2, §2, §3 plus CLI-visible behavior and tests. Shippable alone; custom targets fully usable from git without the portal.
- Plan 2 (portal): §1.1 metadata table, §4 page, §6 demo, portal tests.
- Future, out of scope: fleet compliance view "repos not pinning the org base pack / not acknowledging its targets"; per-target adoption stats from telemetry; Gemini `context.fileName`-style renames of built-ins; co-writable shared targets.

## 10. Review findings incorporated

- Architecture review: silent dedup removed in favor of single-owner collision errors (C1); pack-scoped `Compose`/`fragmentApplies` named as required changes (C2); case-fold/NFC comparisons (I1); control-dir denylist beyond `.git` (I2); `custom_targets` key naming (I3); write-time symlink checks with documented residual race (M1); custom targets made explicit-opt-in for fragments (M3).
- Security review: repo-side `allow_custom_target_files` acknowledgment as the primary boundary, motivated by markdown transclusion (C1); case/Unicode normalization (I1); Lstat moved to the apply phase per component (I2); orphan block removal identified as new code and specced (I3); catalog carve-out (M1); validation-after-verifyTrust ordering stated (M3).
- Design team: full-width built-in rows; explicit empty/read-only/error states; publish-flow-verbatim add/edit with version-bump copy; doc links show host; single primary Add action; built-ins-first ordering.
