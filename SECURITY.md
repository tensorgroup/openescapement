# Security Policy

OpenEscapement writes instructions that AI agents execute, so its distribution path is a high-value target. We take that seriously and design fail-closed.

## Reporting a vulnerability

Email **security@tensorgroup.example** (placeholder — replace before public release) with details and a proof of concept if you have one. Please do not open public issues for undisclosed vulnerabilities. We aim to acknowledge within 3 business days.

## Threat model

The primary threat is **compromise of the policy-distribution path** — an attacker who can publish or alter a rule pack could deploy malicious or dangerous instructions everywhere the pack is consumed. Defenses:

- **Signature verification** before any pack content is used (SSH `allowed_signers`); unsigned sources require an explicit, greppable `trust: unsigned`.
- **Hash-pinned lockfile**: a moved tag or tampered fetch fails closed (exit 3) before any file is written.
- **Path containment**: pack-controlled names and file entries cannot escape the pack directory or the governed repo; symlinks in packs are rejected.
- **Argument-injection hardening**: refs/sources are charset-validated and passed to `git` after a `--` separator.
- **Constraint gate**: pack `forbidden_patterns`/`max_file_bytes` are validated against *all* rendered content — agent files, skill files, and MCP entries — before writing.
- **No standing write path**: no daemon or server pushes into repos; only `esc`, run by someone with write access, changes files.

## Known limitations (v0.1)

Tracked for hardening; documented rather than hidden:

- **First-sync TOFU.** The content hash of a pack is trusted on first sync (signatures are still verified). Integrity pinning kicks in on subsequent syncs. Pin to signed tags, not branches, for the strongest guarantee.
- **Branch/SHA refs bypass version-tag checks.** Branch refs are a moving target by design; prefer immutable signed tags.
- **MCP command entries are not sandboxed.** A pack can declare MCP servers that your agent runtime may launch. `esc` refuses to overwrite servers it doesn't own and surfaces what it adds, but review pack-provided MCP entries as you would any dependency.
- **Binary release signing** (cosign/SLSA provenance) lands with the first tagged release; until then, build from source.
