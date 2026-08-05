---
targets: [agents]
---
# GPT-5.6 Luna governance (gpt-5.6-luna)

- Route bulk, mechanical, and high-volume work to `gpt-5.6-luna`: classification, extraction, mechanical edits, and simple agentic subtasks run at scale (1.05M-token context).
- It is the cheapest tier in the GPT-5.6 family, roughly 25x cheaper than Sol and 10x cheaper than Terra on output tokens.
- Send day-to-day coding to `gpt-5.6-terra` and planning or review to `gpt-5.6-sol`.
- Migrate any configuration still pointing at the legacy `gpt-5.2` to Luna or Terra.

See the OpenAI model guidance page for sources and full governance notes.
