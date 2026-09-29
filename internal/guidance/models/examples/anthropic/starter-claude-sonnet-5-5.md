---
targets: [claude, agents]
---
# Claude Sonnet 5.5 governance (claude-sonnet-5-5)

- Route day-to-day coding to `claude-sonnet-5-5`: near-frontier coding quality at a fraction of the frontier tier's cost (1M-token context), and the executor behind a written plan with tests.
- Also use it for review on lower-stakes diffs where a full frontier pass is not warranted; escalate risky or multi-file diffs to `claude-fable-5-1`, or `claude-opus-5-5` where price or retention says so.
- Moving a pin from `claude-sonnet-5` is not a drop-in swap. `thinking: {type: "disabled"}` and forced `tool_choice` (`any` or `tool`) both return a 400: send `{type: "between_tools"}` (effort `high` or below) where thinking must stay off, and use `auto` with `strict: true` tools. Effort levels are recalibrated, so re-run the effort sweep instead of carrying the old setting over.
- Keep planning and plan-checking on the frontier tier, not on Sonnet 5.5.
- Apply the org's standard review policy to Sonnet 5.5 diffs; reserve the mandatory human review gate for high-autonomy frontier-model changes.

See the Anthropic model guidance page for sources and full governance notes.
