## Choosing a DeepSeek model

DeepSeek V4.1 Flash is cheap enough to run on every triage pass and every second opinion. Name it by the id of the route you call it through.

| Model | Route | ID | $/1M tokens (in/out), 2026-10-24 | Use for |
|---|---|---|---|---|
| V4.1 Flash | DeepSeek API | `deepseek-flash` | $0.30 / $1.20 peak; $0.15 / $0.60 off-peak | Second opinions, triage, coding, bulk. |
| V4.1 Flash, 2026-09-10 release | OpenRouter | `deepseek/deepseek-v4.1-flash` | $0.02 / $1.00 at all hours, a provider price that can end without notice | Input-heavy second opinions and triage. |
| V4.1 Flash, 2026-10-17 release | OpenRouter | `deepseek/deepseek-v4.1-flash-1017` | $0.30 / $1.20 weekday peak; $0.15 / $0.60 otherwise | Results that must match DeepSeek's own API. |
| V4 Pro | DeepSeek API | `deepseek-v4-pro` | $1.32 / $3.96 peak; $0.66 / $1.98 off-peak | Planning and review. |

Rules of thumb:

- Use `deepseek-flash` on DeepSeek's own API. It serves the 2026-10-17 release.
- On OpenRouter, use `deepseek/deepseek-v4.1-flash-1017` for that same release. The alias `deepseek/deepseek-v4.1-flash` is the older 2026-09-10 release, so a config that swaps one id for the other changes the model.
- Set a spend cap on the OpenRouter key. A cheap model in an agent loop can still run all night.
- Verify V4.1 Flash's findings like anyone else's. A cheap model earns its place on confirmed-finding rate, not on volume.
- Schedule flexible work off-peak on DeepSeek's API and on `-1017`, where every rate is half of peak. Peak is 01:00 to 04:00 and 06:00 to 10:00 UTC, Monday to Friday.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
