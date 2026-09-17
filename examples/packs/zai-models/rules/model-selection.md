## Choosing a Z.ai GLM model

GLM 5.3 Flash is the near-free model: cheap enough to run on every review and every bulk job, capable enough that its findings are worth verifying. Reach it through OpenRouter so one key and one spend cap cover it.

| Model | ID (OpenRouter) | $/1M tokens (in/out) | Use for |
|---|---|---|---|
| GLM 5.3 Flash | `z-ai/glm-5.3-flash` | $0.15 / $0.50 list at Z.ai; OpenRouter providers post $0.075 to $0.15 in and $0.25 to $0.50 out (2026-09-16), the cheapest a per-provider discount that can end without notice | Second opinions, code review, long-context reads, bulk work. 1.31M context on paper; 262K to 1.31M by provider. MIT weights. |
| GLM 5.3 Flash, dated | `z-ai/glm-5.3-flash-20260826` | same | The same model behind a stable id, for configs that must not move. |
| GLM 5.3 | `z-ai/glm-5.3` | $1.40 / $4.40 list at Z.ai; OpenRouter providers posted $0.8775 / $2.97 in 2026-09 | The full-size sibling when Flash's answers run thin. |
| stealth/ox-alpha | retired | none | The preview id Flash was served under before release. Retired 2026-09-02; move any config to the released id. |

Rules of thumb:

- Put Flash on everything that benefits from a second reader: reviews, plan checks, doc passes. At list price a repo-aware review costs cents, so the cost of running it is never the reason to skip it.
- Verify its findings like anyone else's. A cheap model earns its place on confirmed-finding rate, not on volume.
- Pin a provider through OpenRouter's provider routing when you need the full context window or a specific data policy; the effective window and the terms both vary by provider, and private code goes wherever the request is routed.
- Set a spend cap on the OpenRouter key. Near-free is not free, and an agent loop can run all night.
- Never name a preview id in a wrapper, a schema, or an interface. Pin it in one line of config. The stealth id this model launched under lasted weeks. This pack names it once, in the catalog, as banned: a policy may name what it forbids so a stale config is caught by its own text.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
