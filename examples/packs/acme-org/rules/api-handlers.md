---
targets: [rules]
paths:
  - "src/api/**"
---
# API handler rules

- Validate every request body at the trust boundary before use.
- Never log secrets, tokens, or full request bodies.
- Return typed errors; do not leak internal messages to clients.
