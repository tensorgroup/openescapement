# OpenAI

OpenAI's current lineup is GPT-6 Astra at the frontier, launched 2026-09-03, above the GPT-5.6 family: Sol, the previous frontier model, Terra in the middle, and Luna for fast, high-volume work, with GPT-5.2 held over in this registry as a legacy id. They fit an agentic SDLC cleanly: plan and review on Astra, implement day-to-day on Terra, and run bulk work on Luna. Refreshed 2026-09-16: Astra added, Sol's role narrowed to review.

## Models

### GPT-6 Astra (gpt-6-astra) - frontier

OpenAI's flagship, built for complex professional work with reasoning effort selectable up to max. 1.05M-token context window, 128K max output, priced at $10 / $50 per million input/output tokens on the standard tier; batch and flex at $5 / $25, fast mode at $20 / $100, and requests over 272K input tokens at $20 / $75. It matches the top Anthropic tier on price, so treat it the same way: the first model for judgment, not for volume. Also available in Codex CLI under a ChatGPT sign-in, where a Pro plan is what heavy use needs and billing is to the plan rather than per token. Recommended roles: planning, plan-check, and review.

### GPT-5.6 Sol (gpt-5.6-sol) - frontier

The previous frontier model, still a capable reviewer. 1.05M-token context, 128K max output; promotional pricing of $4 / $20 per million tokens is published through 2026-11-21, list $5 / $30. Recommended role: review, where it already performs well. Planning and design work moves to Astra as sessions roll over.

### GPT-5.6 Terra (gpt-5.6-terra) - mid

OpenAI's balanced GPT-5.6 model: performance competitive with the prior GPT-5.5 flagship at well under half the cost. 1.05M-token context, 128K max output, $2 / $12 per million tokens. Recommended roles: coding and review. This is the default day-to-day coding model, and it also covers review on lower-stakes diffs where a full frontier pass is not warranted.

### GPT-5.6 Luna (gpt-5.6-luna) - fast

OpenAI's fastest and cheapest GPT-5.6 model, built for cost-sensitive, high-volume workloads; it roughly corresponds to the nano tier from earlier GPT-5 generations. 1.05M-token context, 128K max output, $0.20 / $1.20 per million tokens. Recommended role: bulk. Use it for classification, extraction, mechanical edits, and simple agentic subtasks run at scale.

### GPT-5.2 (gpt-5.2) - legacy

Previously OpenAI's flagship, superseded by GPT-5.3-Codex, GPT-5.4, GPT-5.5, the GPT-5.6 family, and now GPT-6 Astra. Codex sessions signed in through ChatGPT already reject it; only Codex and the API authenticated with a direct API key still serve it, which is why a stale pin does not fail loudly. It stays in this registry because seeded usage telemetry references it; do not route new work to it.

## Governance

- Require a human review gate before merging any change GPT-6 Astra produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs); its reasoning depth is strong but not infallible, and the blast radius of an autonomous frontier session is larger than a supervised one.
- Know which route a session is on before running something long: Codex CLI under a ChatGPT sign-in bills to the plan, the same models through the API bill per token.
- Route bulk, mechanical, and high-volume work to GPT-5.6 Luna to hold spend down; it is 10x cheaper than Terra and roughly 40x cheaper than Astra on output tokens.
- Treat GPT-5.2 as retired for new work: migrate saved configurations, custom agents, and scheduled tasks to GPT-5.6 Terra, or to Luna for the cheapest tier.
- Routing policy: plan, plan-check, and review on GPT-6 Astra; review also on GPT-5.6 Sol where it already reviews well; code day-to-day on GPT-5.6 Terra; run bulk and mechanical edits on GPT-5.6 Luna.

## Sources

- https://platform.openai.com/docs/models
- https://platform.openai.com/docs/models/gpt-6-astra
- https://platform.openai.com/docs/models/gpt-5.6-sol
- https://platform.openai.com/docs/models/gpt-5.6-terra
- https://platform.openai.com/docs/models/gpt-5.6-luna
- https://developers.openai.com/codex/models
- https://openrouter.ai/openai/gpt-6-astra (launch pricing and tiers as listed 2026-09)
- docs/roadmap/vendor-guidance-tracking.md (this repo, 2026-09-16 entry on this refresh)
