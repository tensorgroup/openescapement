---
targets: [claude, agents]
---
# Claude Opus 5 governance (claude-opus-5)

- Use `claude-opus-5` as the default frontier model for planning, plan-checking, and reviewing multi-file changes (1M-token context, $5 / $25 per million input/output tokens).
- Do not route routine day-to-day coding here; send that to `claude-sonnet-5` and reserve Opus 5 for work that needs its reasoning depth.
- Escalate to `claude-fable-5` only when Opus 5 genuinely stalls: Fable 5 costs double per token and runs slower.
- Require a human review gate before merging any change Opus 5 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
