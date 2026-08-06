# Examples

- `packs/acme-org/` — a complete example rule pack: build/SDLC/hosting/agent-behavior rules, a paved-path pointer table, a tool catalog (preferred/allowed/review-required/banned), a skill, an MCP server entry, and merge constraints. Copy it into your own `policy-packs` repo and adapt.
- `governed-service/` — a repo governed by that pack via a local-path source. Try it:

```sh
cd examples/governed-service
go run ../../cmd/esc sync     # renders CLAUDE.md, AGENTS.md, GEMINI.md, GOVERNANCE.md, skills, .mcp.json
go run ../../cmd/esc status   # everything in sync
sed -i '' 's/Use Vault/Use whatever/' CLAUDE.md
go run ../../cmd/esc status --check   # exit 1: managed block altered
go run ../../cmd/esc diff     # see exactly what drifted
go run ../../cmd/esc sync     # repair
```

The rendered artifacts in `governed-service/` are committed on purpose — they show what escapement output looks like.
