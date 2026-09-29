## Choosing a Claude model

Pick the model by the task, not by habit. Start at the top for anything that needs judgment. Step down for volume and speed. Treat every departure from the vendor's own default as a decision with a reason and a reversal condition.

| Model | ID | $/1M tokens (in/out) | Use for |
|---|---|---|---|
| Claude Fable 5.1 | `claude-fable-5-1` | $10 / $50 | The default for planning, design, review, debugging, and anything not simple. Thinking is always on. Requires 30-day data retention; its safety classifiers may refuse security-adjacent work. |
| Claude Opus 5.5 | `claude-opus-5-5` | $4 / $20 | The Opus-tier choice when price or the retention requirement rules Fable out: everyday coding, long agentic runs, planning behind a reviewed spec. Thinking is always on; default effort is `medium`. |
| Claude Sonnet 5.5 | `claude-sonnet-5-5` | $2 / $10 | Routine coding, high-volume pipelines, subagents, and executors working from a written plan with tests. Default effort is `high`, on recalibrated levels. |
| Claude Haiku 4.5 | `claude-haiku-4-5` | $1 / $5 | High-volume simple tasks: classification, summarization, extraction. 200K context. |
| Claude Opus 5 | `claude-opus-5` | $5 / $25 | Review required. Legacy; superseded by Opus 5.5 at a lower price. Migrate pins, do not start new work on it. |
| Claude Opus 4.8 | `claude-opus-4-8` | $5 / $25 | Review required. Legacy; this pack's Opus-tier choice until Opus 5.5. Migrate pins. |
| Claude Fable 5 | `claude-fable-5` | $10 / $50 | Review required. Superseded by 5.1 at the same price; migrate pins, do not start new work on it. |
| Claude Sonnet 5 | `claude-sonnet-5` | $2 / $10 | Review required. Legacy; superseded by Sonnet 5.5 at the same price. Migrate pins. |

Rules of thumb:

- Start with Fable 5.1 for work where being wrong is expensive: plans, reviews, debugging, design. Keep a fallback model configured, because its classifiers can refuse.
- Route every Fable 5.1 slot to Opus 5.5 while a plan's Fable usage cap is spent: the session, pinned subagents, and review seats. Do not fall back to Opus 5, because Opus 5.5 supersedes it at a lower price.
- Pin the `opus` alias to `claude-opus-5-5` in harness settings (Claude Code: `ANTHROPIC_DEFAULT_OPUS_MODEL`) before relying on it for a fallback. An alias resolves to whatever the client maps it to, which can be a model you ruled out.
- Use Opus 5.5 when the task is Opus-shaped or when Fable's price or data-retention requirement is a constraint. Anthropic recommends it as the starting point for most workloads.
- Set effort explicitly on Opus 5.5. Its default is `medium`, one level below Opus 4.8 and Opus 5, so a pin moved over without an effort setting quietly thinks less.
- Moving a pin from Opus 4.8 or Opus 5 to Opus 5.5 is not a drop-in swap. Thinking can no longer be disabled, and forced `tool_choice` (`any` or `tool`) returns a 400. Lower effort instead of disabling thinking, and use `auto` with `strict: true` tools instead of forcing a call.
- Moving a pin from Sonnet 5 to Sonnet 5.5 is not a drop-in swap. `thinking: {type: "disabled"}` and forced `tool_choice` return a 400: send `{type: "between_tools"}` (effort `high` or below) where thinking must stay off, and use `auto` with `strict: true` tools. The effort levels are recalibrated, so re-run the sweep rather than carrying the old setting over; start at `medium` for agentic coding.
- Pin the `sonnet` alias to `claude-sonnet-5-5` in harness settings (Claude Code: `ANTHROPIC_DEFAULT_SONNET_MODEL`), for the same reason as the `opus` alias.
- Step down to Sonnet 5.5 or Haiku 4.5 when cost or latency matters more than peak capability: bulk jobs, mechanical edits, subagent fan-out, an executor behind a written plan and tests.
- Use the exact model IDs in the table. Never guess or construct IDs, and never append date suffixes to them.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
