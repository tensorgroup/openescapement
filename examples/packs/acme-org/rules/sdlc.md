## SDLC requirements

- Keep all code, including POCs and vibe-coded experiments, in a tracked repository under the org's GitHub organization.
- Run CI on every PR with tests, dependency scanning, and SAST. Use the shared workflow templates at https://github.com/acme/workflows.
- Do not disable, skip, or `--no-verify` past commit hooks or CI gates. A red gate is a finding to fix, not an obstacle to route around.
