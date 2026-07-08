## SDLC requirements

- All code — including POCs and vibe-coded experiments — lives in a tracked repository under the org's GitHub organization.
- CI runs on every PR and must include: tests, dependency scanning, and SAST. Use the shared workflow templates at https://github.com/acme/workflows.
- Do not disable, skip, or `--no-verify` past commit hooks or CI gates.
