---
targets: [gemini, agents]
---
# Gemini 4 Argon governance (gemini-4-argon)

- Route new planning, plan-checking, and review to `gemini-4-neon`, not `gemini-4-argon`. Argon is still served with no shutdown date, but Google now leads with Neon, Carbon, and Hydrogen.
- Keep `gemini-4-argon` where a workload is already tuned to it. Google describes it as a more efficient model than Carbon, with faster responses and lower token spend per turn.
- Set the thinking level explicitly (`low`, `medium`, or `high`; the default is `medium`). Argon has no `xhigh` level.
- In the Antigravity CLI (`agy`), select it by an effort-suffixed id: `gemini-4-argon-low`, `gemini-4-argon-medium`, or `gemini-4-argon-high`.
- Require a human review gate before merging any change it produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the Google model guidance page for sources and full governance notes.
