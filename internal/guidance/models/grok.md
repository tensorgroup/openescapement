# Grok

xAI's current flagship text model is Grok 4.7, announced 2026-09-21 and positioned for coding, agentic tasks, and knowledge work at the same price as Grok 4.6. Grok Build 0.1 is the cheaper, faster coding model behind xAI's terminal agent of the same name, at half the flagship's input price and a third of its output price. Grok 4.6 and 4.5, the previous flagships, are still served but superseded; Grok 4.3 remains callable for compatibility only. xAI's own model guide says to use 4.7 for code, chat, and everything that is not audio, image, or video. xAI's docs and announcements now carry the SpaceXSI name; the API, model ids, and docs.x.ai URLs are unchanged. Refreshed 2026-10-24: Grok 4.7 added; Grok 4.6 moved to legacy.

## Models

### Grok 4.7 (grok-4.7) - frontier

xAI's flagship for coding, agentic tasks, and knowledge work, on a new, larger base model than 4.6 trained with a longer reinforcement-learning run weighted toward multi-hour tasks; xAI reports it works longer on hard tasks and checks its own work more carefully. Reasoning effort low, medium, high, or xhigh (default high). Text and image in, 500K-token context window, priced at $2 / $6 per million input/output tokens (cached input $0.50), with a higher rate ($4 / $12, cached $1) for every token once a request's prompt reaches 200K tokens. The Batch API is supported in us-east-1 and us-west-2 at a 20% discount. Grok 4.7 Fast, the same model at 2x the token rates, runs only in Cursor and Grok Build and bills through the plan there, not on the public API. On the Responses API it always returns `reasoning.encrypted_content`. Recommended roles: planning, plan-check, review, and coding; it is the recommended model for every role that needs judgment.

### Grok Build 0.1 (grok-build-0.1) - fast

xAI's coding model, the one Grok Build runs on, served under the aliases `grok-code-fast-1`, `grok-code-fast`, and `grok-code-fast-1-0825`. Text and image in, 256K-token context window, priced at $1 / $2 per million input/output tokens ($2 / $4 past 200K tokens of prompt). The Batch API is not supported. Recommended roles: coding and bulk. Use it for mechanical edits, code generation subagents, and high-volume agentic subtasks where 4.7's judgment is not needed.

### Grok 4.6 (grok-4.6) - legacy

The previous flagship, released 2026-08-12: 500K-token context, reasoning effort low through xhigh, priced the same as 4.7 ($2 / $6 per million tokens, $4 / $12 past 200K prompt tokens). Still served, and its Batch API support now matches 4.7's, but xAI's model guide names Grok 4.7 for every text role and OpenRouter lists 4.6 as succeeded by it. Migrate saved configuration to `grok-4.7`, which scores higher on xAI's coding benchmarks at the same price.

### Grok 4.5 (grok-4.5) - legacy

The flagship before 4.6: 500K-token context, configurable reasoning, priced the same as 4.7 ($2 / $6 per million tokens). Still served by xAI after Grok 4.6's release but no longer recommended for anything; migrate saved configuration to `grok-4.7`, which costs the same.

### Grok 4.3 (grok-4.3) - legacy

Two generations back, 1M-token context, $1.25 / $2.50 per million tokens, kept live for backward compatibility but receiving no further updates. Only where a pack or integration still pins this id; migrate to Grok 4.7, or to Grok Build 0.1 where the work was cost-driven.

## Governance

- Require human review before merging changes Grok 4.7 produced with high-autonomy tool calling (shell access, multi-step agentic runs in Grok Build); it is xAI's most capable model, so it also carries the largest share of autonomous-change risk.
- Route mechanical edits, code-generation subagents, and bulk work to Grok Build 0.1 to hold spend down; it is half of 4.7's price on input and a third on output. Within 4.7, use the reasoning_effort parameter to trade quality for latency on lower-stakes work, and expect the Fast variant in Cursor and Grok Build to draw plan usage at double the rate.
- Routing policy: plan, plan-check, and review on Grok 4.7; code day-to-day on Grok 4.7, dropping to Grok Build 0.1 for mechanical and bulk work; migrate any remaining Grok 4.6, 4.5, or 4.3 references off as soon as practical.

## Access routes

Prices as of 2026-10-24.

| Route | What you pay | Tool | Limits, per the vendor |
|---|---|---|---|
| Plan | The X Subscription: Plus $8/month, Premium $30 (4x usage), Super $100 (15x), Ultra $200 (50x) | Grok Build (`grok`) signs in through a browser on first launch, or takes an `XAI_API_KEY` | Each paid seat gets a weekly pool for eligible first-party usage across supported Grok and Cursor surfaces. xAI's pricing pages do not say whether Grok Build draws on that pool. |
| Direct API | Grok 4.7 $2 / $6 per million input/output tokens ($4 / $12 past 200K prompt tokens; Batch API 20% off); Grok Build 0.1 $1 / $2 | Grok Build with an API key, or any harness | Metered per token. |
| OpenRouter | `x-ai/grok-4.7` $2 / $6; `x-ai/grok-build-0.1` $1 / $2 | Any OpenAI-compatible harness | Provider prices can change without notice. |

- Treat Grok Build on an API key as the documented route, and check the live usage view before assuming a subscription covers agent work.
- Use a metered key for fast, small, occasional calls; Grok Build 0.1 at $2 per M output is cheap enough that a plan rarely matters for light use.

## Sources

- https://docs.x.ai/developers/models
- https://docs.x.ai/developers/models/grok-4.7 (pricing, reasoning effort, Batch API; retrieved 2026-10-24)
- https://x.ai/news/grok-4-7 (announced 2026-09-21)
- https://docs.x.ai/developers/release-notes (September entry: Grok 4.7; Grok 4.7 Fast only in Cursor and Grok Build; retrieved 2026-10-24)
- https://docs.x.ai/developers/models/grok-4.6
- https://x.ai/news/grok-4-6
- https://docs.x.ai/developers/models/grok-build-0.1
- https://docs.x.ai/developers/models/grok-4.5
- https://docs.x.ai/developers/models/grok-4.3
- https://docs.x.ai/developers/pricing
- https://docs.x.ai/build/overview
- https://docs.x.ai/x-subscription/pricing (retrieved 2026-10-24)
- https://docs.x.ai/x-subscription/usage-limits (retrieved 2026-10-24)
- https://openrouter.ai/api/v1/models (OpenRouter prices; retrieved 2026-10-24)
