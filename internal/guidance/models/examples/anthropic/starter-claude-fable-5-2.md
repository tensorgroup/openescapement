---
targets: [claude, agents]
---
# Claude Fable 5.2 governance (claude-fable-5-2)

- Hold `claude-fable-5-2` in reserve. It costs twice as much as `claude-opus-5-5` per token, and its default effort is `high`.
- Escalate to it only when a person asks for it, when the task is very hard or very high-stakes, or when the work is stuck after a failed first fix. Raise `claude-opus-5-5` effort first, because effort costs less than a model change.
- Configure `claude-opus-5-5` as its fallback. Its safety classifiers may decline security-adjacent work, and it requires 30-day data retention.
- Return to `claude-opus-5-5` when the escalated task is done.
- Require a human review gate before merging any change Fable 5.2 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
