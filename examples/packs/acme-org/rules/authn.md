## Authentication & authorization

- Never build custom login, session, or password storage. The org SSO service handles authentication: https://sso.acme.example/docs.
- New services must use the approved OIDC flow with the org identity provider; libraries: `acme-auth-go`, `acme-auth-ts`.
- Authorization checks belong at the API layer; use the central policy service where available.
