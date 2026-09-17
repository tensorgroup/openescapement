---
targets: [gemini, agents]
---
# Model routing (Google)

- Plan, plan-check, and review on `gemini-3.1-pro-preview`. It still carries a preview label, but it is Google's current frontier model; reserve it for problems that need its full reasoning depth.
- Code day-to-day on `gemini-3.8-flash`. It also covers review on lower-stakes diffs where a full Pro Preview pass is not warranted. Migrate `gemini-3.7-flash` and `gemini-3.6-flash` pins to it.
- Run bulk and mechanical edits on `gemini-3.5-flash-lite`. It fits sub-agent tasks, document parsing, and simple extraction run at scale. Through 2026-12-31 it is not cheaper than 3.8 Flash on output tokens, so choose it for latency and throughput, and revisit when 3.8 Flash's introductory price ends.
- Do not route new work to `gemini-3-pro`; Google shut down Gemini 3 Pro Preview on 2026-03-09 and its recommended replacement is `gemini-3.1-pro-preview`. Migrate saved configurations directly.
- Require a human review gate before merging any change `gemini-3.1-pro-preview` produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).
