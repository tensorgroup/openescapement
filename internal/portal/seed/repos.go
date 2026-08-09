package seed

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Repos materializes, if absent, two on-disk git repos for `esc serve
// --demo`:
//
//   - dataDir/packs/org-baseline — a pack repo in the same shape
//     internal/portal/publish's Manager expects (git repo root == pack
//     root), tagged v1.1.0 (2 rules) then v1.2.0 (3 rules), so the packs
//     page has real version history to show.
//   - dataDir/demo-repo — a governed repo whose .escapement/config.yaml
//     points at the pack repo via file://, pinned to v1.2.0, trust:
//     unsigned. Running `esc sync` there is the other half of the portal
//     pitch: publish a rule change through the portal, then watch it land
//     in this repo's CLAUDE.md.
//
// Repos is idempotent by existence check: a directory that's already there
// is left untouched, so repeated `--demo` starts don't clobber whatever a
// prior demo session (or a user poking around the demo repo) left behind.
func Repos(dataDir string) (packDir, demoRepo string, err error) {
	packDir = filepath.Join(dataDir, "packs", "org-baseline")
	demoRepo = filepath.Join(dataDir, "demo-repo")

	if _, statErr := os.Stat(packDir); statErr != nil {
		if err := buildPackRepo(packDir); err != nil {
			return "", "", fmt.Errorf("seed pack repo: %w", err)
		}
	}
	if _, statErr := os.Stat(demoRepo); statErr != nil {
		if err := buildDemoRepo(demoRepo, packDir); err != nil {
			return "", "", fmt.Errorf("seed demo repo: %w", err)
		}
	}
	return packDir, demoRepo, nil
}

// securityRules12 is the fragment body at v1.2.0: the 3 memorable demo
// rules the pitch shows arriving in CLAUDE.md. v1.1.0 carries just the
// first two, so the pack's tag history has two real, different commits.
const securityFragHeader = "---\ntargets: [claude, agents, gemini]\n---\n# Security\n\n"

const securityRule1 = "- Never commit secrets or API keys.\n"
const securityRule2 = "- All authentication goes through the campus SSO service.\n"
const securityRule3 = "- Any newly opened port requires security review.\n"

// reconcileSkillMD is the demo pack's esc-reconcile skill. Reconciling a
// team's existing instructions against a pack's managed rules is judgment
// work: a CLI heuristic would be wrong often enough to erode trust, and a
// model call would break the single-dependency, no-network design. So it
// ships as a skill delivered through the same versioned, signed channel as
// every other rule, instead of code in esc itself.
const reconcileSkillMD = `---
name: esc-reconcile
description: Use after esc sync adds a managed block to an instruction file, to check the block against the human-authored content already in that file for duplicate or contradictory rules.
---

# Reconciling pack rules with existing instructions

'esc sync' writes a managed block into files like CLAUDE.md or AGENTS.md, but a team usually already has its own instructions in that file. The two can repeat each other or disagree. Spotting that is judgment work: a heuristic gets it wrong often enough to erode trust, and a model call inside the CLI would break esc's no-network, single-dependency design. So this check runs as a skill, in an agent, not in the CLI.

Do this for every instruction file that has a managed block:

1. Read the whole file: the managed block and all the surrounding, human-authored content. The block's markers are HTML comments, ` + "`<!-- escapement:begin packs=... hash=sha256:... -->`" + ` on its own line and ` + "`<!-- escapement:end -->`" + ` on its own line.
2. Compare each rule in the managed block against the surrounding content.
   - Duplicate: the surrounding text already states the same rule. Report it so the team can remove the redundant copy.
   - Contradiction: the surrounding text conflicts with, loosens, or tightens the rule. Report both sides, quoted, so the team can see the disagreement.
3. Propose edits only in the human-authored sections, outside the markers, to resolve what you found.
4. The hard rule: never edit anything between the begin and end markers. The two marker lines are part of the block, so never edit them either, including the ` + "`hash=`" + ` value on the begin line. Those bytes are what esc compares against the pack version it synced. Editing inside the markers, or the markers themselves, puts the file into the ` + "`altered`" + ` state, and ` + "`esc sync`" + ` will stop updating that file until a human resolves the drift by hand. So an in-marker edit does not just break a rule, it freezes that file's policy updates.

Report findings as a list: duplicates, contradictions, and the human-authored edit you propose for each. Do not modify the managed block yourself.
`

func packYAML(version string) string {
	return "schema: 1\n" +
		"name: org-baseline\n" +
		"version: " + version + "\n" +
		"description: Demo org baseline\n" +
		"rules:\n" +
		"  - rules/security.md\n" +
		"skills:\n" +
		"  - skills/esc-reconcile\n"
}

// buildPackRepo writes the org-baseline pack fixture at dir and commits it
// as two tagged versions: v1.1.0 (2 rules), then v1.2.0 (3 rules).
func buildPackRepo(dir string) error {
	if err := os.MkdirAll(filepath.Join(dir, "rules"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "skills", "esc-reconcile"), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "skills", "esc-reconcile", "SKILL.md"), []byte(reconcileSkillMD), 0o644); err != nil {
		return err
	}

	if err := writePackVersion(dir, "1.1.0", securityFragHeader+securityRule1+securityRule2); err != nil {
		return err
	}
	if _, err := gitDemo(dir, "init", "-q", "-b", "main"); err != nil {
		return err
	}
	if err := gitDemoIdentity(dir); err != nil {
		return err
	}
	if err := commitAndTag(dir, "1.1.0"); err != nil {
		return err
	}

	if err := writePackVersion(dir, "1.2.0", securityFragHeader+securityRule1+securityRule2+securityRule3); err != nil {
		return err
	}
	if err := commitAndTag(dir, "1.2.0"); err != nil {
		return err
	}
	return nil
}

func writePackVersion(dir, version, fragBody string) error {
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(packYAML(version)), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "rules", "security.md"), []byte(fragBody), 0o644)
}

func commitAndTag(dir, version string) error {
	if _, err := gitDemo(dir, "add", "-A"); err != nil {
		return err
	}
	if _, err := gitDemo(dir, "commit", "-q", "-m", "v"+version); err != nil {
		return err
	}
	if _, err := gitDemo(dir, "tag", "-a", "v"+version, "-m", "v"+version); err != nil {
		return err
	}
	return nil
}

// gitDemoIdentity sets a local (never global) git identity for the demo
// pack repo, so commits/tags don't depend on — or pollute — the caller's
// global git config.
func gitDemoIdentity(dir string) error {
	for _, kv := range [][2]string{
		{"user.name", "esc demo"},
		{"user.email", "demo@escapement.local"},
		{"commit.gpgsign", "false"},
		{"tag.gpgsign", "false"},
	} {
		if _, err := gitDemo(dir, "config", kv[0], kv[1]); err != nil {
			return err
		}
	}
	return nil
}

func gitDemo(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %v: %w\n%s", args, err, out)
	}
	return string(out), nil
}

// buildDemoRepo writes a governed repo at dir whose config points at
// packDir over file://, pinned to v1.2.0, trust: unsigned — matching the
// config YAML shape internal/cli's tests use for local pack sources.
func buildDemoRepo(dir, packDir string) error {
	if err := os.MkdirAll(filepath.Join(dir, ".escapement"), 0o755); err != nil {
		return err
	}
	cfg := "schema: 1\n" +
		"packs:\n" +
		"  - source: file://" + packDir + "\n" +
		"    ref: v1.2.0\n" +
		"    trust: unsigned\n"
	if err := os.WriteFile(filepath.Join(dir, ".escapement", "config.yaml"), []byte(cfg), 0o644); err != nil {
		return err
	}
	// Pre-existing user content, one file per target. On sync, esc appends its
	// managed block below this content and never touches these bytes — the
	// core renderer invariant the publish demo is meant to show.
	files := map[string]string{
		"CLAUDE.md": "# Payments service\n\n" +
			"Go 1.24 monorepo; run `make test` before pushing.\n" +
			"Ask in #payments-eng before changing the ledger schema.\n",
		"AGENTS.md": "# Payments service\n\n" +
			"Primary language is Go. Keep handlers thin and push logic into `internal/`.\n" +
			"Integration tests need a local Postgres; see `docs/dev-setup.md`.\n",
		"GEMINI.md": "# Payments service\n\n" +
			"This repo settles real money. Prefer boring, well-tested changes.\n" +
			"Never log full card numbers or auth tokens.\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}
