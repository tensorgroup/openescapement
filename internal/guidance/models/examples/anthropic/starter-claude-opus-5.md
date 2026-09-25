---
targets: [claude, agents]
---
# Claude Opus 5 governance (claude-opus-5)

- `claude-opus-5` is review-required: `claude-opus-5-5` supersedes it at a lower price, and the maintainers' logged agentic use (2026-09) found it weaker than the Opus 4.8 it was meant to replace. Do not start new work on it.
- Migrate existing pins to `claude-opus-5-5`, setting effort explicitly (Opus 5.5 defaults to `medium`, not `high`).
- Route planning, plan-checking, and review to `claude-fable-5-1`, or `claude-opus-5-5` where price or retention says so; keep day-to-day coding on `claude-sonnet-5`.
- Require a human review gate before merging any change Opus 5 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
