# Deepseek

Deepseek's current API lineup is two V4-generation models: DeepSeek V4 Pro, the frontier reasoning model, and DeepSeek V4 Flash, the faster and cheaper general model. Both are OpenAI- and Anthropic-API compatible, share a 1M-token context window, and support thinking and non-thinking modes on a single model id.

## Models

### DeepSeek V4 Pro (deepseek-v4-pro) - frontier

Deepseek's frontier reasoning model: a 1.6T-parameter mixture-of-experts model (49B active) with a "Think Max" mode for hard math and agentic tasks, and the strongest coding benchmark results in the lineup. Recommended roles: planning, plan-check, and review.

### DeepSeek V4 Flash (deepseek-v4-flash) - mid

The faster, cost-efficient sibling: a 284B-parameter mixture-of-experts model (13B active), recommended by Deepseek for chat, coding, summaries, and high-volume pipelines at roughly a third of Pro's per-token price. Recommended roles: coding and bulk.

## Governance

- Require human review before merging Pro-produced changes made with high autonomy; it is the more expensive and more capable model, and a bad frontier-tier decision has the largest blast radius.
- Route bulk and high-volume pipeline work to Flash to hold spend down; it is priced at roughly a third of Pro on both input and output tokens.
- Routing policy: plan, plan-check, and review on Pro; day-to-day coding and bulk work on Flash.
- The legacy deepseek-chat and deepseek-reasoner aliases were retired on 2026-07-24. Update any pack or config still referencing them to deepseek-v4-flash or deepseek-v4-pro directly.

## Sources

- https://api-docs.deepseek.com/api/list-models/
- https://api-docs.deepseek.com/quick_start/pricing
- https://api-docs.deepseek.com/updates/
