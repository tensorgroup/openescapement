---
targets: [claude, agents]
---
# Claude Sonnet 5 governance (claude-sonnet-5)

- Route day-to-day coding to `claude-sonnet-5`: near-frontier coding quality at a fraction of Claude Opus 5's cost (1M-token context).
- Also use it for review on lower-stakes diffs where a full frontier pass is not warranted; escalate risky or multi-file diffs to `claude-opus-5`.
- Keep planning and plan-checking on the frontier tier (`claude-opus-5`), not on Sonnet 5.
- Apply the org's standard review policy to Sonnet 5 diffs; reserve the mandatory human review gate for high-autonomy frontier-model changes.

See the Anthropic model guidance page for sources and full governance notes.
