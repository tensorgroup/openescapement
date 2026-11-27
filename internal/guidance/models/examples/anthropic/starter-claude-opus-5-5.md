---
targets: [claude, agents]
---
# Claude Opus 5.5 governance (claude-opus-5-5)

- Use `claude-opus-5-5` as the default model for all work: planning, plan-checking, review, debugging, and long agentic runs (1M-token context).
- Escalate to `claude-fable-5-2` only when a person asks, when the task is very hard or very high-stakes, or when the work is stuck. Fable 5.2 costs twice as much.
- Start at effort `medium` and set it explicitly. Raise effort before you escalate the model. `medium` is one level below Opus 4.8 and Opus 5, so a pin moved over without an effort setting quietly thinks less.
- Thinking cannot be disabled and forced `tool_choice` (`any` or `tool`) returns a 400. Lower effort instead, and use `auto` with `strict: true` tools.
- Keep day-to-day coding on `claude-sonnet-5-5` and bulk work on `claude-haiku-4-5`.
- Require a human review gate before merging any change Opus 5.5 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
