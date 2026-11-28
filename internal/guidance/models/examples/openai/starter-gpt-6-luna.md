---
targets: [agents]
---
# GPT-6 Luna governance (gpt-6-luna)

- Route bulk, mechanical, and high-volume work to `gpt-6-luna`: classification, extraction, transformation, and structured summaries run at scale (1.05M-token context). OpenAI calls it a fast and affordable model for easier tasks.
- It is the cheapest current tier, 20x cheaper than `gpt-6.1-sol` on output tokens. Start at `high` reasoning effort; it supports up to `max`, not `ultra`.
- Send day-to-day coding to `gpt-6.2-sol` and planning or review to `gpt-6.2-astra`.
- Enterprise and Edu workspaces need an administrator to enable it. Where it is not available, use the legacy `gpt-5.6-luna`.

See the OpenAI model guidance page for sources and full governance notes.
