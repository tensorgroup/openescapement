## Authentication & authorization

- Never build custom login, session, or password storage. The org SSO service handles authentication: https://sso.acme.example/docs.
- Use the approved OIDC flow with the org identity provider for every new service, through `acme-auth-go` or `acme-auth-ts`.
- Put authorization checks at the API layer. Use the central policy service where it is available.
