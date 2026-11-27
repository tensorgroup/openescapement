# Google

Google's current lineup is led by three Gemini 4 models. Gemini 4 Neon (generally available 2026-11-18) is the default frontier pick for agentic coding, planning, and review. Gemini 4 Carbon (2026-11-10) is Google's strongest reasoning model, for the hardest deep work. Gemini 4 Hydrogen (2026-11-12) is the fastest and cheapest Gemini 4 model, for everyday coding. Gemini 4 Argon (2026-10-13) is still served, and Gemini 3.8 Flash and Gemini 3.5 Flash-Lite cover cost-bound and bulk work. Gemini 3.1 Pro Preview, Gemini 3.7 Flash, Gemini 3.6 Flash, and the original Gemini 3 Pro Preview are held in this registry as legacy ids. The tiers fit an agentic SDLC cleanly: plan and review on Neon, escalate the hardest problems to Carbon, implement day-to-day on Hydrogen, and run bulk work on Flash-Lite. Prices below are the Standard tier; Batch and Flex run at half. Refreshed 2026-11-20: Neon, Carbon, and Hydrogen added; Neon replaces Argon as the recommended frontier pick; Gemini 3.1 Pro Preview moved to legacy because Google now names Neon as its upgrade. The 2026-10-24 refresh added Argon and moved Gemini 3.7 Flash to legacy after Google deprecated it on 2026-10-08.

## Models

### Gemini 4 Neon (gemini-4-neon) - frontier

Google's model for complex software engineering, knowledge work, and science workflows with fast end-to-end task completion. Google places it between Hydrogen and Carbon: it reasons more deeply than Hydrogen, and it finishes faster, with fewer output tokens, than Carbon. Text, image, video, audio, and PDF in; thinking at `low`, `medium`, `high` (the default), or `xhigh`. A 1,048,576-token context window shared by input and output; 65,536 output tokens by default on a synchronous call, up to 1,048,576 with long decode. Priced at $3 / $15 per million input/output tokens with no introductory period; Batch and Flex at $1.50 / $7.50, Priority at $5.40 / $27. Not on the Free tier. Only implicit caching is supported. Recommended roles: planning, plan-check, review, and complex agentic coding. It is the default model for new conversations in Antigravity. In the Antigravity CLI it is selected by an effort-suffixed id (`gemini-4-neon-low`, `-medium`, `-high`, or `-xhigh`, as listed by `agy models` on agy 1.4.7), and its context window there is up to 512K tokens.

### Gemini 4 Carbon (gemini-4-carbon) - frontier

Google's strongest reasoning model, built for sustained agentic coding, multimodal knowledge work, scientific reasoning, and cybersecurity defense. It spends more thinking tokens per turn than Neon to test hypotheses, isolate root causes across repositories, and check fixes against tests. Thinking at `low`, `medium` (the default), or `high`. The same 1,048,576-token unified context and long decode as Neon. Priced at an introductory $2 / $10 per million input/output tokens through 2026-11-30, then $4 / $20 from 2026-12-01; Batch and Flex at half. Not on the Free tier. Only implicit caching is supported. Recommended roles: planning, plan-check, and review, held for the hardest problems: system architecture, multi-file debugging, dense multimodal analysis, formal proofs, security audits, or a task where a faster model gets stuck. In the Antigravity CLI it is selected as `gemini-4-carbon-low`, `-medium`, or `-high`, on Google AI Ultra and Enterprise only.

### Gemini 4 Hydrogen (gemini-4-hydrogen) - mid

Google's fast, efficient Gemini 4 model for everyday coding and agentic workflows. Google says it cuts fabricated claims by more than 5x compared with Gemini 3.8 Flash, and it batches read and search tool calls in parallel. Thinking at `low`, `medium`, `high` (the default), or `xhigh`. The same 1,048,576-token unified context and long decode as Neon. Priced at $1.50 / $7.50 per million input/output tokens; Batch and Flex at $0.75 / $3.75, Priority at $2.70 / $13.50. It is the only Gemini 4 model on the Free tier. Only implicit caching is supported. Recommended roles: coding and lower-stakes review. Google names it the upgrade for Gemini 3.8 Flash and 3.6 Flash. In the Antigravity CLI it is selected as `gemini-4-hydrogen-low`, `-medium`, `-high`, or `-xhigh`, on paid Google AI Pro and on Ultra.

### Gemini 4 Argon (gemini-4-argon) - frontier

Google's first Gemini 4 model, generally available since 2026-10-13 and still served with no shutdown date. Google describes it as a more efficient model than Carbon, with faster responses and lower token spend per turn. Thinking at `low`, `medium` (the default), or `high`; it has no `xhigh` level. A 1,048,576-token unified context; 65,536 output tokens by default, up to 1,048,576 with long decode. Priced at an introductory $2 / $10 per million input/output tokens through 2026-11-30, then $4 / $20 from 2026-12-01; Batch and Flex at half. Not on the Free tier. Only implicit caching is supported, and `temperature`, `top_p`, and `top_k` are ignored. Google's model-choice guide no longer lists it, so route new work to Neon. Keep Argon only where a workload is already tuned to it. A move to Neon needs no request schema change; set the thinking level explicitly, because Neon defaults to `high`. In the Antigravity CLI it is selected as `gemini-4-argon-low`, `-medium`, or `-high`.

### Gemini 3.8 Flash (gemini-3.8-flash) - mid

Google's most intelligent Flash model, generally available since 2026-09-02 and engineered for long-horizon software engineering, autonomous agents, and complex enterprise workflows at Flash speed and cost. Text, image, video, audio, and PDF in; thinking at low, medium, or high effort (`minimal` is rejected). 1,048,576-token input context, 65,536-token max output. Priced at $0.75 / $3.75 per million input/output tokens through 2026-12-31, then $1.50 / $7.50 from 2027-01-01. Google now names Gemini 4 Hydrogen as its upgrade. Until 2026-12-31 it costs half as much as Hydrogen; after that the prices are the same. It is also the only Gemini model on Antigravity's Free and Plus plans. Recommended roles: coding and review where cost binds. Route new day-to-day coding to Hydrogen.

### Gemini 3.5 Flash-Lite (gemini-3.5-flash-lite) - fast

Google's fastest and lowest-cost general model, optimized for high-throughput, low-cost work: sub-agent tasks, document parsing, and simple data extraction where latency and API cost are the binding constraints. 1,048,576-token input context and a separate 65,536-token max output, priced at $0.30 / $2.50 per million tokens with no introductory period. Recommended role: bulk. Use it for classification, extraction, mechanical edits, and high-volume agentic subtasks run at scale.

### Gemini 3.1 Pro Preview (gemini-3.1-pro-preview) - legacy

Google's previous Pro model, still served with no shutdown date announced. It still carries a preview label. Google's model-choice guide now names `gemini-4-neon` as its upgrade (or `gemini-4-carbon` for the hardest problems). 1,048,576-token input context, 65,536-token max output, priced at $2 / $12 per million input/output tokens for prompts up to 200K tokens (rising to $4 / $18 above that). Keep it only where a workload depends on explicit context caching, which no Gemini 4 model supports. In Antigravity it is offered on Enterprise only. Migrate other pins to `gemini-4-neon`.

### Gemini 3.7 Flash (gemini-3.7-flash) - legacy

Released 2026-08-13, superseded by 3.8 Flash on 2026-09-02, and deprecated on 2026-10-08. The endpoint is not shutting down: API requests to `gemini-3.7-flash` are served by `gemini-3.8-flash`, and Google AI Studio no longer offers it. Migrate pins to `gemini-4-hydrogen` (or to `gemini-3.8-flash` where cost binds), so the configured id names the model that does the work.

### Gemini 3.6 Flash (gemini-3.6-flash) - legacy

The previous Flash generation, still served by Google on the same price schedule as 3.7 and 3.8 Flash ($0.75 / $3.75 per million tokens through 2026-12-31, $1.50 / $7.50 after). Google names Gemini 4 Hydrogen as its upgrade, so migrate any saved configuration to `gemini-4-hydrogen`.

### Gemini 3 Pro Preview (gemini-3-pro) - legacy

Previously Google's frontier reasoning model, Gemini 3 Pro Preview (`gemini-3-pro-preview`) launched November 2025 and was shut down on 2026-03-09; its listed replacement was `gemini-3.1-pro-preview`. It stays in this registry because seeded usage telemetry references it under the shorter id `gemini-3-pro`; do not route new work to it, and migrate any saved configuration to `gemini-4-neon`.

## Governance

- Require a human review gate before merging any change a Gemini 4 frontier model (Neon, Carbon, or Argon) produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs). Their reasoning depth is strong but not infallible, and the blast radius of an autonomous frontier session is larger than a supervised one.
- Route plan, plan-check, and review to Gemini 4 Neon, with the thinking level set explicitly. Use `xhigh` only when `high` is not enough, because it spends the most tokens.
- Escalate to Gemini 4 Carbon only for the hardest problems or when Neon gets stuck. Budget for Carbon's price doubling on 2026-12-01 ($4 / $20 per million tokens from then).
- Route day-to-day coding to Gemini 4 Hydrogen. Keep Gemini 3.8 Flash only where cost binds before 2026-12-31.
- Route bulk, mechanical, and high-volume work to Gemini 3.5 Flash-Lite to hold spend down. On output tokens it is 3x cheaper than Hydrogen and 6x cheaper than Neon.
- Treat Gemini 4 Argon as a held-over choice: keep it where a workload is tuned to it, and point new configuration at Neon.
- Treat Gemini 3.1 Pro Preview as legacy: keep it only for workloads that depend on explicit context caching.
- Treat Gemini 3.7 Flash and 3.6 Flash as legacy, and Gemini 3 Pro Preview (`gemini-3-pro`) as retired: Google shut it down on 2026-03-09.
- Routing policy: plan, plan-check, and review on Gemini 4 Neon; the hardest problems on Gemini 4 Carbon; code day-to-day on Gemini 4 Hydrogen; run bulk and mechanical edits on Gemini 3.5 Flash-Lite.

## Access routes

API prices as of 2026-11-20. Plan prices and OpenRouter prices as of 2026-10-24.

| Route | What you pay | Tool | Limits, per the vendor |
|---|---|---|---|
| Plan + vendor CLI | Google AI Pro $20/month; Google AI Ultra $100/month (5x Pro's tokens) or $200/month (20x) | Antigravity CLI (`agy`), included on every plan | Pro: a quota refreshed every five hours until a weekly limit. Ultra: the highest quota, refreshed every five hours. No numbers are published. Gemini models share one pool drawn down at API prices; non-Gemini models have a separate fixed limit. Overage runs on purchased AI credits only if the "AI Credit Overages" setting is `Always`. |
| Direct API | Gemini 4 Neon $3 / $15 per million input/output tokens; Carbon and Argon $2 / $10 through 2026-11-30, then $4 / $20; Hydrogen $1.50 / $7.50; 3.8 Flash $0.75 / $3.75 through 2026-12-31; 3.5 Flash-Lite $0.30 / $2.50 (Batch and Flex half) | Any harness with a Gemini API key | Metered per token. Of the Gemini 4 models, only Hydrogen is on the Free tier. |
| OpenRouter | `google/gemini-4-argon` $2 / $10; `google/gemini-4-argon:batch` $1 / $5; `google/gemini-3.8-flash` $0.825 / $4.125 | Any OpenAI-compatible harness | Provider prices can change without notice. Neon, Carbon, and Hydrogen prices there are not checked yet. |

- In agy, Neon and Argon are on Google AI Pro (paid, or a trial on eligible premium devices) and Ultra. Hydrogen is on paid Pro and on Ultra. Carbon is on Ultra and Enterprise only. None of them is on Free or Plus, where Gemini 3.8 Flash is the only Gemini model. Gemini 3.1 Pro is on Enterprise only.
- Neon is the default model for new conversations in Antigravity. Neon and Hydrogen add an `xhigh` level, labeled Extra in the model selector. In agy, Neon's context window is up to 512K tokens, half the API's.
- agy also serves Claude Opus 5.5 and Claude Sonnet 5.5 (thinking) on Google AI Pro (paid subscriptions, not trials) and Ultra. That makes the Google plan a second route to Claude-family models that does not draw on an Anthropic plan.
- When a plan pays for itself, counting output tokens only: Pro at $20 matches about 1.3M Neon output tokens a month at $15 per M. Ultra at $100 matches about 6.7M, and Ultra at $200 about 13.3M. Input tokens push the real break-even earlier.
- Run repo-aware agent work through agy on a plan for daily use. Use a metered key for fast, small, occasional calls; the Batch tier halves the price for work that can wait.

## Sources

- https://ai.google.dev/gemini-api/docs/choosing-a-model (positioning, thinking levels, free tier, upgrade paths; retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/models/gemini-4-neon (retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/whats-new-gemini-4-neon (retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/models/gemini-4-carbon (retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/models/gemini-4-hydrogen (retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/models/gemini-4-argon (retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/thinking (thinking levels per model; retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/latest-model (Argon API changes and migration; retrieved 2026-10-24)
- https://ai.google.dev/gemini-api/docs/models (retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/models/gemini-3.1-pro-preview
- https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash
- https://ai.google.dev/gemini-api/docs/models/gemini-3.7-flash
- https://ai.google.dev/gemini-api/docs/models/gemini-3.6-flash
- https://ai.google.dev/gemini-api/docs/models/gemini-3.5-flash-lite
- https://ai.google.dev/gemini-api/docs/deprecations (release dates; retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/pricing (retrieved 2026-11-20)
- https://ai.google.dev/gemini-api/docs/changelog (release notes for Neon, Carbon, and Hydrogen; retrieved 2026-11-20)
- https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/model-routing.md
- https://antigravity.google/blog/changes-to-antigravity-plans/ (plan prices and multipliers, 2026-05-19; retrieved 2026-10-24)
- https://antigravity.google/docs/plans (quota and overage rules; retrieved 2026-10-24)
- https://antigravity.google/docs/models (models per plan in agy, agy ids, Neon's 512K context in agy; retrieved 2026-11-20)
- https://openrouter.ai/google/gemini-4-argon and https://openrouter.ai/google/gemini-4-argon:batch (retrieved 2026-10-24)
- https://openrouter.ai/api/v1/models (OpenRouter prices; retrieved 2026-10-24)
