# Examples

- `packs/acme-org/` — a complete example rule pack: build/SDLC/hosting/agent-behavior rules, a paved-path pointer table, a tool catalog (preferred/allowed/review-required/banned), a skill, an MCP server entry, and merge constraints. Copy it into your own `policy-packs` repo and adapt.
- `packs/anthropic-models/`, `packs/openai-models/`, `packs/zai-models/` — per-vendor model packs: a catalog of models with price, role, and status (preferred / allowed / review-required / banned), each status carrying its reason and, where it departs from the vendor's own default, the condition that would reverse it. Rendered as a status-labelled policy list in the managed block and a readable table in `GOVERNANCE.md`. Prices and lineups move; bump the pack version when they do.
- `packs/model-seats/` — governance for multi-model agent collaboration: seats named by role (moderator, planning lead, peers, on-demand), a dispute substance bar, mandatory panel-outcome logging, and evidence-cited demotion with explicit re-promotion criteria. The catalog carries a dated reference roster so the models are visibly a config detail; copy and adapt, and replace that roster, and the scoreboard numbers and dates cited in its notes, with your own. Distilled from a real seat-policy decision; pairs with the [Balancewheel](https://github.com/tensorgroup/balancewheel) scoreboard that generates the evidence.
- `governed-service/` — a repo governed by the acme-org pack via a local-path source. Try it:

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

Maintenance note: any change to escapement's rendered output (notice text, catalog formatting, block layout) changes these artifacts AND the hashes in `governed-service/.escapement/escapement.lock`. Regenerate both together by running `go run ../../cmd/esc sync` inside `governed-service/` and committing the result; `TestExamplesLockMatchesShippedFiles` fails if the pair ever drifts apart.
