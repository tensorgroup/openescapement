# Licensing & Open-Core Split

**Status:** Recommendation applied (Apache-2.0 on this repo); revisit before first external release announcement. Changing license is easy *now* (single contributor) and painful later (needs every contributor's consent) — decide before accepting outside PRs.

## The license decision

| Option | For | Against | Verdict |
|---|---|---|---|
| **Apache-2.0** (chosen) | Explicit patent grant; the default for infra tools whose strategy is "become the standard" (Kubernetes, Terraform pre-BUSL, OpenTelemetry); zero friction in enterprise/university legal review — critical for the Caltech-shaped buyer | Permits closed-source forks and hosted copycats | ✅ **Recommended** |
| MIT | Simplest, equally adoption-friendly | No patent grant — matters once a foundation or big co-contributor gets involved | Fine, but strictly weaker than Apache-2.0 here |
| AGPL-3.0 | Blocks hosted copycats without reciprocity | Security-sensitive and university counsel routinely ban AGPL dependencies; kills the "adopted by a single team in an afternoon" motion; poisons the community-packs flywheel | ❌ Fights the strategy |
| BUSL / FSL (source-available) | Strongest copycat protection | Not open source; forfeits the "open standard" thesis entirely — §8 of the product spec argues the openness *is* the moat | ❌ Contradicts the thesis |

**Why permissive is strategically correct for this product specifically:** the spec's defense against copyability is that the rule-pack *format* becomes the standard way orgs express AI policy. Standards win by being frictionless to adopt and safe to build on. Every community pack, integration, and blog post compounds our position; a copyleft license would suppress exactly that compounding. The copycat risk Apache-2.0 leaves open is real but is the risk the strategy already chose to run — and mitigations below don't require licensing.

**Non-license defenses (do these):**
1. **Trademark.** Register "OpenEscapement" (and decide on "esc"/"escapement" usage policy). Anyone can fork the code; nobody can fork the name. This is the actual moat for permissive-licensed projects (Firefox, WordPress, Docker).
2. **Signed releases.** cosign/SLSA provenance on binaries — for a security tool, "the genuine build" is a trust asset forks can't inherit.
3. **The pack ecosystem.** Community packs live in *our* org/registry with provenance and versioning. Copycats get the code, not the library or its contributors.
4. **CLA or DCO.** Use DCO (lighter, community-friendlier) unless/until a relicensing option must be preserved — a CLA enables future relicensing but chills contribution. Given we intend to stay Apache-2.0, DCO is the lean choice.

## The open-core split

**Open source (this repo, Apache-2.0), forever:**
- Rule-pack format spec + schema validation
- Renderer (all targets) and managed-block mechanism
- `esc` CLI: sync, status, diff, update, signatures, lockfile
- GitHub Action
- Example/starter packs; future community-pack index metadata format
- Base MCP server (v0.2) — the *protocol surface* must be open or the standard play fails

**Paid (separate repo/product, proprietary or enterprise-licensed):**
- Hosted multi-tenant control plane: org registry at scale, team/project lifecycle + expiry nagging, cross-repo dashboard
- SIEM export (Splunk/Datadog/webhook/syslog)
- Review workflows (port openings, exceptions, vendor intake) + audit/evidence export (SOC 2, ISO 42001, NIST AI RMF)
- SSO/SCIM, RBAC at org scale
- Self-hosted enterprise licenses for institutions that can't use SaaS
- Support, SLAs

**The test for which side something lands on:** if it's needed for one team to govern its own repos, it's open. If its value comes from aggregation across many teams (visibility, workflows, evidence, identity), it's paid. This line is defensible in public and matches how successful open-core companies (GitLab, Grafana, HashiCorp-before-BUSL) explained themselves.

**Boring practicalities before announcing:**
- [ ] Add DCO check (`Signed-off-by`) or contributor policy note to CONTRIBUTING.md
- [ ] Trademark search + filing for "OpenEscapement"
- [ ] SECURITY.md with a disclosure contact (a security tool without one is embarrassing)
- [ ] goreleaser + cosign signed release pipeline before v0.1 binaries ship
