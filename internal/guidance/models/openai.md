# OpenAI

OpenAI's current lineup is the GPT-5.6 family across three tiers, Sol at the frontier, Terra in the middle, and Luna for fast, high-volume work, with GPT-5.2 held over in this registry as a legacy id. The three current tiers fit an agentic SDLC cleanly: plan and review on Sol, implement day-to-day on Terra, and run bulk work on Luna.

## Models

### GPT-5.6 Sol (gpt-5.6-sol) - frontier

OpenAI's frontier model in the GPT-5.6 family (the `gpt-5.6` alias routes to it), built for complex professional work: strongest capability for complex coding, computer use, research, and cybersecurity, with reasoning effort selectable from none through max. 1.05M-token context window, 128K max output, priced at $5 / $30 per million input/output tokens. Recommended roles: planning, plan-check, and review. Use it to plan multi-file changes, plan-check another agent's plan, and review diffs for risk; reserve it for problems that need its full reasoning depth rather than routine coding.

### GPT-5.6 Terra (gpt-5.6-terra) - mid

OpenAI's balanced GPT-5.6 model: performance competitive with the prior GPT-5.5 flagship at well under half the cost. 1.05M-token context, 128K max output, $2 / $12 per million tokens. Recommended roles: coding and review. This is the default day-to-day coding model, and it also covers review on lower-stakes diffs where a full Sol pass is not warranted.

### GPT-5.6 Luna (gpt-5.6-luna) - fast

OpenAI's fastest and cheapest GPT-5.6 model, built for cost-sensitive, high-volume workloads; it roughly corresponds to the nano tier from earlier GPT-5 generations. 1.05M-token context, 128K max output, $0.20 / $1.20 per million tokens. Recommended role: bulk. Use it for classification, extraction, mechanical edits, and simple agentic subtasks run at scale.

### GPT-5.2 (gpt-5.2) - frontier

Previously OpenAI's flagship, GPT-5.2 has been superseded by GPT-5.3-Codex, GPT-5.4, GPT-5.5, and now the GPT-5.6 family. Codex sessions signed in through ChatGPT already reject it; only Codex and the API authenticated with a direct API key still serve it. It stays in this registry because seeded usage telemetry references it; do not route new work to it.

## Governance

- Require a human review gate before merging any change GPT-5.6 Sol produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs); its reasoning depth is strong but not infallible, and the blast radius of an autonomous frontier session is larger than a supervised one.
- Route bulk, mechanical, and high-volume work to GPT-5.6 Luna to hold spend down; it is 25x cheaper than Sol and 10x cheaper than Terra on output tokens.
- Treat GPT-5.2 as retired: it is already deprecated in Codex under ChatGPT sign-in, so migrate saved configurations, custom agents, and scheduled tasks to GPT-5.6 Terra, or to Luna for the cheapest tier.
- Routing policy: plan, plan-check, and review on GPT-5.6 Sol; code day-to-day on GPT-5.6 Terra; run bulk and mechanical edits on GPT-5.6 Luna.

## Sources

- https://platform.openai.com/docs/models
- https://platform.openai.com/docs/models/gpt-5.6-sol
- https://platform.openai.com/docs/models/gpt-5.6-terra
- https://platform.openai.com/docs/models/gpt-5.6-luna
- https://developers.openai.com/codex/models
