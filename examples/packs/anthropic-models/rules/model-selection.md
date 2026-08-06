## Choosing a Claude model

Pick the model by the task, not by habit. Follow Anthropic's own guidance: start with Opus, step down for volume and speed, and reserve the top tier for work that has earned it.

| Model | ID | $/1M tokens (in/out) | Use for |
|---|---|---|---|
| Claude Opus 5 | `claude-opus-5` | $5 / $25 | The recommended starting point. Good at most things: everyday coding, complex features, refactors, autonomous agent runs, debugging. |
| Claude Sonnet 5 | `claude-sonnet-5` | $3 / $15 | Cost and latency saver. Near Opus quality on routine coding, high-volume pipelines, and subagents. |
| Claude Fable 5 | `claude-fable-5` | $10 / $50 | The deepest reasoning tier. Reserve for high-thinking tasks, code and design reviews, and problems that have failed on Opus or are not simple or obvious. |
| Claude Haiku 4.5 | `claude-haiku-4-5` | $1 / $5 | High-volume simple tasks: classification, summarization, extraction. |

Rules of thumb:

- Start with Opus 5. It is Anthropic's recommended default and handles most work well.
- Step down to Sonnet 5 or Haiku 4.5 when cost or latency matters more than peak capability: bulk jobs, mechanical edits, subagent fan-out.
- Escalate to Fable 5 when the work calls for its depth: hard multi-step reasoning, thorough reviews, or a problem that failed on Opus or does not look simple or obvious. Note it requires 30-day data retention, and its safety classifiers may refuse security-adjacent work.
- Use the exact model IDs in the table. Never guess or construct IDs, and never append date suffixes to them.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
