---
targets: [claude, agents]
---
# Claude Opus 5 governance (claude-opus-5)

- `claude-opus-5` is review-required, not banned: Anthropic recommends it as the default, and the maintainers' logged agentic use (2026-09) found it weaker than `claude-opus-4-8` at the same price. Do not pin it as a default or pass it as an override without logged evidence.
- Reversal condition: at least 10 logged reviews over 30 days in which Opus 5's confirmed-finding rate meets or beats Opus 4.8's. Meet it, record it, and flip this rule.
- Until then, route planning, plan-checking, and review to `claude-fable-5-1`, or `claude-opus-4-8` where price or retention says so; keep day-to-day coding on `claude-sonnet-5`.
- Require a human review gate before merging any change Opus 5 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
