---
targets: [agents]
---
# GPT-5.6 Sol governance (gpt-5.6-sol)

- Use `gpt-5.6-sol` for review where it already performs well; it is the previous frontier model and still a capable reviewer (1.05M-token context).
- Move planning, plan-checking, and design work to `gpt-6-astra` as sessions roll over; send day-to-day coding to `gpt-5.6-terra` and bulk work to `gpt-5.6-luna`.
- Its promotional price runs through 2026-11-21; budget on the list price after that (figures on the guidance page).
- Require a human review gate before merging any change Sol produced with high autonomy (broad file access, destructive commands, or unsupervised multi-step runs).

See the OpenAI model guidance page for sources and full governance notes.
