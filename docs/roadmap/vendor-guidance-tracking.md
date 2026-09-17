# Vendor guidance tracking

The instruction files this product renders (CLAUDE.md, AGENTS.md, GEMINI.md, skills, MCP config) are conventions owned by model vendors, and the guidance for writing them shifts with each model generation. Example: Anthropic's "The new rules of context engineering for Claude 5 generation models" (claude.com/blog, 2026-07-24) reversed several earlier best practices. Claude 5-generation models want lightweight, gotcha-focused instruction files with progressive disclosure (point to deeper docs instead of inlining everything), no repeated or ALL-CAPS emphasis, and no restating of things discoverable from the repo itself.

This matters to us twice over:

1. Our own repo instruction files should follow current guidance (done 2026-07-28 for the Claude 5 rules: AGENTS.md is canonical, CLAUDE.md imports it).
2. The renderer targets, defaults, and pack-authoring guidance we ship encode assumptions about what agents read and how much of it they want (e.g. `max_file_bytes`, catalog directive verbosity, which target files exist at all). Stale assumptions degrade every pack our users deploy.

## Practice

Two tiers, both run by the `/sweep-sources` skill in this repo (`.claude/skills/sweep-sources/SKILL.md`). With no argument the skill runs whatever is due; `weekly`, `deep`, or `all` forces a tier.

- **Weekly, community signal.** Scan the community sources listed under Sources for instruction-file convention changes (new files, renames, hierarchy or size-cap changes), context and prompting guidance changes, new agent targets worth rendering to, and major model-generation releases. Ignore hype, benchmarks, and product drama. Community sources are signal only: every candidate claim is verified against the vendor's primary source before it enters this log or drives any change. Secondary coverage has already been wrong twice (a stale Grok size cap and a wrong AAIF date, both caught in the 2026-07-30 sweep).
- **Monthly, deep vendor check.** Read the primary sources for every vendor below, and again at every major model-generation release. Quarterly was judged too slow for the current pace of change (decision 2026-08-08).

**Core vendor set** — always tracked, and always shown top-level in the admin portal's guidance-freshness view (spec §3, Pillar B) regardless of whether a given org uses them:

- **Anthropic / Claude** — claude.com/blog, anthropic.com/engineering (context engineering series), Claude Code docs
- **OpenAI** — the AGENTS.md convention (agents.md; stewarded by the Agentic AI Foundation under the Linux Foundation since 2025-12-09), Codex docs, OpenAI cookbook
- **Google** — Gemini CLI and GEMINI.md docs
- **xAI** — Grok Build / grok-cli docs
- **Moonshot** — Kimi Code docs

**Demand-driven:** any other agent that shows up in user demand for new render targets (Cursor, etc.).

**Also watched, not a render target:** GitHub Copilot's enterprise managed settings. It governs client configuration rather than instruction files, so it changes nothing in the renderer, but it is the one place a vendor is shipping into our lane. Track it here for competitive movement (see the 2026-08-11 entry) rather than for guidance changes.

**Also tracked for the model registry, not a render target:** DeepSeek and Z.ai. The model registry (`internal/guidance/models/`) and the example model packs (`examples/packs/*-models/`) carry their lineups and prices, so the deep check reads their primary sources for model releases, retirements, and price changes only.

Record each sweep in the log below, newest first, tagged with its tier: `- **YYYY-MM-DD** — [weekly] ...` or `- **YYYY-MM-DD** — [deep] ...`, then source, what changed, and action taken. A sweep that finds nothing still gets a one-line `no signal` entry, because the skill computes what is due from the newest tagged entry per tier (a `[deep]` entry also satisfies the weekly tier). Entries without a tag predate the tiers and do not count toward a due date. Every sweep ends with a verdict line, `packs need updating: yes` or `no`, with reasons. If a change affects render targets, renderer defaults, or the model registry, it goes through the normal spec → design-doc cycle rather than being patched ad hoc.

The control plane surfaces this log as a product feature: the admin portal shows the last-checked date per vendor, so admins can see how current the render-target assumptions behind their deployed packs are. The core vendor set above is the minimum always-visible row set in that view.

## Sources

### Primary sources per vendor

- **Anthropic / Claude** — claude.com/blog (context engineering posts), Claude Code docs
- **OpenAI / Codex** — AGENTS.md guide: <https://developers.openai.com/codex/guides/agents-md>; convention site: <https://agents.md>; donation announcement: <https://openai.com/index/agentic-ai-foundation/>
- **AGENTS.md governance** — Agentic AI Foundation: <https://aaif.io>; formation press release (2025-12-09): <https://www.linuxfoundation.org/press/linux-foundation-announces-the-formation-of-the-agentic-ai-foundation>
- **Google / Gemini CLI** — GEMINI.md docs: <https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/gemini-md.md>
- **xAI / Grok Build** — AGENTS.md / project rules: <https://docs.x.ai/build/features/project-rules>
- **Moonshot / Kimi Code** — agents docs: <https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/customization/agents.md>; skills docs: <https://github.com/MoonshotAI/kimi-code/blob/main/docs/en/customization/skills.md>
- **GitHub / Copilot** — changelog: <https://github.blog/changelog/>; enterprise managed settings reference: <https://docs.github.com/copilot/reference/enterprise-managed-settings-reference>
- **DeepSeek** — API news and model releases: <https://api-docs.deepseek.com/news>
- **Z.ai / GLM** — developer docs and model pages: <https://docs.z.ai/>

### Community signal (weekly tier)

Signal only; nothing enters the log from these without primary-source verification. Subreddit names are themselves a moving target: when one renames or dies, the sweep corrects this list.

- Anthropic: r/Anthropic, r/ClaudeAI, r/ClaudeCode
- OpenAI: r/OpenAI, r/ChatGPTCoding
- Google: r/Bard, r/GeminiAI
- xAI: r/grok
- DeepSeek: r/DeepSeek
- Cross-vendor: r/LocalLLaMA (also covers Moonshot/Kimi and Z.ai, neither of which has an active English subreddit), Hacker News via the Algolia search API (<https://hn.algolia.com/api/v1/search_by_date>)

## Log

- **2026-09-16** — Anthropic and OpenAI model registry refresh (`internal/guidance/models/`). Added `claude-fable-5-1` (successor to Fable 5, same tier and price; always-on thinking; 30-day retention; classifier refusals) and `claude-opus-4-8`; moved `claude-fable-5` to legacy; corrected Sonnet 5 to its now-permanent $2 / $10 (Anthropic's August edit to the launch post withdrew the $3 / $15 list price). Added `gpt-6-astra` (launched 2026-09-03, $10 / $50 standard, 1.05M context) and narrowed `gpt-5.6-sol` to review. The frontier routing now prefers Fable 5.1, with Opus 4.8 as the Opus-tier choice and Opus 5 review-required on the maintainers' logged evidence, with a measurable reversal condition recorded on the guidance page and mirrored in `examples/packs/anthropic-models`. Sources on each vendor page.
- **2026-08-11** — GitHub / Copilot, "Enterprise managed settings in the GitHub Copilot app and Copilot cloud agent" (github.blog changelog, published 2026-07-27). Not instruction-file guidance; logged as market evidence that a first-party vendor is now shipping centrally administered, client-enforced agent governance. Enterprise owners write one `copilot/managed-settings.json` in a `.github-private` repo (MDM or a distributed file are the alternatives); the Copilot app, cloud agent, CLI, and VS Code all read it, and the managed value takes precedence over anything a developer sets locally. Governed keys: which plugins and which plugin marketplaces are allowed, whether a developer may bypass approval prompts before Copilot runs a command / reads a file / fetches a URL, and the default model-selection mode. Clients pick up changes within about an hour, or on restart or re-auth. The post's framing is our own thesis in GitHub's words: "any client that sits outside your policy is a gap," and "your governance is only as strong as its least-covered surface."
  - **Validates:** the problem is real enough for the largest incumbent to ship against it and sell it to enterprises now, and central-over-local precedence is settled as the expected behavior rather than something we have to argue for.
  - **Does not cover (our lane):** single-vendor by construction (Copilot clients only, nothing for Claude Code / Codex / Gemini / Cursor in the same repo — the exact heterogeneity spec §2 is about); governs *client settings*, not the instruction content agents actually read; closed and hosted, with no pack format, no versioning or signing of what gets applied, and no drift reporting on the developer's checkout.
  - **Watch:** GitHub extending managed settings to cover instruction files is the nearest competitive pressure. Re-read the enterprise managed settings reference for new keys at each check. No renderer changes.
- **2026-07-31** — Google / Antigravity (Antigravity docs, GEMINI.md + AGENTS.md conventions). Google Antigravity reads a project-level `GEMINI.md` as its tool-specific, higher-priority instruction file, plus the cross-tool `AGENTS.md` family, and honors global copies under `~/.gemini/`. Additional workspace rules live in `.agent/rules/`. There is no `ANTIGRAVITY.md` target: covering CLAUDE.md, AGENTS.md, and GEMINI.md already covers Antigravity, so the demo seed now publishes to all three. No renderer changes; all three targets already ship in `internal/render/render.go`.
- **2026-07-30** — OpenAI / Codex (developers.openai.com/codex/guides/agents-md, agents.md). AGENTS.md was donated to the Agentic AI Foundation (a Linux Foundation directed fund) at the AAIF's formation on 2025-12-09, alongside Anthropic's MCP and Block's goose — neutral multi-vendor stewardship, verified against the Linux Foundation press release. Codex, Cursor, Copilot, Gemini CLI, and others align on it. Codex builds an instruction chain (global `$CODEX_HOME/AGENTS.override.md` or `AGENTS.md`, then project root walked down to cwd, concatenated), default cap 32 KiB (`project_doc_max_bytes`). Official guidance matches the Claude 5 rules: keep it minimal, don't restate repo-derivable content; auto-generated files measurably hurt. No renderer changes; the consolidation strengthens the case for AGENTS.md-first render defaults in v0.2.
- **2026-07-30** — Google / Gemini CLI (gemini-cli docs, gemini-md.md). GEMINI.md loads hierarchically (global `~/.gemini/GEMINI.md`, project + ancestor dirs, component-local dirs on file access), supports modular imports via `@file.md`, and the filename is configurable (`context.fileName` in settings.json). The import syntax is a candidate render mode (fragment files + imports instead of one managed block). No action yet; consider when writing v0.2 renderer docs.
- **2026-07-30** — Moonshot / Kimi Code (MoonshotAI/kimi-code docs). Uses AGENTS.md, not a KIMI.md: global `~/.kimi-code/AGENTS.md`, cross-tool `~/.agents/AGENTS.md`, project-level `AGENTS.md` or `.kimi-code/AGENTS.md`; MCP config is compatible with Claude Code's `.mcp.json`. Our existing AGENTS.md and `.mcp.json` targets already cover it. Watch `~/.agents/` as an emerging cross-tool home-dir location.
- **2026-07-30** — xAI / Grok Build (docs.x.ai project-rules). No GROK.md; reads the AGENTS.md family plus CLAUDE.md/CLAUDE.local.md and `.grok/rules/*.md` (also `.claude/rules/` and `.cursor/rules/` for compatibility); hierarchy is global `~/.grok/AGENTS.md` → repo root → cwd, deeper wins. Files are loaded in full with no size cap (verified against docs.x.ai 2026-07-30; a 10,000-char cap reported by secondary sources is outdated). Existing CLAUDE.md/AGENTS.md targets cover it.
- **2026-07-28** — Anthropic, "The new rules of context engineering for Claude 5 generation models" (published 2026-07-24). Rewrote this repo's own instruction files to comply: AGENTS.md created as the canonical agent instructions, CLAUDE.md reduced to an import of it, content trimmed to gotchas plus pointers. No renderer changes yet; revisit pack-authoring guidance against these rules when writing v0.2 docs.
