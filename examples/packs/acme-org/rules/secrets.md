## Secrets & keys

- Never write API keys, tokens, or credentials inline in code, config files, or agent instructions.
- Keep secrets in the org vault: https://vault.acme.example. Request access in #platform-team.
- Get API keys for AI providers from https://keys.acme.example. Do not create personal provider accounts for org work: the org can rotate and revoke a centrally issued key, and cannot touch a personal one.
- If you find a hardcoded secret, stop and flag it to the developer. Do not copy or move it: moving a leaked secret spreads it without rotating it.
