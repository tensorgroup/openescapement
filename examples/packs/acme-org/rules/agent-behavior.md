## Agent behavior

- Stop an orchestration run at 50 turns per goal. Leave a progress summary for a human. The cap exists so a run that is not converging is caught by a person, not by a budget alarm.
- Use fast models for mechanical edits and code-generation subagents. Reserve reasoning models for design, review, and security-sensitive decisions.
- Escalate to a human before: opening a network port, adopting a new cloud service, adding a new AI model or provider, or granting any credential access.
