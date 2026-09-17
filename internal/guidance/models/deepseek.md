# Deepseek

Deepseek's current API lineup is DeepSeek V4 Pro, the frontier reasoning model, and DeepSeek V4.1 Flash, the new-architecture general model released 2026-09-10 under the model name `deepseek-flash`. Both are OpenAI- and Anthropic-API compatible, share a 1M-token context window and a 384K-token max output, and support thinking and non-thinking modes on a single model id. Prices are peak / off-peak: off-peak (all hours outside 01:00 to 04:00 and 06:00 to 10:00 UTC on weekdays) is half of peak.

## Models

### DeepSeek V4 Pro (deepseek-v4-pro) - frontier

Deepseek's frontier reasoning model (served as DeepSeek-V4-Pro-0813): a 1.6T-parameter mixture-of-experts model (49B active) with a "Think Max" mode for hard math and agentic tasks. $1.32 / $3.96 per million input/output tokens at peak, $0.66 / $1.98 off-peak. Recommended roles: planning, plan-check, and review. Deepseek announced on 2026-09-10 that `deepseek-v4-pro` requests would route to V4.1 Flash from 2026-09-14 until a V4.1 Pro launches, then reversed that: the pricing page now says V4 Pro service continues with billing unchanged, with further notice promised before any change. Treat the id as live but watch the changelog.

### DeepSeek V4.1 Flash (deepseek-flash) - mid

The smallest model in Deepseek's new architecture family: a 552B-parameter mixture-of-experts model with a causal encoder-decoder design (8B active parameters for input, 16B for output), native vision, and a KV cache a quarter the size of the previous generation. Tests by multiple parties, per Deepseek's launch note, put it ahead of V4 Pro on performance, cost, and speed. $0.30 / $1.20 per million input/output tokens at peak, $0.15 / $0.60 off-peak (cache hits far cheaper). Recommended roles: coding and bulk.

### DeepSeek V4 Flash (deepseek-v4-flash) - legacy

Retired on 2026-09-10 together with `deepseek-v4-flash-vision-exp`. Both names are still accepted for compatibility and route to V4.1 Flash at the Flash price, for now. Migrate saved configuration to `deepseek-flash`.

## Governance

- Require human review before merging Pro-produced changes made with high autonomy; it is the more expensive model, and a bad frontier-tier decision has the largest blast radius.
- Route day-to-day coding and bulk work to V4.1 Flash to hold spend down; it is priced at under a quarter of Pro on input and under a third on output, and the launch benchmarks put it ahead of Pro on general work. Planning, plan-check, and review stay on Pro for its Think Max mode until a V4.1 Pro lands.
- Schedule flexible workloads off-peak; the rate is half of peak on every model.
- Routing policy: plan, plan-check, and review on Pro; day-to-day coding and bulk work on V4.1 Flash.
- Retired ids: `deepseek-chat` and `deepseek-reasoner` (2026-07-24), `deepseek-v4-flash` and `deepseek-v4-flash-vision-exp` (2026-09-10, still routed). Update any pack or config still referencing them to `deepseek-flash` or `deepseek-v4-pro` directly.

## Sources

- https://api-docs.deepseek.com/api/list-models/
- https://api-docs.deepseek.com/quick_start/pricing
- https://api-docs.deepseek.com/news/news260910/
- https://api-docs.deepseek.com/updates/
