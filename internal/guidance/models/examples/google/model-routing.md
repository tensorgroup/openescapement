---
targets: [gemini, agents]
---
# Model routing (Google)

- Plan, plan-check, and review on `gemini-4-argon`, Google's flagship reasoning model. Set its thinking level explicitly; the default is `medium`.
- Keep `gemini-3.1-pro-preview` only for workloads that depend on explicit context caching, which Argon does not support. Point everything else at `gemini-4-argon`.
- Code day-to-day on `gemini-3.8-flash`. It also covers review on lower-stakes diffs where a full Argon pass is not warranted. Migrate `gemini-3.7-flash` and `gemini-3.6-flash` pins to it.
- Run bulk and mechanical edits on `gemini-3.5-flash-lite`. It fits sub-agent tasks, document parsing, and simple extraction run at scale, and it stays the cheapest tier even while 3.8 Flash's introductory price runs (through 2026-12-31).
- Do not route new work to `gemini-3-pro`; Google shut down Gemini 3 Pro Preview on 2026-03-09. Migrate saved configurations to `gemini-4-argon`.
- Require a human review gate before merging any change `gemini-4-argon` produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).
