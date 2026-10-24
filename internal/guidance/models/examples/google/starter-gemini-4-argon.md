---
targets: [gemini, agents]
---
# Gemini 4 Argon governance (gemini-4-argon)

- Route planning, plan-checking, and review to `gemini-4-argon`: Google's flagship reasoning model, generally available, with a 1,048,576-token context window shared by input and output.
- Set the thinking level explicitly (`low`, `medium`, or `high`; the default is `medium`) and test the workload at each, since the level changes both quality and spend.
- In the Antigravity CLI (`agy`), select it by an effort-suffixed id: `gemini-4-argon-low`, `gemini-4-argon-medium`, or `gemini-4-argon-high`.
- Remove explicit context caching, `temperature`, `top_p`, and `top_k` when migrating from `gemini-3.1-pro-preview`; Argon does not support explicit caching and ignores the sampling parameters. Keep `gemini-3.1-pro-preview` only where a workload depends on explicit caching.
- Require a human review gate before merging any change it produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Google model guidance page for sources and full governance notes.
