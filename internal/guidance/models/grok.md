# Grok

xAI has consolidated its API onto a single current flagship text model, Grok 4.5, positioned for both chat and code with configurable reasoning effort. The previous generation, Grok 4.3, remains callable for compatibility but is no longer the recommended default. Grok Build, xAI's terminal coding agent, runs on Grok 4.5 by default (alias `grok-build-latest`).

## Models

### Grok 4.5 (grok-4.5) - frontier

xAI's flagship model for code and everything else: agentic tool calling, minimal hallucinations, configurable reasoning, and a 500K-token context window. Recommended roles: planning, plan-check, review, and coding; it is currently the only actively recommended text model, so route all of these through it.

### Grok 4.3 (grok-4.3) - legacy

The prior flagship, 1M-token context, kept live for backward compatibility after Grok 4.5's release but receiving no further updates. Recommended roles: planning and coding, only where a pack or integration still pins this id; migrate new work to Grok 4.5.

## Governance

- Require human review before merging changes Grok 4.5 produced with high-autonomy tool calling (shell access, multi-step agentic runs in Grok Build); it is xAI's most capable model and the only one currently recommended, so it also carries the largest share of autonomous-change risk.
- xAI does not currently publish a separate low-cost or fast tier. Use the reasoning_effort parameter to trade quality for latency and cost on lower-stakes work instead of switching models.
- Routing policy: plan, plan-check, review, and code on Grok 4.5; migrate any remaining Grok 4.3 references off as soon as practical.

## Sources

- https://docs.x.ai/developers/models
- https://docs.x.ai/developers/models/grok-4.5
- https://docs.x.ai/developers/models/grok-4.3
- https://docs.x.ai/developers/pricing
- https://docs.x.ai/build/overview
