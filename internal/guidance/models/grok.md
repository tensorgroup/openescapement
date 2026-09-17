# Grok

xAI's current flagship text model is Grok 4.6, released 2026-08-12 and positioned for code and everything else with configurable reasoning effort. Grok 4.5, the previous flagship, is still served but superseded; Grok 4.3 remains callable for compatibility only. xAI's own model guide says to use 4.6 for code, chat, and everything that is not audio, image, or video.

## Models

### Grok 4.6 (grok-4.6) - frontier

xAI's flagship model for code and everything else: agentic tool calling, minimal hallucinations, and reasoning effort configurable at low, medium, high, or xhigh (default high). Text and image in, 500K-token context window, priced at $2 / $6 per million input/output tokens, with a higher rate for requests past 200K tokens of context and a fast variant at double the price. The Batch API is not supported. Recommended roles: planning, plan-check, review, and coding; it is the only actively recommended text model, so route all of these through it.

### Grok 4.5 (grok-4.5) - legacy

The previous flagship: 500K-token context, configurable reasoning. Still served by xAI after Grok 4.6's release but no longer recommended for anything; migrate saved configuration to `grok-4.6`, which costs the same and scores higher on xAI's agentic coding benchmarks.

### Grok 4.3 (grok-4.3) - legacy

Two generations back, 1M-token context, kept live for backward compatibility but receiving no further updates. Only where a pack or integration still pins this id; migrate to Grok 4.6.

## Governance

- Require human review before merging changes Grok 4.6 produced with high-autonomy tool calling (shell access, multi-step agentic runs in Grok Build); it is xAI's most capable model and the only one currently recommended, so it also carries the largest share of autonomous-change risk.
- xAI does not publish a separate low-cost or fast tier. Use the reasoning_effort parameter to trade quality for latency and cost on lower-stakes work instead of switching models, and expect the fast variant to cost double.
- Routing policy: plan, plan-check, review, and code on Grok 4.6; migrate any remaining Grok 4.5 or 4.3 references off as soon as practical.

## Sources

- https://docs.x.ai/developers/models
- https://docs.x.ai/developers/models/grok-4.6
- https://x.ai/news/grok-4-6
- https://docs.x.ai/developers/models/grok-4.5
- https://docs.x.ai/developers/models/grok-4.3
- https://docs.x.ai/developers/pricing
- https://docs.x.ai/build/overview
