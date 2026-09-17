# Google

Google's current lineup centers on the Gemini 3 family: Gemini 3.1 Pro Preview at the frontier, Gemini 3.8 Flash in the middle, and Gemini 3.5 Flash-Lite for fast, high-volume work. Gemini 3.7 Flash, released three weeks before 3.8 at the same price, stays current for configurations that pinned it; Gemini 3.6 Flash and the original Gemini 3 Pro Preview are held in this registry as legacy ids. The tiers fit an agentic SDLC cleanly: plan and review on 3.1 Pro Preview, implement day-to-day on 3.8 Flash, and run bulk work on Flash-Lite. Prices below are the Standard tier; Batch and Flex run at half.

## Models

### Gemini 3.1 Pro Preview (gemini-3.1-pro-preview) - frontier

Google's most advanced reasoning model, built to refine the Gemini 3 Pro series with better thinking, improved token efficiency, and a more grounded, factually consistent output. It is optimized for software engineering behavior and for agentic workflows that need precise tool use and reliable multi-step execution across real-world domains. 1,048,576-token input context, 65,536-token max output, priced at $2 / $12 per million input/output tokens for prompts up to 200K tokens (rising to $4 / $18 above that). Recommended roles: planning, plan-check, and review. Use it to plan multi-file changes, plan-check another agent's plan, and review diffs for risk. It still carries a preview release label even though it is Google's current frontier model; treat that as a production caveat, not a reason to avoid it.

### Gemini 3.8 Flash (gemini-3.8-flash) - mid

Google's most intelligent Flash model, generally available since 2026-09-02 and engineered for long-horizon software engineering, autonomous agents, and complex enterprise workflows at Flash speed and cost. Text, image, video, audio, and PDF in; thinking at low, medium, or high effort (`minimal` is rejected). 1,048,576-token input context, 65,536-token max output. Priced at $0.75 / $3.75 per million input/output tokens through 2026-12-31, then $1.50 / $7.50 from 2027-01-01. Recommended roles: coding and review. This is the default day-to-day coding model, and it also covers review on lower-stakes diffs where a full Pro Preview pass is not warranted.

### Gemini 3.7 Flash (gemini-3.7-flash) - mid

Released 2026-08-13 as Google's workhorse model for coding and agents, then superseded by 3.8 Flash on 2026-09-02 at the same limits and the same price schedule ($0.75 / $3.75 per million tokens through 2026-12-31, $1.50 / $7.50 after). Recommended role: coding, only where a configuration already pins it; there is no price or capability reason to start new work on it rather than 3.8 Flash.

### Gemini 3.5 Flash-Lite (gemini-3.5-flash-lite) - fast

Google's fastest and most cost-effective current model, optimized for high-throughput, low-cost work: sub-agent tasks, document parsing, and simple data extraction where latency and API cost are the binding constraints. Same 1,048,576-token input context and 65,536-token max output as the larger Gemini 3 models, priced at $0.30 / $2.50 per million tokens with no introductory period. Recommended role: bulk. Use it for classification, extraction, mechanical edits, and high-volume agentic subtasks run at scale.

### Gemini 3.6 Flash (gemini-3.6-flash) - legacy

The previous Flash generation, still served by Google on the same price schedule as 3.7 and 3.8 Flash ($0.75 / $3.75 per million tokens through 2026-12-31, $1.50 / $7.50 after). Both successors are more capable at the same price, so migrate any saved configuration to `gemini-3.8-flash`.

### Gemini 3 Pro Preview (gemini-3-pro) - legacy

Previously Google's frontier reasoning model, Gemini 3 Pro Preview (`gemini-3-pro-preview`) launched November 2025 and was shut down on 2026-03-09; its recommended replacement is `gemini-3.1-pro-preview`. It stays in this registry because seeded usage telemetry references it under the shorter id `gemini-3-pro`; do not route new work to it, and migrate any saved configuration to `gemini-3.1-pro-preview`.

## Governance

- Require a human review gate before merging any change Gemini 3.1 Pro Preview produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs); its reasoning depth is strong but it still carries a preview label, and the blast radius of an autonomous frontier session is larger than a supervised one.
- Route bulk, mechanical, and high-volume work to Gemini 3.5 Flash-Lite to hold spend down. Through 2026-12-31 it is 2.5x cheaper than 3.8 Flash on input and 1.5x cheaper on output ($2.50 vs $3.75 per million); from 2027-01-01, when Flash's introductory price ends, the gap widens to 5x and 3x. Against 3.1 Pro Preview it is roughly 5x cheaper on output ($2.50 vs $12 per million, prompts up to 200K tokens).
- Treat Gemini 3.6 Flash as legacy: 3.8 Flash is more capable at the same price, so there is no configuration that should still prefer 3.6.
- Treat Gemini 3 Pro Preview (`gemini-3-pro`) as retired: Google shut it down on 2026-03-09 and its recommended replacement is `gemini-3.1-pro-preview`, so point new configuration there directly.
- Routing policy: plan, plan-check, and review on Gemini 3.1 Pro Preview; code day-to-day on Gemini 3.8 Flash; run bulk and mechanical edits on Gemini 3.5 Flash-Lite.

## Sources

- https://ai.google.dev/gemini-api/docs/models
- https://ai.google.dev/gemini-api/docs/models/gemini-3.1-pro-preview
- https://ai.google.dev/gemini-api/docs/models/gemini-3.8-flash
- https://ai.google.dev/gemini-api/docs/models/gemini-3.7-flash
- https://ai.google.dev/gemini-api/docs/models/gemini-3.6-flash
- https://ai.google.dev/gemini-api/docs/models/gemini-3.5-flash-lite
- https://ai.google.dev/gemini-api/docs/deprecations
- https://ai.google.dev/gemini-api/docs/pricing
- https://ai.google.dev/gemini-api/docs/changelog
- https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/model-routing.md
