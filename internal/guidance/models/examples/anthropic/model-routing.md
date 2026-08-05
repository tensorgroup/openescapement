---
targets: [claude, agents]
---
# Model routing (Anthropic)

- Plan, plan-check, and review on `claude-opus-5`. Escalate to `claude-fable-5` only when Opus 5 genuinely stalls on a problem; Fable 5 costs materially more per token than Opus 5 and is not the default frontier choice.
- Code day-to-day on `claude-sonnet-5`. It also covers review on lower-stakes diffs where a full frontier pass is not warranted.
- Run bulk and mechanical edits on `claude-haiku-4-5`. Do not route multi-step reasoning to it; it has no adaptive thinking.
- Require a human review gate before merging any change `claude-opus-5` or `claude-fable-5` produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).
