# OpenEscapement v0.1 — Rule-Pack Format, Renderer, and CLI Design

**Status:** Approved (brainstorm 2026-07-07)
**Scope:** The full "Dependabot-meets-dotfiles" loop: pack format + renderer + CLI sync/drift.
**Parent spec:** `../../../ai-governance-product-spec.md`

## Summary

`esc` is a single Go binary that syncs versioned policy "rule packs" from git repos into the instruction files AI agents natively read (CLAUDE.md, AGENTS.md, GEMINI.md, `.claude/skills/`, `.mcp.json`) plus a human-readable `GOVERNANCE.md`. Policy is deterministic literal text; distribution is git-native, signature-verified, and hash-pinned; drift is detected and optionally gated in CI. No server, no daemon, no standing write path.

## Decisions (from brainstorm)

| Decision | Choice |
|---|---|
| First slice | Full loop: format + renderer + CLI sync/drift |
| Output model | Managed blocks inside team-owned files |
| Pack format | YAML manifest (`pack.yaml`) + markdown rule fragments |
| Distribution | Git-native: repo + ref, hash-pinned lockfile; registry later |
| Drift stance | Detect by default; `esc status --check` opt-in CI gate; no auto-remediation |
| Stack | Go (supply-chain posture: signed static binary, minimal deps) |
| Render targets (v0.1) | CLAUDE.md, AGENTS.md, GEMINI.md, `.claude/skills/`, `.mcp.json`, GOVERNANCE.md |
| Sync scope | Repo-level only; home-dir sync deferred to v0.2 |
| CLI shape | Stateless verb CLI; all state in two committed files (config + lock) |
| Git operations | Shell out to system `git` (inherits user auth for private repos; enables `verify-commit`/`verify-tag`) |
| Name | Project: OpenEscapement. Binary: `esc`. An escapement converts an unregulated power source into precise, countable ticks. |

## Architecture

Two repo kinds:

- **Pack repo** — policy source of truth. One per org, subdirectory per pack (org-wide, per-department, per-stack). Versioned by git tags, signed.
- **Governed repo** — any repo consuming packs. Carries `.escapement/config.yaml` (intent), `.escapement/escapement.lock` (resolution), and rendered artifacts.

```
policy-packs/  (pack repo)              my-service/  (governed repo)
├─ org/                                 ├─ .escapement/
│  ├─ pack.yaml                         │  ├─ config.yaml
│  └─ rules/                            │  └─ escapement.lock
│     ├─ secrets.md                     ├─ CLAUDE.md         # managed block
│     ├─ authn.md                       ├─ AGENTS.md         # managed block
│     ├─ hosting.md                     ├─ GEMINI.md         # managed block
│     └─ ports.md                       ├─ GOVERNANCE.md     # fully managed
├─ eng-dept/                            ├─ .claude/skills/
│  ├─ pack.yaml                         │  └─ esc-<pack>-<skill>/  # whole-dir managed
│  └─ skills/vault-usage/SKILL.md       └─ .mcp.json         # managed entries
```

## Pack format

### pack.yaml

```yaml
schema: 1
name: acme-org
version: 1.4.0
description: Acme org-wide AI governance baseline

rules:                      # ordered markdown fragments → managed blocks
  - rules/secrets.md
  - rules/hosting.md

skills:                     # optional: dirs rendered into .claude/skills/
  - skills/vault-usage

mcp:                        # optional: entries merged into .mcp.json
  servers:
    acme-paved-path:
      command: npx
      args: ["-y", "@acme/paved-path-mcp"]

catalog:                    # structured tool/service allow-deny list
  - name: Tailscale
    category: hosting-exposure
    status: preferred       # preferred | allowed | review-required | banned
    notes: Org tailnet — join at tailscale.acme.edu
  - name: Raw port forwarding
    category: hosting-exposure
    status: banned

constraints:                # validated against the FINAL merged file
  max_file_bytes: 20000
  forbidden_patterns:
    - "ignore (the )?(above|governance)"
```

- `version` must match the git ref being consumed (tag `v1.4.0` ↔ `version: 1.4.0`); mismatch is a sync error.
- `catalog` renders twice: concise directive text in agent artifacts, readable table in GOVERNANCE.md. Structured so the future MCP server/dashboard can query it.

### Rule fragments

Plain markdown, optional frontmatter:

```markdown
---
targets: [claude, agents]    # omit = all targets. `governance` = GOVERNANCE.md only.
---
## Secrets
Use Vault at vault.acme.edu. Never write secrets inline.
```

### Policy domains (content taxonomy, not mechanism)

Build rules, SDLC rules, agent-behavior rules, **hosting rules** (platform hosting: Lovable/Vercel; model-vendor hosting; local hosting; exposure: Tailscale/Headscale preferred, Cloudflare Tunnel review-required, raw port forwarding banned — exact statuses are pack content, these are the starter-pack defaults), paved-path pointers.

### Composition

`config.yaml` lists packs in order; blocks render in listed order (org → department → team). v0.1 composition is concatenation — no override/merge semantics between packs. Team content outside the managed block is untouched; pack `constraints` validate the merged whole.

## Governed-repo files

### .escapement/config.yaml

```yaml
schema: 1
packs:
  - source: github.com/acme/policy-packs//org     # terraform-style //subdir
    ref: v1.4.0
  - source: ../local-packs/team                    # local paths supported (tests, examples)
    ref: ""                                        # local paths are unversioned
    trust: unsigned
allowed_signers_file: .escapement/allowed_signers  # ssh allowed_signers format
targets: [claude, agents, gemini, governance, skills, mcp]  # optional; default all
```

### .escapement/escapement.lock

Records, per pack: resolved commit SHA, content hash (SHA-256 over sorted relative paths + file bytes of the pack dir). Per rendered artifact: target path + expected content hash (for managed blocks: hash of block body; for whole files/dirs: file hashes; for `.mcp.json`: owned keys + their serialized values' hash). JSON, stable ordering, committed.

## Managed output mechanisms

1. **Managed block** (CLAUDE.md, AGENTS.md, GEMINI.md):

```markdown
<!-- escapement:begin packs=acme-org@1.4.0,eng-dept@2.1.0 hash=sha256:9f2c… -->
> Managed by escapement — do not edit. Run `esc diff` to see source. Team content goes outside this block.

…rendered policy text…
<!-- escapement:end -->
```

- Hash covers block body ⇒ tampering detectable from the file alone.
- No block present ⇒ append at end of file (create file if missing). A `<!-- escapement:block -->` placeholder positions it explicitly.
- Core invariant: **bytes outside the block are never modified.**

2. **Whole-file** (GOVERNANCE.md): fully rendered, header comment with provenance + hash.

3. **Whole-directory** (`.claude/skills/esc-<pack>-<skill>/`): copied from pack; `esc-` prefix marks ownership; hashes in lockfile. Directories we own that no longer appear in any pack are removed on sync.

4. **Per-key JSON merge** (`.mcp.json`): add/verify only pack-declared `mcpServers` entries; ownership recorded in lockfile; other keys never touched.

## CLI

| Command | Behavior |
|---|---|
| `esc init` | Scaffold `.escapement/config.yaml` (+ placeholder allowed_signers) |
| `esc sync` | Fetch pinned packs → verify signatures + lockfile → parse → compose → render → validate constraints → write atomically → update lock |
| `esc status [--check]` | Same pipeline read-only; classify artifacts: in-sync / modified / missing / stale (config pin ≠ lock) / constraint-violated. `--check` exits non-zero on any finding |
| `esc diff [--against REF]` | No flag: unified diff of expected vs actual artifacts (drift detail). `--against`: diff rendered policy text between locked version and REF (pre-update review) |
| `esc update [SOURCE] [--ref REF]` | Bump config pin, refresh lock (does not write artifacts; `sync` does) |
| `esc render --stdout` | Render to stdout without writing (debugging, previews) |
| `esc version` | Version + build provenance |

`sync` on first run with no lockfile creates it (TOFU for content hash; signatures still verified). Subsequent runs verify against lock.

## Security model

- **Trust anchors:** allowed_signers file (SSH/GPG keys) named in config; `esc sync` verifies the pinned tag/commit signature (`git verify-tag`/`verify-commit` with `gpg.ssh.allowedSignersFile`) before using content. Unsigned sources require explicit `trust: unsigned` — visible in review, greppable in audit.
- **Lockfile integrity:** re-pointed tag, tampered fetch, or hand-edited artifact each fail loudly with a specific error, before any write.
- **Explicit updates:** sync never auto-jumps versions; updates land via `esc update` + `esc diff --against` + PR review. Changelog/notification story rides on normal code review.
- **No standing write path:** only `esc` runs by actors with repo write access can change policy files. A future compromised control plane observes; it cannot distribute.
- **Binary supply chain:** signed releases (goreleaser + cosign, SLSA provenance), minimal vendored deps, reproducible builds.

## Error handling

Fail loudly, specifically, never destructively. Atomic writes (temp file + rename, same filesystem). Any integrity failure aborts before the first write with exactly what mismatched (expected vs got). Typed/sentinel errors per failure class (`ErrSignature`, `ErrLockMismatch`, `ErrConstraint`, `ErrDrift`…) so exit codes and CI messages are precise. `status` output is actionable per-artifact.

## Testing

- **Golden-file tests** for the renderer: fixture pack → expected artifacts, byte-exact. Determinism is the product claim; test it literally.
- **Table-driven unit tests:** manifest/fragment parsing, hashing, drift classification, constraint validation, block splice, JSON merge.
- **Integration tests** against real local fixture git repos (temp dirs): fetch/verify/sync/status/update cycles, signature-failure and tag-moved cases. No mocking git.

## v0.1 milestones

1. Pack parsing + renderer + managed-block writer, golden tests (markdown targets)
2. `init`/`sync`/`status`: git fetch, lockfile, drift classification
3. Signatures, `diff`, `update`, constraints, catalog + GOVERNANCE.md
4. Skills dirs + `.mcp.json` targets; GitHub Action wrapper (composite action running `esc status --check`)

## Deferred (explicitly)

Home-dir sync & daemon, telemetry, MCP server surface, registry/dashboard, auto-remediation (`sync --force`), pack override/merge semantics, hosted pack registry, OCI distribution, Windows support niceties beyond `filepath` correctness.
