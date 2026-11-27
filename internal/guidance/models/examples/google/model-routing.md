---
targets: [gemini, agents]
---
# Model routing (Google)

- Plan, plan-check, and review on `gemini-4-neon`. It is also the pick for complex agentic coding. Set its thinking level explicitly; the default is `high`, and `xhigh` is for tasks where `high` is not enough.
- Send the hardest problems to `gemini-4-carbon`: system architecture, multi-file debugging, security audits, or a task where Neon gets stuck. Its default thinking level is `medium`, and its highest is `high`.
- Code day-to-day on `gemini-4-hydrogen`, the fastest and cheapest Gemini 4 model. It also covers review on lower-stakes diffs. Google names it the upgrade for `gemini-3.8-flash` and `gemini-3.6-flash`.
- Keep `gemini-3.8-flash` only where cost binds before 2026-12-31, while its introductory price is half of Hydrogen's.
- Run bulk and mechanical edits on `gemini-3.5-flash-lite`. It fits sub-agent tasks, document parsing, and simple extraction run at scale, and it is the cheapest tier.
- Keep `gemini-4-argon` only where a workload is already tuned to it. Point new configuration at `gemini-4-neon`.
- Keep `gemini-3.1-pro-preview` only for workloads that depend on explicit context caching, which no Gemini 4 model supports. Google names `gemini-4-neon` as its upgrade.
- Do not route new work to `gemini-3-pro`; Google shut down Gemini 3 Pro Preview on 2026-03-09. Migrate saved configurations to `gemini-4-neon`.
- Require a human review gate before merging any change a frontier model (`gemini-4-neon`, `gemini-4-carbon`, or `gemini-4-argon`) produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).
