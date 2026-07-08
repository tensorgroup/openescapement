## Agent behavior

- Orchestration runs are limited to 50 turns per goal; if a task exceeds this, stop and summarize progress for a human instead of continuing.
- Model routing: use fast models for mechanical edits and code generation subagents; reserve reasoning models for design, review, and security-sensitive decisions.
- Escalate to a human before: opening a network port, adopting a new cloud service, adding a new AI model or provider, or granting any credential access.
