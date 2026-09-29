---
targets: [claude, agents]
---
# Claude Opus 5.5 governance (claude-opus-5-5)

- Use `claude-opus-5-5` as the Opus-tier model: planning, plan-checking, and review where `claude-fable-5-1`'s price or retention requirement rules it out, and for long agentic runs that need Opus-level judgment (1M-token context).
- Set effort explicitly. Its default is `medium`, one level below Opus 4.8 and Opus 5, so an unset effort quietly thinks less than the pin it replaced.
- Thinking cannot be disabled and forced `tool_choice` (`any` or `tool`) returns a 400. Lower effort instead, and use `auto` with `strict: true` tools.
- Keep day-to-day coding on `claude-sonnet-5-5` and bulk work on `claude-haiku-4-5`.
- Require a human review gate before merging any change Opus 5.5 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
