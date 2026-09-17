# Grok

xAI's current flagship text model is Grok 4.6, released 2026-08-12 and positioned for code and everything else with configurable reasoning effort. Grok Build 0.1 is the cheaper, faster coding model behind xAI's terminal agent of the same name, at half the flagship's input price and a third of its output price. Grok 4.5, the previous flagship, is still served but superseded; Grok 4.3 remains callable for compatibility only. xAI's own model guide says to use 4.6 for code, chat, and everything that is not audio, image, or video.

## Models

### Grok 4.6 (grok-4.6) - frontier

xAI's flagship model for code and everything else: agentic tool calling, minimal hallucinations, and reasoning effort configurable at low, medium, high, or xhigh (default high). Text and image in, 500K-token context window, priced at $2 / $6 per million input/output tokens, with a higher rate ($4 / $12) once a request's prompt reaches 200K tokens and a fast variant at double the price. The Batch API is not supported. Recommended roles: planning, plan-check, review, and coding; it is the recommended model for every role that needs judgment.

### Grok Build 0.1 (grok-build-0.1) - fast

xAI's coding model, the one Grok Build runs on, served under the aliases `grok-code-fast-1`, `grok-code-fast`, and `grok-code-fast-1-0825`. Text and image in, 256K-token context window, priced at $1 / $2 per million input/output tokens ($2 / $4 past 200K tokens of prompt). The Batch API is not supported. Recommended roles: coding and bulk. Use it for mechanical edits, code generation subagents, and high-volume agentic subtasks where 4.6's judgment is not needed.

### Grok 4.5 (grok-4.5) - legacy

The previous flagship: 500K-token context, configurable reasoning, priced the same as 4.6 ($2 / $6 per million tokens). Still served by xAI after Grok 4.6's release but no longer recommended for anything; migrate saved configuration to `grok-4.6`, which scores higher on xAI's agentic coding benchmarks at the same price.

### Grok 4.3 (grok-4.3) - legacy

Two generations back, 1M-token context, $1.25 / $2.50 per million tokens, kept live for backward compatibility but receiving no further updates. Only where a pack or integration still pins this id; migrate to Grok 4.6, or to Grok Build 0.1 where the work was cost-driven.

## Governance

- Require human review before merging changes Grok 4.6 produced with high-autonomy tool calling (shell access, multi-step agentic runs in Grok Build); it is xAI's most capable model, so it also carries the largest share of autonomous-change risk.
- Route mechanical edits, code-generation subagents, and bulk work to Grok Build 0.1 to hold spend down; it is half of 4.6's price on input and a third on output. Within 4.6, use the reasoning_effort parameter to trade quality for latency on lower-stakes work, and expect the fast variant to cost double.
- Routing policy: plan, plan-check, and review on Grok 4.6; code day-to-day on Grok 4.6, dropping to Grok Build 0.1 for mechanical and bulk work; migrate any remaining Grok 4.5 or 4.3 references off as soon as practical.

## Sources

- https://docs.x.ai/developers/models
- https://docs.x.ai/developers/models/grok-4.6
- https://x.ai/news/grok-4-6
- https://docs.x.ai/developers/models/grok-build-0.1
- https://docs.x.ai/developers/models/grok-4.5
- https://docs.x.ai/developers/models/grok-4.3
- https://docs.x.ai/developers/pricing
- https://docs.x.ai/build/overview
