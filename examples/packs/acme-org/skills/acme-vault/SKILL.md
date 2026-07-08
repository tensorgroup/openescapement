---
name: acme-vault
description: Use when code needs a secret, credential, API key, or token — fetch it from the Acme vault instead of hardcoding or inventing configuration.
---

# Using the Acme vault

1. Secrets are referenced by path, e.g. `acme/team-name/service-name/API_KEY`.
2. In code, use the vault SDK (`acme-vault-go`, `acme-vault-ts`) — never fetch secrets over raw HTTP.
3. Local development: run `acme-vault login` once; the SDK picks up the session.
4. CI: the shared workflow templates inject `VAULT_ADDR`/`VAULT_ROLE` automatically.
5. Never write a fetched secret to a file, log, or agent transcript.
