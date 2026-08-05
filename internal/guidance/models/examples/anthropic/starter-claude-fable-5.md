---
targets: [claude, agents]
---
# Claude Fable 5 governance (claude-fable-5)

- Reserve `claude-fable-5` for the hardest problems `claude-opus-5` cannot close: it costs materially more per token than Opus 5 and is slower in practice.
- Do not make Fable 5 the default frontier choice for routine planning or review; that burns budget with no quality gain over Opus 5.
- Keep day-to-day coding on `claude-sonnet-5` and bulk work on `claude-haiku-4-5`.
- Require a human review gate before merging any change Fable 5 produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Anthropic model guidance page for sources and full governance notes.
