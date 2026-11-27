## Choosing a Claude model

Start every task on Claude Opus 5.5 at effort `medium`. Raise effort when the task needs more. Escalate to Claude Fable 5.2 only for the reasons below, because it costs twice as much. Treat every departure from this order as a decision with a reason.

| Model | ID | $/1M tokens (in/out) | Status | Use for |
|---|---|---|---|---|
| Claude Opus 5.5 | `claude-opus-5-5` | $4 / $20 | Preferred | The default for all work: planning, design, review, debugging, and coding. Start at effort `medium`. Thinking is always on. |
| Claude Fable 5.2 | `claude-fable-5-2` | $8 / $40 | Reserved | Escalation only. See the escalation rules below. Default effort is `high`. Requires 30-day data retention; its safety classifiers may refuse security-adjacent work. |
| Claude Sonnet 5.5 | `claude-sonnet-5-5` | $2 / $10 | Allowed | Routine coding, high-volume pipelines, subagents, and executors working from a written plan with tests. |
| Claude Haiku 4.5 | `claude-haiku-4-5` | $1 / $5 | Allowed | High-volume simple tasks: classification, summarization, extraction. 200K context. |
| Claude Fable 5.1 | `claude-fable-5-1` | $10 / $50 | Review required | Legacy; superseded by Fable 5.2 at a lower price. Migrate pins. |
| Claude Opus 5 | `claude-opus-5` | $5 / $25 | Review required | Legacy; superseded by Opus 5.5 at a lower price. Migrate pins, do not start new work on it. |
| Claude Opus 4.8 | `claude-opus-4-8` | $5 / $25 | Review required | Legacy; superseded by Opus 5.5. Migrate pins. |
| Claude Fable 5 | `claude-fable-5` | $10 / $50 | Review required | Legacy; superseded by Fable 5.2. Migrate pins. |
| Claude Sonnet 5 | `claude-sonnet-5` | $2 / $10 | Review required | Legacy; superseded by Sonnet 5.5 at the same price. Migrate pins. |

Escalation to Fable 5.2:

- Escalate in two steps. First raise Opus 5.5's effort to `high` or `xhigh`. Switch to Fable 5.2 only if that is not enough, because effort costs less than a model change.
- Use Fable 5.2 when a person asks for it by name.
- Use Fable 5.2 when the task is very hard or very high-stakes, and a wrong answer is expensive to undo.
- Use Fable 5.2 when the work is stuck: a first fix failed, the same bug came back, or review seats keep disagreeing after cross-examination.
- Record each escalation with its reason. Unrecorded escalations cannot show whether the reserved model earns its price.
- Return to Opus 5.5 when the escalated task is done. An escalation covers one task, not the rest of the session.
- Keep a second model from another vendor as the check on Opus 5.5's work. Cross-vendor review catches more than a more expensive model from the same vendor.

Rules of thumb:

- Set effort explicitly on Opus 5.5. Its default is `medium`, one level below Opus 4.8 and Opus 5, so a pin moved over without an effort setting quietly thinks less.
- Pin the `opus` alias to `claude-opus-5-5` in harness settings (Claude Code: `ANTHROPIC_DEFAULT_OPUS_MODEL`). An alias resolves to whatever the client maps it to, which can be a model you ruled out.
- Pin the `sonnet` alias to `claude-sonnet-5-5` in harness settings (Claude Code: `ANTHROPIC_DEFAULT_SONNET_MODEL`), for the same reason.
- Route a Fable 5.2 slot to Opus 5.5 while a plan's Fable usage cap is spent. Do not fall back to Opus 5, because Opus 5.5 supersedes it at a lower price.
- Moving a pin from Opus 4.8 or Opus 5 to Opus 5.5 is not a drop-in swap. Thinking can no longer be disabled, and forced `tool_choice` (`any` or `tool`) returns a 400. Lower effort instead of disabling thinking, and use `auto` with `strict: true` tools.
- Moving a pin from Sonnet 5 to Sonnet 5.5 is not a drop-in swap. `thinking: {type: "disabled"}` and forced `tool_choice` return a 400: send `{type: "between_tools"}` (effort `high` or below) where thinking must stay off, and use `auto` with `strict: true` tools. Re-run the effort sweep; start at `medium` for agentic coding.
- Step down to Sonnet 5.5 or Haiku 4.5 when cost or latency matters more than peak capability: bulk jobs, mechanical edits, subagent fan-out, an executor behind a written plan and tests.
- Use the exact model IDs in the table. Never guess or construct IDs, and never append date suffixes to them.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
