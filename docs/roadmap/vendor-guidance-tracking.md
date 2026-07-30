# Vendor guidance tracking

The instruction files this product renders (CLAUDE.md, AGENTS.md, GEMINI.md, skills, MCP config) are conventions owned by model vendors, and the guidance for writing them shifts with each model generation. Example: Anthropic's "The new rules of context engineering for Claude 5 generation models" (claude.com/blog, 2026-07-24) reversed several earlier best practices. Claude 5-generation models want lightweight, gotcha-focused instruction files with progressive disclosure (point to deeper docs instead of inlining everything), no repeated or ALL-CAPS emphasis, and no restating of things discoverable from the repo itself.

This matters to us twice over:

1. Our own repo instruction files should follow current guidance (done 2026-07-28 for the Claude 5 rules: AGENTS.md is canonical, CLAUDE.md imports it).
2. The renderer targets, defaults, and pack-authoring guidance we ship encode assumptions about what agents read and how much of it they want (e.g. `max_file_bytes`, catalog directive verbosity, which target files exist at all). Stale assumptions degrade every pack our users deploy.

## Practice

Roughly quarterly, and at every major model-generation release, check for updated context/prompting guidance and new or changed instruction-file conventions from:

- **Anthropic / Claude** — claude.com/blog, anthropic.com/engineering (context engineering series), Claude Code docs
- **OpenAI** — the AGENTS.md convention (agents.md), Codex docs, OpenAI cookbook
- **Google** — Gemini CLI and GEMINI.md docs
- **xAI** — Grok / grok-cli docs
- **Moonshot** — Kimi docs
- Any other agent that shows up in user demand for new render targets (Cursor, etc.)

Record each check in the log below (date, source, what changed, action taken). If a change affects render targets or renderer defaults, it goes through the normal spec → design-doc cycle rather than being patched ad hoc.

## Log

- **2026-07-28** — Anthropic, "The new rules of context engineering for Claude 5 generation models" (published 2026-07-24). Rewrote this repo's own instruction files to comply: AGENTS.md created as the canonical agent instructions, CLAUDE.md reduced to an import of it, content trimmed to gotchas plus pointers. No renderer changes yet; revisit pack-authoring guidance against these rules when writing v0.2 docs.
