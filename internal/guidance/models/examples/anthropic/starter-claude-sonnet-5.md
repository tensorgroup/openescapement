---
targets: [claude, agents]
---
# Claude Sonnet 5 governance (claude-sonnet-5)

- Route day-to-day coding to `claude-sonnet-5`: near-frontier coding quality at a fraction of the frontier tier's cost (1M-token context), and the executor behind a written plan with tests.
- Also use it for review on lower-stakes diffs where a full frontier pass is not warranted; escalate risky or multi-file diffs to `claude-fable-5-1`, or `claude-opus-4-8` where price or retention says so.
- Keep planning and plan-checking on the frontier tier, not on Sonnet 5.
- Apply the org's standard review policy to Sonnet 5 diffs; reserve the mandatory human review gate for high-autonomy frontier-model changes.

See the Anthropic model guidance page for sources and full governance notes.
