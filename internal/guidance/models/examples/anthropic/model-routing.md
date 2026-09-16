---
targets: [claude, agents]
---
# Model routing (Anthropic)

- Plan, plan-check, and review on `claude-fable-5-1`, with a fallback configured for requests its classifiers decline. Use `claude-opus-4-8` where Fable's price or retention requirement rules it out, and for autonomous agentic runs.
- `claude-opus-5` is review-required: same price as Opus 4.8, weaker in the maintainers' logged use (2026-09). Adopt it only on the recorded reversal condition. `claude-fable-5` is superseded; migrate pins to 5.1.
- Code day-to-day on `claude-sonnet-5`. It also covers review on lower-stakes diffs where a full frontier pass is not warranted.
- Run bulk and mechanical edits on `claude-haiku-4-5`. Do not route multi-step reasoning to it; it has no adaptive thinking.
- Require a human review gate before merging any change a frontier model produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).
