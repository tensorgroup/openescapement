---
targets: [claude, agents]
---
# Claude Opus 4.8 governance (claude-opus-4-8)

- Use `claude-opus-4-8` as the Opus-tier model: planning, plan-checking, and review where `claude-fable-5-1`'s price or retention requirement rules it out, and for autonomous agentic runs that need Opus-level judgment (1M-token context).
- Prefer it over `claude-opus-5` at the same price; see the guidance page for the recorded reason and the reversal condition.
- Keep day-to-day coding on `claude-sonnet-5` and bulk work on `claude-haiku-4-5`.
- Require a human review gate before merging any change Opus 4.8 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
