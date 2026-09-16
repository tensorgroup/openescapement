---
targets: [claude, agents]
---
# Claude Fable 5.1 governance (claude-fable-5-1)

- Use `claude-fable-5-1` for planning, plan-checking, and reviewing multi-file changes, and for any problem that is not simple (1M-token context; thinking always on).
- Configure a fallback model: its safety classifiers may decline security-adjacent work, and it requires 30-day data retention.
- Use `claude-opus-4-8` where Fable's price or retention requirement rules it out; keep day-to-day coding on `claude-sonnet-5` and bulk work on `claude-haiku-4-5`.
- Require a human review gate before merging any change Fable 5.1 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
