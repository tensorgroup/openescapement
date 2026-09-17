## Choosing a Claude model

Pick the model by the task, not by habit. Start at the top for anything that needs judgment. Step down for volume and speed. Treat every departure from the vendor's own default as a decision with a reason and a reversal condition.

| Model | ID | $/1M tokens (in/out) | Use for |
|---|---|---|---|
| Claude Fable 5.1 | `claude-fable-5-1` | $10 / $50 | The default for planning, design, review, debugging, and anything not simple. Thinking is always on. Requires 30-day data retention; its safety classifiers may refuse security-adjacent work. |
| Claude Opus 4.8 | `claude-opus-4-8` | $5 / $25 | The Opus-tier choice when price or the retention requirement rules Fable out: everyday coding, agentic runs, planning behind a reviewed spec. |
| Claude Sonnet 5 | `claude-sonnet-5` | $2 / $10 (the launch rate, made permanent in 2026-08) | Routine coding, high-volume pipelines, subagents, and executors working from a written plan with tests. |
| Claude Haiku 4.5 | `claude-haiku-4-5` | $1 / $5 | High-volume simple tasks: classification, summarization, extraction. 200K context. |
| Claude Opus 5 | `claude-opus-5` | $5 / $25 | Review required. Anthropic's recommended default; see below for why this pack does not follow it. |
| Claude Fable 5 | `claude-fable-5` | $10 / $50 | Review required. Superseded by 5.1 at the same price; migrate pins, do not start new work on it. |

Rules of thumb:

- Start with Fable 5.1 for work where being wrong is expensive: plans, reviews, debugging, design. Keep a fallback model configured, because its classifiers can refuse.
- Use Opus 4.8 when the task is Opus-shaped or when Fable's price or data-retention requirement is a constraint. It is the same request surface as 4.7 and the safe pin for autonomous runs.
- Step down to Sonnet 5 or Haiku 4.5 when cost or latency matters more than peak capability: bulk jobs, mechanical edits, subagent fan-out, an executor behind a written plan and tests.
- Opus 5 is review-required, not banned. Anthropic recommends it as the default; the maintainers found it weaker than Opus 4.8 in logged agentic use (2026-09) at the same price. The reversal condition is concrete: at least 10 logged reviews over 30 days in which Opus 5's confirmed-finding rate meets or beats Opus 4.8's. When the condition is met, record the evidence and flip the status.
- Use the exact model IDs in the table. Never guess or construct IDs, and never append date suffixes to them.
- Model lineup and pricing change. This guidance is versioned and updated centrally; changes arrive through the pack, not by editing this block.
