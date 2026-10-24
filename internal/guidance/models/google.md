# Google

Google's current lineup is led by Gemini 4 Argon, its flagship reasoning model (generally available since 2026-10-13), with Gemini 3.1 Pro Preview still served beside it, Gemini 3.8 Flash in the middle, and Gemini 3.5 Flash-Lite for fast, high-volume work. Gemini 3.7 Flash, Gemini 3.6 Flash, and the original Gemini 3 Pro Preview are held in this registry as legacy ids. The tiers fit an agentic SDLC cleanly: plan and review on Argon, implement day-to-day on 3.8 Flash, and run bulk work on Flash-Lite. Prices below are the Standard tier; Batch and Flex run at half. Refreshed 2026-10-24: Gemini 4 Argon added as the frontier choice; Gemini 3.7 Flash moved to legacy after Google deprecated it on 2026-10-08. Google has not named a replacement for 3.1 Pro Preview, so it stays current, behind Argon.

## Models

### Gemini 4 Argon (gemini-4-argon) - frontier

Google's flagship reasoning model, built for complex software engineering, deep mathematical and scientific problem solving, autonomous multi-step agents, and long decode. Text, image, video, audio, and PDF in; thinking at `low`, `medium` (the default), or `high`. A 1,048,576-token context window shared by input and output; 65,536 output tokens by default on a synchronous call, up to 1,048,576 with long decode. Priced at an introductory $2 / $10 per million input/output tokens through 2026-11-30, then $4 / $20 from 2026-12-01, with no long-prompt tier; Batch and Flex at half, Priority at $3.60 / $18 (then $7.20 / $36). Not on the Free tier. Only implicit caching is supported, and `temperature`, `top_p`, and `top_k` are ignored, so a migration removes explicit caching calls and sampling parameters and raises `max_output_tokens` (Google recommends at least 32,768). Recommended roles: planning, plan-check, and review. In the Antigravity CLI it is selected by an effort-suffixed id (`gemini-4-argon-low`, `-medium`, or `-high`, as listed by `agy models` on agy 1.4.7).

### Gemini 3.1 Pro Preview (gemini-3.1-pro-preview) - frontier

Google's previous Pro model, still served with no shutdown date announced, and built to refine the Gemini 3 Pro series with better thinking, improved token efficiency, and a more grounded, factually consistent output. It is optimized for software engineering behavior and for agentic workflows that need precise tool use and reliable multi-step execution across real-world domains. 1,048,576-token input context, 65,536-token max output, priced at $2 / $12 per million input/output tokens for prompts up to 200K tokens (rising to $4 / $18 above that). It still carries a preview label. Recommended roles: planning, plan-check, and review, only where a workload depends on explicit context caching, which Argon does not support; route new work to Argon.

### Gemini 3.8 Flash (gemini-3.8-flash) - mid

Google's most intelligent Flash model, generally available since 2026-09-02 and engineered for long-horizon software engineering, autonomous agents, and complex enterprise workflows at Flash speed and cost. Text, image, video, audio, and PDF in; thinking at low, medium, or high effort (`minimal` is rejected). 1,048,576-token input context, 65,536-token max output. Priced at $0.75 / $3.75 per million input/output tokens through 2026-12-31, then $1.50 / $7.50 from 2027-01-01. Recommended roles: coding and review. This is the default day-to-day coding model, and it also covers review on lower-stakes diffs where a full Argon pass is not warranted.

### Gemini 3.7 Flash (gemini-3.7-flash) - legacy

Released 2026-08-13, superseded by 3.8 Flash on 2026-09-02, and deprecated on 2026-10-08. The endpoint is not shutting down: API requests to `gemini-3.7-flash` are served by `gemini-3.8-flash`, and Google AI Studio no longer offers it. Migrate pins to `gemini-3.8-flash`, so the configured id names the model that does the work.

### Gemini 3.5 Flash-Lite (gemini-3.5-flash-lite) - fast

Google's fastest and most cost-effective current model, optimized for high-throughput, low-cost work: sub-agent tasks, document parsing, and simple data extraction where latency and API cost are the binding constraints. Same 1,048,576-token input context and 65,536-token max output as the larger Gemini 3 models, priced at $0.30 / $2.50 per million tokens with no introductory period. Recommended role: bulk. Use it for classification, extraction, mechanical edits, and high-volume agentic subtasks run at scale.

### Gemini 3.6 Flash (gemini-3.6-flash) - legacy

The previous Flash generation, still served by Google on the same price schedule as 3.7 and 3.8 Flash ($0.75 / $3.75 per million tokens through 2026-12-31, $1.50 / $7.50 after). Both successors are more capable at the same price, so migrate any saved configuration to `gemini-3.8-flash`.

### Gemini 3 Pro Preview (gemini-3-pro) - legacy

Previously Google's frontier reasoning model, Gemini 3 Pro Preview (`gemini-3-pro-preview`) launched November 2025 and was shut down on 2026-03-09; its listed replacement was `gemini-3.1-pro-preview`. It stays in this registry because seeded usage telemetry references it under the shorter id `gemini-3-pro`; do not route new work to it, and migrate any saved configuration to `gemini-4-argon`.

## Governance

- Require a human review gate before merging any change Gemini 4 Argon produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs); its reasoning depth is strong but not infallible, and the blast radius of an autonomous frontier session is larger than a supervised one.
- Route plan, plan-check, and review to Gemini 4 Argon, with the thinking level set explicitly. Keep Gemini 3.1 Pro Preview only for workloads that depend on explicit context caching.
- Budget for Argon's price doubling on 2026-12-01 ($4 / $20 per million tokens from then); its Batch tier at half price suits reviews that can wait.
- Route bulk, mechanical, and high-volume work to Gemini 3.5 Flash-Lite to hold spend down. Through 2026-12-31 it is 2.5x cheaper than 3.8 Flash on input and 1.5x cheaper on output ($2.50 vs $3.75 per million); from 2027-01-01, when Flash's introductory price ends, the gap widens to 5x and 3x. Against Argon it is 4x cheaper on output through 2026-11-30 and 8x after.
- Treat Gemini 3.7 Flash and 3.6 Flash as legacy: 3.8 Flash is more capable at the same price, and 3.7 Flash requests are already served by 3.8 Flash.
- Treat Gemini 3 Pro Preview (`gemini-3-pro`) as retired: Google shut it down on 2026-03-09, so point new configuration at `gemini-4-argon` directly.
- Routing policy: plan, plan-check, and review on Gemini 4 Argon; code day-to-day on Gemini 3.8 Flash; run bulk and mechanical edits on Gemini 3.5 Flash-Lite.

## Access routes

Prices as of 2026-10-24.

| Route | What you pay | Tool | Limits, per the vendor |
|---|---|---|---|
| Plan + vendor CLI | Google AI Pro $20/month; Google AI Ultra $100/month (5x Pro's tokens) or $200/month (20x) | Antigravity CLI (`agy`), included on every plan | Pro: a quota refreshed every five hours until a weekly limit. Ultra: the highest quota, refreshed every five hours. No numbers are published. Gemini models share one pool drawn down at API prices; non-Gemini models have a separate fixed limit. Overage runs on purchased AI credits only if the "AI Credit Overages" setting is `Always`. |
| Direct API | Gemini 4 Argon $2 / $10 per million input/output tokens through 2026-11-30, then $4 / $20 (Batch and Flex half); 3.8 Flash $0.75 / $3.75; 3.5 Flash-Lite $0.30 / $2.50 | Any harness with a Gemini API key | Metered per token. Argon is not on the Free tier. |
| OpenRouter | `google/gemini-4-argon` $2 / $10; `google/gemini-4-argon:batch` $1 / $5; `google/gemini-3.8-flash` $0.825 / $4.125 | Any OpenAI-compatible harness | Provider prices can change without notice. |

- agy also serves Claude Opus 5.5 and Claude Sonnet 5.5 (thinking) on Google AI Pro (paid subscriptions, not trials) and Ultra. That makes the Google plan a second route to Claude-family models that does not draw on an Anthropic plan.
- Gemini 4 Argon is on paid Pro and Ultra in agy, not on Free or Plus. Gemini 3.1 Pro is offered in agy only on Enterprise.
- When a plan pays for itself, counting output tokens only: Pro at $20 matches 2M Argon output tokens a month at the introductory $10 per M, and 1M from 2026-12-01 at $20 per M. Ultra at $100 matches 10M, then 5M. Ultra at $200 matches 20M, then 10M. Input tokens push the real break-even earlier.
- Run repo-aware agent work through agy on a plan for daily use. Use a metered key for fast, small, occasional calls; the Batch tier halves the price for work that can wait.

## Sources

- https://ai.google.dev/gemini-api/docs/models/gemini-4-argon (retrieved 2026-10-24)
- https://ai.google.dev/gemini-api/docs/latest-model (Argon API changes and migration; retrieved 2026-10-24)
- https://ai.google.dev/gemini-api/docs/models
- https://ai.google.dev/gemini-api/docs/models/gemini-3.1-pro-preview
- https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash
- https://ai.google.dev/gemini-api/docs/models/gemini-3.7-flash
- https://ai.google.dev/gemini-api/docs/models/gemini-3.6-flash
- https://ai.google.dev/gemini-api/docs/models/gemini-3.5-flash-lite
- https://ai.google.dev/gemini-api/docs/deprecations
- https://ai.google.dev/gemini-api/docs/pricing
- https://ai.google.dev/gemini-api/docs/changelog
- https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/model-routing.md
- https://antigravity.google/blog/changes-to-antigravity-plans/ (plan prices and multipliers, 2026-05-19; retrieved 2026-10-24)
- https://antigravity.google/docs/plans (quota and overage rules; retrieved 2026-10-24)
- https://antigravity.google/docs/models (models per plan in agy; retrieved 2026-10-24)
- https://openrouter.ai/google/gemini-4-argon and https://openrouter.ai/google/gemini-4-argon:batch (retrieved 2026-10-24)
- https://openrouter.ai/api/v1/models (OpenRouter prices; retrieved 2026-10-24)
