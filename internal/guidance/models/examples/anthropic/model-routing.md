---
targets: [claude, agents]
---
# Model routing (Anthropic)

- Plan, plan-check, review, and debug on `claude-opus-5-5`, starting at effort `medium`. Raise effort before you change model.
- Hold `claude-fable-5-2` in reserve. It costs twice as much as Opus 5.5. Escalate to it only when a person asks, when the task is very hard or very high-stakes, or when the work is stuck. Configure `claude-opus-5-5` as its fallback for requests its classifiers decline.
- Route every `claude-fable-5-2` slot to `claude-opus-5-5` while a plan's Fable usage cap is spent, never to `claude-opus-5`. Pin the `opus` alias to `claude-opus-5-5` in harness settings, because an alias resolves to whatever the client maps it to.
- `claude-opus-5` and `claude-opus-4-8` are legacy and review-required: Opus 5.5 supersedes both at a lower price, so migrate their pins to it. `claude-fable-5` and `claude-fable-5-1` are superseded; migrate pins to `claude-fable-5-2`.
- Code day-to-day on `claude-sonnet-5-5`. It also covers review on lower-stakes diffs where a full frontier pass is not warranted. `claude-sonnet-5` is superseded at the same price; migrate pins to 5.5, re-running the effort sweep, and pin the `sonnet` alias to `claude-sonnet-5-5` in harness settings.
- Run bulk and mechanical edits on `claude-haiku-4-5`. Do not route multi-step reasoning to it; it has no adaptive thinking.
- Require a human review gate before merging any change a frontier model produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).
