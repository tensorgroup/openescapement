---
targets: [agents]
---
# Model routing (OpenAI)

- Plan, plan-check, and review on `gpt-6.2-astra`. Reserve it for problems that need its full reasoning depth, and check its price on OpenAI's pricing page before running it at scale.
- Code day-to-day on `gpt-6.2-sol`. It also covers review on lower-stakes diffs where a full Astra pass is not warranted.
- Run bulk and mechanical edits on `gpt-6-luna`, the cheapest current tier.
- Send visual-understanding and computer-use tasks to `gpt-6.1-terra`.
- Where Codex under a ChatGPT sign-in does not yet offer a GPT-6.2 model, fall back to the previous generation: `gpt-6-astra` for judgment, `gpt-6.1-sol` for coding.
- Move new work off the legacy models: `gpt-6-sol` and the GPT-5.6 family. Move off `gpt-5.5`, which left Codex on 2026-10-14. Do not route new work to `gpt-5.2`, which Codex under ChatGPT sign-in already rejects. Migrate saved configurations and scheduled tasks to `gpt-6.2-sol` or `gpt-6-luna`.
- Require a human review gate before merging any change `gpt-6.2-astra` or `gpt-6-astra` produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).
