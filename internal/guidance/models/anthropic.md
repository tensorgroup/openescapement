# Anthropic

Anthropic's current lineup is four Claude 5-generation models across three tiers: Claude Opus 5 and Claude Fable 5 at the frontier, Claude Sonnet 5 in the middle, and Claude Haiku 4.5 for fast, high-volume work. All four fit an agentic SDLC cleanly: plan and review on the frontier tier, implement day-to-day on Sonnet, and run bulk or mechanical work on Haiku.

## Models

### Claude Opus 5 (claude-opus-5) - frontier

Anthropic's model for complex agentic coding and enterprise work: deep reasoning, long-horizon agentic tasks, and test-time compute scaling via the effort parameter (defaults to `high`; step up to `xhigh` for the hardest coding and agentic work). 1M-token context window, 128K max output, priced at $5 / $25 per million input/output tokens. Recommended roles: planning, plan-check, and review. Use it to plan multi-file changes, plan-check another agent's plan, and review diffs for risk; it is the default starting point for frontier-tier work, so reserve Claude Fable 5 for the problems Opus 5 cannot close.

### Claude Fable 5 (claude-fable-5) - frontier

Anthropic's most capable widely released model, built for long-running agents and its hardest reasoning workloads. Always-on adaptive thinking, 1M-token context, 128K max output, but priced at $10 / $50 per million tokens, double Opus 5, and slower in practice. Recommended roles: planning, plan-check, and review, but only when Opus 5 genuinely stalls; it is not the default frontier choice for routine planning or review.

### Claude Sonnet 5 (claude-sonnet-5) - mid

Anthropic's balance of speed and intelligence: near-frontier coding and agentic quality at a fraction of Opus 5's cost. 1M-token context, 128K max output, $3 / $15 per million tokens ($2 / $10 introductory pricing through 2026-08-31). Recommended roles: coding and review. This is the default day-to-day coding model, and it also covers review on lower-stakes diffs where a full frontier pass is not warranted.

### Claude Haiku 4.5 (claude-haiku-4-5) - fast

Anthropic's fastest model with near-frontier intelligence, built for high-volume, repetitive work: classification, extraction, mechanical edits, and simple agentic subtasks run thousands of times over. 200K-token context (smaller than the other three), 64K max output, $1 / $5 per million tokens. Recommended role: bulk. It is the only current model without adaptive thinking, so keep it off tasks that need multi-step reasoning.

## Governance

- Require a human review gate before merging any change Opus 5 or Fable 5 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs). Frontier-tier judgment is strong but not infallible, and the blast radius of an autonomous frontier session is larger than a supervised one.
- Reserve Claude Fable 5 for tasks Opus 5 cannot close. It costs double Opus 5 per token and runs slower, so routing routine frontier work to it burns budget with no quality gain.
- Route bulk, mechanical, and high-volume work to Haiku 4.5 to hold spend down; it is roughly 5x cheaper than Sonnet 5 and 25x cheaper than Fable 5 on output tokens.
- Routing policy: plan, plan-check, and review on Opus 5, escalating to Fable 5 only for the hardest problems; code day-to-day on Sonnet 5; run bulk and mechanical edits on Haiku 4.5.

## Sources

- https://platform.claude.com/docs/en/about-claude/models/overview
- https://platform.claude.com/docs/en/about-claude/models/choosing-a-model
- https://claude.com/blog/the-new-rules-of-context-engineering-for-claude-5-generation-models
- https://code.claude.com/docs/en/best-practices
- docs/roadmap/vendor-guidance-tracking.md (this repo, 2026-07-28 log entry on the Claude 5 context-engineering rules)
