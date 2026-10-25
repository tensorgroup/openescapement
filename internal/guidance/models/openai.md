# OpenAI

OpenAI's current lineup is the GPT-6 family: Astra at the frontier (launched 2026-09-03), GPT-6.1 Sol for everyday and complex coding (2026-09-29), and Luna for focused, high-volume work (2026-09-22). GPT-6 Sol (2026-09-22) stays available and is the fallback while GPT-6.1 Sol rolls out to Codex accounts; the GPT-5.6 family (Sol, Terra, Luna) is the fallback where the GPT-6 rollout has not landed; GPT-5.5 and GPT-5.2 are held over in this registry as legacy ids. They fit an agentic SDLC cleanly: plan and review on Astra, implement day-to-day on GPT-6.1 Sol, and run bulk work on GPT-6 Luna. Refreshed 2026-10-24: GPT-6.1 Sol added and preferred over GPT-6 Sol, which OpenAI's featured-models list and Codex's recommendation now name in its place; OpenAI has not deprecated GPT-6 Sol.

## Models

### GPT-6 Astra (gpt-6-astra) - frontier

OpenAI's flagship, built for the hardest end-to-end work, with reasoning effort from `low` up to `max` (no `none`; Codex adds an `ultra` mode that delegates to subagents). 1.05M-token context window, 128K max output, priced at $10 / $50 per million input/output tokens on the standard tier; batch and flex at $5 / $25, fast mode at $20 / $100, and requests over 272K input tokens at $20 / $75. It matches the top Anthropic tier on price, so treat it the same way: the first model for judgment, not for volume. Also available in Codex CLI under a ChatGPT sign-in, where a Pro plan is what heavy use needs and billing is to the plan rather than per token. Recommended roles: planning, plan-check, and review.

### GPT-6.1 Sol (gpt-6.1-sol) - mid

An upgrade to GPT-6 Sol, released 2026-09-29, that OpenAI describes as near-Astra performance for complex coding, computer use, and professional work at a lower cost. Codex recommends it for complex coding and agentic workflows where it is available. Reasoning effort `low` through `max`, default `medium`; unlike GPT-6 Sol it does not accept `none` or `minimal`. 1.05M-token context (922K max input), 128K max output, $2 / $10 per million tokens standard, the same as GPT-6 Sol, with cached input at $0.10 (half GPT-6 Sol's) and cache writes at $2.50. Prompts over 272K input tokens cost 2x input and 1.5x output; batch and flex run at half price, fast mode at double, and Ultrafast at 6x. Tool calling needs the Responses API; Chat Completions works without tools. Recommended roles: coding and review. This is the default day-to-day coding model, and it also covers review on lower-stakes diffs where a full Astra pass is not warranted.

Rollout caveat: GPT-6.1 Sol has its own Codex rollout, to Plus, Pro, Business, Enterprise, and Edu (Enterprise and Edu keep it off until an administrator enables it; Free and Go are not included). Pin GPT-6 Sol as the fallback until it reaches the account; the API serves it to direct API keys.

### GPT-6 Sol (gpt-6-sol) - mid

Built for complex coding and agentic workflows; OpenAI describes it as more factually reliable and clearer than GPT-5.6 Sol. GPT-6.1 Sol replaced it in OpenAI's featured models and in Codex's recommendation, at the same price; GPT-6 Sol is not deprecated and is the fallback where GPT-6.1 Sol has not rolled out. Reasoning effort `none` through `max`, default `medium`. 1.05M-token context, 128K max output, $2 / $10 per million tokens standard (cached input $0.20; over 272K input tokens at 2x input and 1.5x output; batch and flex at half, fast mode at double). It undercuts GPT-5.6 Terra on output and GPT-5.6 Sol on both sides. Recommended roles: coding and review, as the fallback on accounts that do not yet have GPT-6.1 Sol.

Rollout caveat: Codex under a ChatGPT sign-in serves it "when available" to the account. Until then, a request for it returns a 400 (`not supported when using Codex with a ChatGPT account`). Pin GPT-5.6 Terra as the fallback on those accounts; the API lists it for direct API keys.

### GPT-6 Luna (gpt-6-luna) - fast

OpenAI's most efficient model for focused, high-volume tasks: summarization, extraction, classification, transformation, and focused coding. Reasoning effort `none` through `max` (no `ultra`); Codex suggests starting at `high`. 1.05M-token context, 128K max output, $0.10 / $0.50 per million tokens standard, half GPT-5.6 Luna's input price and under half its output price. Recommended role: bulk. Enterprise and Edu workspaces need an administrator to enable it first. The same rollout caveat as Sol applies; GPT-5.6 Luna is the fallback.

### GPT-5.6 Sol (gpt-5.6-sol) - frontier

The previous frontier model, still a capable reviewer. 1.05M-token context, 128K max output; promotional pricing of $4 / $20 per million tokens is published through 2026-11-21, list $5 / $30. GPT-6.1 Sol is cheaper at $2 / $10, so new review work belongs there once it is available. Recommended role: review, where it already performs well.

### GPT-5.6 Terra (gpt-5.6-terra) - mid

OpenAI's balanced GPT-5.6 model. 1.05M-token context, 128K max output, $2 / $12 per million tokens. It has no GPT-6 counterpart by name; GPT-6.1 Sol takes over its coding role at a lower output price. Recommended roles: coding and review, as the fallback on accounts that have neither GPT-6 Sol model.

### GPT-5.6 Luna (gpt-5.6-luna) - fast

The cheapest GPT-5.6 model, built for cost-sensitive, high-volume workloads. 1.05M-token context, 128K max output, $0.20 / $1.20 per million tokens. Recommended role: bulk, as the fallback on accounts that do not yet have GPT-6 Luna.

### GPT-5.5 (gpt-5.5) - legacy

The flagship before the GPT-5.6 family. It retired from ChatGPT, ChatGPT Work, and Codex on all plans on 2026-10-14; the API keeps serving it, so a stale pin behind a direct API key keeps working. It stays in this registry so usage that still reports it is attributed; do not route new work to it.

### GPT-5.2 (gpt-5.2) - legacy

Previously OpenAI's flagship, superseded by GPT-5.3-Codex, GPT-5.4, GPT-5.5, the GPT-5.6 family, and now the GPT-6 family. Codex sessions signed in through ChatGPT already reject it; only Codex and the API authenticated with a direct API key still serve it, which is why a stale pin does not fail loudly. It stays in this registry because seeded usage telemetry references it; do not route new work to it.

## Governance

- Require a human review gate before merging any change GPT-6 Astra produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs); its reasoning depth is strong but not infallible, and the blast radius of an autonomous frontier session is larger than a supervised one.
- Know which route a session is on before running something long: Codex CLI under a ChatGPT sign-in bills to the plan, the same models through the API bill per token.
- Route bulk, mechanical, and high-volume work to GPT-6 Luna to hold spend down; it is 20x cheaper than GPT-6.1 Sol and 100x cheaper than Astra on output tokens.
- GPT-5.5 retired from ChatGPT, ChatGPT Work, and Codex on all plans on 2026-10-14 (the API keeps it). Migrate workspace defaults, saved settings, custom agents, scheduled tasks, and scripts to GPT-6.1 Sol (paid plans) or GPT-6 Luna (Free and Go), as Codex's migration note directs. GPT-5.4 and GPT-5.4-mini already left Codex under ChatGPT sign-in on 2026-08-31.
- Treat GPT-5.2 as retired for new work: migrate saved configurations, custom agents, and scheduled tasks to GPT-6.1 Sol, or to GPT-6 Luna for the cheapest tier (GPT-5.6 Terra or Luna where GPT-6 is not yet available).
- Routing policy: plan, plan-check, and review on GPT-6 Astra; code day-to-day on GPT-6.1 Sol; run bulk and mechanical edits on GPT-6 Luna. Where GPT-6.1 Sol has not reached an account, fall back to GPT-6 Sol; where the GPT-6 rollout has not reached it, fall back to GPT-5.6 Terra and Luna; GPT-5.6 Sol stays on review where it already reviews well.

## Access routes

Prices as of 2026-10-24.

| Route | What you pay | Tool | Limits, per the vendor |
|---|---|---|---|
| Plan + vendor CLI | ChatGPT Plus $20/month; Pro $100, $200, or $500/month | Codex CLI (`codex`) under a ChatGPT sign-in | Plus, per five hours: roughly 5-45 local messages on GPT-6 Astra, 15-160 on GPT-6.1 Sol, 15-150 on GPT-6 Sol, 350-3,000 on GPT-6 Luna; weekly limits may also apply. Pro plans currently have no five-hour limit. OpenAI calls these estimates, not fixed limits. |
| Direct API | GPT-6 Astra $10 / $50; GPT-6.1 Sol $2 / $10; GPT-6 Sol $2 / $10; GPT-6 Luna $0.10 / $0.50 per million input/output tokens | Codex CLI with an API key, or any harness | Metered per token. |
| OpenRouter | `openai/gpt-6-astra` $10 / $50; `openai/gpt-6.1-sol` $2 / $10; `openai/gpt-6-sol` $2 / $10; `openai/gpt-6-luna` $0.10 / $0.50 | Any OpenAI-compatible harness | Provider prices can change without notice. |

- When a plan pays for itself, counting output tokens only: Plus at $20 matches 2M GPT-6.1 Sol output tokens a month ($20 / $10 per M). Pro at $100 matches 2M GPT-6 Astra output tokens ($100 / $50 per M), Pro at $200 matches 4M, and Pro at $500 matches 10M. Input tokens push the real break-even earlier.
- Run repo-aware agent work through Codex CLI on a plan for daily use; Astra-heavy review is where the plan saves most, since Astra is the most expensive model per token.
- Use a metered key for fast, small, occasional calls, and for any job on an account the GPT-6 rollout has not reached (the API serves GPT-6.1 Sol, GPT-6 Sol, and Luna to direct keys).

## Sources

- https://platform.openai.com/docs/models
- https://platform.openai.com/docs/models/gpt-6-astra
- https://developers.openai.com/api/docs/models/gpt-6.1-sol (reasoning effort, context, pricing; retrieved 2026-10-24)
- https://platform.openai.com/docs/models/gpt-6-sol
- https://platform.openai.com/docs/models/gpt-6-luna
- https://platform.openai.com/docs/models/gpt-5.6-sol
- https://platform.openai.com/docs/models/gpt-5.6-terra
- https://platform.openai.com/docs/models/gpt-5.6-luna
- https://platform.openai.com/docs/models/gpt-5.5
- https://developers.openai.com/api/docs/changelog (2026-09-22 entry: GPT-6 Sol and Luna; 2026-09-29 entry: GPT-6.1 Sol)
- https://developers.openai.com/codex/models (recommended models, GPT-5.5 retirement, rollout; GPT-6.1 Sol rollout re-read 2026-10-24)
- https://openrouter.ai/openai/gpt-6-astra (launch pricing and tiers as listed 2026-09)
- docs/roadmap/vendor-guidance-tracking.md (this repo, 2026-09-16 and 2026-09-25 entries on these refreshes)
- https://developers.openai.com/codex/pricing (plans, per-five-hour estimates; retrieved 2026-10-24)
- https://openrouter.ai/api/v1/models (OpenRouter prices; retrieved 2026-10-24)
