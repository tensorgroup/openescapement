---
targets: [agents]
---
# Model routing (OpenAI)

- Plan, plan-check, and review on `gpt-6-astra`. Reserve it for problems that need its full reasoning depth; it is priced like the top Anthropic tier. `gpt-5.6-sol` stays a review model where it already performs well.
- Code day-to-day on `gpt-5.6-terra`. It also covers review on lower-stakes diffs where a full Astra pass is not warranted.
- Run bulk and mechanical edits on `gpt-5.6-luna`. It is the cheapest tier in the family and fits classification, extraction, and simple agentic subtasks run at scale.
- Do not route new work to `gpt-5.2`; it is deprecated in Codex under ChatGPT sign-in. Migrate saved configurations and scheduled tasks to `gpt-5.6-terra` or `gpt-5.6-luna`.
- Require a human review gate before merging any change `gpt-6-astra` or `gpt-5.6-sol` produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).
