# Anthropic

Anthropic's current lineup is four models across three tiers: Claude Fable 5.1 and Claude Opus 5.5 at the frontier, Claude Sonnet 5.5 in the middle, and Claude Haiku 4.5 for fast, high-volume work. Claude Opus 5, Claude Fable 5, Claude Opus 4.8, and Claude Sonnet 5 are still served but superseded. They fit an agentic SDLC cleanly: plan and review on the frontier tier, implement day-to-day on Sonnet, and run bulk or mechanical work on Haiku. Refreshed 2026-09-29: Sonnet 5.5 added as the Sonnet-tier choice; Sonnet 5 and Opus 5 moved to legacy, so the registry's current entries match Anthropic's models overview exactly. The 2026-09-22 refresh added Opus 5.5 and moved Opus 4.8 to legacy; the 2026-09-16 refresh added Fable 5.1 and corrected Sonnet 5's price.

## Models

### Claude Fable 5.1 (claude-fable-5-1) - frontier

Anthropic's most capable widely released model, the successor to Claude Fable 5 in the same tier at the same price. Thinking is always on; 1M-token context, 128K max output; $10 / $50 per million input/output tokens. Requires 30-day data retention, and its safety classifiers may decline security-adjacent work, so a fallback model belongs in every configuration that uses it. Recommended roles: planning, plan-check, and review. It is the default frontier choice for work where being wrong is expensive.

### Claude Opus 5.5 (claude-opus-5-5) - frontier

The Opus-tier model when Fable's price or its retention requirement is the constraint, and the model Anthropic now recommends as the starting point for most workloads. Thinking is always on and cannot be disabled; default effort is `medium`, one level below Opus 4.8 and Opus 5, so set it explicitly. Forced `tool_choice` (`any` or `tool`) returns a 400. 1M-token context, 128K max output; $4 / $20 per million tokens, cheaper than the Opus models it supersedes. Recommended roles: planning, plan-check, review, and coding for the agentic runs that need Opus-level judgment.

### Claude Opus 5 (claude-opus-5) - legacy

Superseded by Opus 5.5 at a lower price; Anthropic no longer lists it among current models. $5 / $25 per million tokens. The maintainers' logged agentic use (2026-09) found it weaker than Opus 4.8 at the same price. Migrate pins to Opus 5.5 with the same changes listed under Opus 4.8, and never use it as a fallback for a capped Fable 5.1 slot.

### Claude Fable 5 (claude-fable-5) - legacy

Superseded by Fable 5.1 in the same tier at the same price. Still served; migrate existing pins rather than starting new work on it.

### Claude Opus 4.8 (claude-opus-4-8) - legacy

The Opus-tier choice in this registry until Opus 5.5 shipped; Anthropic now lists it among legacy models. $5 / $25 per million tokens. Migrate pins to Opus 5.5, which costs less. When migrating, set effort explicitly (5.5 defaults to `medium`), drop any disabled-thinking setting, and replace forced `tool_choice` with `auto` and `strict: true` tools.

### Claude Sonnet 5 (claude-sonnet-5) - legacy

The Sonnet-tier choice in this registry until Sonnet 5.5 shipped; Anthropic no longer lists it among current models. $2 / $10 per million tokens, the same as Sonnet 5.5, so migrating costs nothing per token. Migrate pins to Sonnet 5.5 with the changes listed under it.

### Claude Sonnet 5.5 (claude-sonnet-5-5) - mid

Anthropic's best combination of speed and intelligence, the successor to Claude Sonnet 5 at the same price: 1M-token context, 128K max output, $2 / $10 per million tokens (cache reads $0.20), same tokenizer. Adaptive thinking with default effort `high`, but the effort levels are recalibrated from Sonnet 5, so re-run the effort sweep (start at `medium` for agentic coding and multi-step tool use, `low` for chat). Five breaking changes for code running on Sonnet 5: `thinking: {type: "disabled"}` returns a 400 (send `{type: "between_tools"}`, accepted at effort `high` or below with no other field); forced `tool_choice` (`any` or `tool`) returns a 400; thinking blocks are bound to the model and the conversation; on the Claude API and Google Cloud, computer use goes only through `computer_toolset_20260801`; and the advisor tool rejects Opus 4.8, Opus 4.7, and Sonnet 5 as advisors. Text between tool calls now arrives in `thinking` blocks, empty unless a `display` value returns it. Unlike Sonnet 5, it accepts mid-conversation system messages and task budgets. Recommended roles: coding and review. This is the default day-to-day coding model, the executor behind a written plan with tests, and the reviewer on lower-stakes diffs where a full frontier pass is not warranted.

### Claude Haiku 4.5 (claude-haiku-4-5) - fast

Anthropic's fastest model with near-frontier intelligence, built for high-volume, repetitive work: classification, extraction, mechanical edits, and simple agentic subtasks run thousands of times over. 200K-token context (smaller than the others), 64K max output, $1 / $5 per million tokens. The alias `claude-haiku-4-5` points at the snapshot `claude-haiku-4-5-20251001`; Anthropic lists its retirement as not sooner than 2026-10-15, so watch for its successor. Recommended role: bulk. It is the only current model without adaptive thinking, so keep it off tasks that need multi-step reasoning.

## Governance

- Require a human review gate before merging any change a frontier model produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs). Frontier-tier judgment is strong but not infallible, and the blast radius of an autonomous frontier session is larger than a supervised one.
- Route plan, plan-check, and review to Claude Fable 5.1, with a fallback configured for the requests its classifiers decline. Use Claude Opus 5.5 where Fable's price or retention requirement rules it out, and for long agentic runs, with effort set explicitly.
- Treat Claude Opus 5 and Claude Opus 4.8 as legacy and review-required. Opus 5.5 supersedes both at a lower price; migrate their pins rather than starting new work on them.
- Treat Claude Sonnet 5 as legacy: Sonnet 5.5 supersedes it at the same price. Migrate pins, re-running the effort sweep, and pin the `sonnet` alias to `claude-sonnet-5-5` in harness settings (Claude Code: `ANTHROPIC_DEFAULT_SONNET_MODEL`).
- Route bulk, mechanical, and high-volume work to Haiku 4.5 to hold spend down; it is 2x cheaper than Sonnet 5.5 and 10x cheaper than Fable 5.1 on output tokens.
- Routing policy: plan, plan-check, and review on Fable 5.1 (Opus 5.5 when price or retention says so); code day-to-day on Sonnet 5.5; run bulk and mechanical edits on Haiku 4.5.

## Sources

- https://platform.claude.com/docs/en/about-claude/models/overview
- https://platform.claude.com/docs/en/about-claude/models/choosing-a-model
- https://platform.claude.com/docs/en/models/opus-5-5/overview
- https://platform.claude.com/docs/en/models/sonnet-5-5/overview
- https://www.anthropic.com/news/claude-sonnet-5 (the August 2026 edit making $2 / $10 permanent)
- https://claude.com/blog/the-new-rules-of-context-engineering-for-claude-5-generation-models
- https://code.claude.com/docs/en/best-practices
- docs/roadmap/vendor-guidance-tracking.md (this repo, 2026-07-28 log entry on the Claude 5 context-engineering rules; 2026-09-16, 2026-09-22, and 2026-09-29 entries on these refreshes)
