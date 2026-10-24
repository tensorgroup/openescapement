## Choosing a DeepSeek model

DeepSeek V4.1 Flash is cheap enough to run on every triage pass and every second opinion. Name it by the id of the route you call it through.

| Model | Route | ID | $/1M tokens (in/out), 2026-10-24 | Use for |
|---|---|---|---|---|
| V4.1 Flash | DeepSeek API | `deepseek-flash` | $0.30 / $1.20 peak; $0.15 / $0.60 off-peak | Second opinions, triage, coding, bulk. |
| V4.1 Flash | OpenRouter | `deepseek/deepseek-v4.1-flash` | $0.02 / $1.00, a provider price that can end without notice | The same, input-heavy work especially. |
| V4.1 Flash, dated | OpenRouter | `deepseek/deepseek-v4.1-flash-1017` | $0.15 / $0.60 | Configs that must not move when the alias advances. |
| V4 Pro | DeepSeek API | `deepseek-v4-pro` | $1.32 / $3.96 peak; $0.66 / $1.98 off-peak | Planning and review. |

Rules of thumb:

- Use `deepseek-flash` on DeepSeek's own API and `deepseek/deepseek-v4.1-flash` on OpenRouter. The two routes name the same model differently, so a config copied between them names the wrong id.
- Set a spend cap on the OpenRouter key. A cheap model in an agent loop can still run all night.
- Verify V4.1 Flash's findings like anyone else's. A cheap model earns its place on confirmed-finding rate, not on volume.
- Schedule flexible work off-peak on DeepSeek's API, where every rate is half of peak.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
