// Package engine orchestrates the sync pipeline: fetch → verify → parse →
// compose → render → validate → write → lock.
package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tensorgroup/openescapement/internal/config"
	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/lockfile"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
	"github.com/tensorgroup/openescapement/internal/source"
	"github.com/tensorgroup/openescapement/internal/targets"
)

// Artifact kinds — the four output mechanisms.
const (
	KindBlock    = "block"     // managed block inside a team-owned file
	KindFile     = "file"      // whole file owned by escapement
	KindDir      = "dir"       // whole directory owned by escapement
	KindJSONKeys = "json-keys" // owned keys inside a shared JSON file
)

// Artifact is one desired output on disk.
type Artifact struct {
	Path string // relative to repo root
	Kind string // one of the Kind* constants
	Hash string // canonical hash of the managed content
	Keys []string
	// Body is the managed-block body (kind=block) or full content (kind=file).
	Body string
	// SrcDir is the pack directory to copy from (kind=dir).
	SrcDir string
	// Servers are the owned MCP entries (kind=json-keys).
	Servers map[string]map[string]any
	// BlockPacks are the "name@version" labels recorded in a managed block's
	// header (kind=block). Built-in blocks carry every pack; a custom target
	// carries only its owning pack.
	BlockPacks []string
}

// PlanResult is the desired state computed from config + packs, plus any
// constraint violations that must block Apply.
type PlanResult struct {
	Config     *config.Config
	Packs      []lockfile.LockPack
	PackObjs   []*pack.Pack
	Artifacts  []Artifact
	Violations []render.Violation
}

var allTargets = []string{
	render.TargetClaude, render.TargetAgents, render.TargetGemini,
	render.TargetGovernance, render.TargetSkills, render.TargetMCP,
}

// CacheDir resolves the pack cache location (ESC_CACHE_DIR overrides).
func CacheDir() (string, error) {
	if d := os.Getenv("ESC_CACHE_DIR"); d != "" {
		return d, nil
	}
	return source.DefaultCacheDir()
}

// Plan fetches and verifies all pinned packs and computes the desired state.
// It never writes to the governed repo.
func Plan(ctx context.Context, root string) (*PlanResult, error) {
	cfg, err := config.Load(root)
	if err != nil {
		return nil, err
	}
	return planFromConfig(ctx, root, cfg)
}

func planFromConfig(ctx context.Context, root string, cfg *config.Config) (*PlanResult, error) {
	lock, err := lockfile.Load(root)
	if err != nil {
		return nil, err
	}
	cacheDir, err := CacheDir()
	if err != nil {
		return nil, err
	}

	res := &PlanResult{Config: cfg}
	for _, ref := range cfg.Packs {
		fetched, err := source.Fetch(ctx, ref, cacheDir, root)
		if err != nil {
			return nil, err
		}
		if err := verifyTrust(ctx, cfg, ref, fetched, root); err != nil {
			return nil, err
		}
		hash, err := pack.DirHash(fetched.Dir)
		if err != nil {
			return nil, err
		}
		// Lock integrity: same source+ref must resolve to same content.
		// Applies to git sources only — local plain dirs are unversioned
		// development mode, and their edits surface as ordinary drift.
		if locked := lock.Pack(ref.Source, ref.Ref); locked != nil && locked.Commit != "" && fetched.Commit != "" {
			if locked.Hash != hash || locked.Commit != fetched.Commit {
				return nil, fmt.Errorf("%w: pack %s@%s content changed since lock (locked %s, got %s) — a moved tag or tampered source; investigate before re-locking",
					esc.ErrLockMismatch, ref.Source, ref.Ref, locked.Hash, hash)
			}
		}
		p, err := pack.Load(fetched.Dir)
		if err != nil {
			return nil, err
		}
		if err := checkVersionRef(ref.Ref, p.Manifest.Version, ref.Source); err != nil {
			return nil, err
		}
		res.PackObjs = append(res.PackObjs, p)
		res.Packs = append(res.Packs, lockfile.LockPack{
			Source: ref.Source, Ref: ref.Ref, Commit: fetched.Commit, Hash: hash,
		})
	}

	// Custom targets (§2). Runs after the fetch/verify/load loop above — parsed
	// from verified pack content only, never during fetch — and before any
	// artifact is composed or written.
	customByName, allDeclared, err := collectCustomTargets(res.PackObjs)
	if err != nil {
		return nil, err
	}

	// Repo targets filter (§3): filtering a custom target out by name is a
	// complete opt-out — a filtered-out target must never require
	// acknowledgment. Selection is resolved before the acknowledgment gate
	// runs, not after.
	targets := cfg.Targets
	selectedCustom := map[string]bool{}
	if len(targets) == 0 {
		for name := range customByName {
			selectedCustom[name] = true
		}
	} else {
		for _, t := range targets {
			if !isBuiltInTarget(t) && !allDeclared[strings.ToLower(t)] {
				return nil, fmt.Errorf("config: unknown target %q", t)
			}
			if allDeclared[strings.ToLower(t)] {
				selectedCustom[strings.ToLower(t)] = true
			}
		}
	}

	ack := map[string]bool{}
	for _, f := range cfg.AllowCustomTargetFiles {
		ack[strings.ToLower(f)] = true
	}
	// Acknowledgment gate (§2.3): a selected (not filtered out) custom target
	// renders only if its file is acknowledged. Unacknowledged targets are a
	// fail-closed Violation (blocks Apply, reported by esc status) — not a
	// silent skip. Iterated in sorted name order for deterministic Violation
	// output when multiple targets are unacknowledged.
	rendered := map[string]custom{} // name -> target to render
	for _, name := range sortedNames(customByName) {
		if !selectedCustom[name] {
			continue // filtered out by config targets: no acknowledgment required
		}
		c := customByName[name]
		if ack[strings.ToLower(c.file)] {
			rendered[name] = c
			continue
		}
		res.Violations = append(res.Violations, render.Violation{
			Path: c.file,
			Rule: fmt.Sprintf("custom target %q from pack %s is not acknowledged; add this line to allow_custom_target_files in %s:\n  - %s",
				name, c.owner.Manifest.Name, config.Path(root), c.file),
		})
	}

	if len(targets) == 0 {
		targets = append(append([]string{}, allTargets...), sortedNames(rendered)...)
	}

	for _, t := range targets {
		if c, ok := rendered[t]; ok {
			body := render.ComposeCustom(c.owner, c.name)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: c.file, Kind: KindBlock, Hash: render.BodyHash(body), Body: body,
				BlockPacks: render.PackLabels([]*pack.Pack{c.owner}),
			})
			continue
		}
		if allDeclared[strings.ToLower(t)] {
			continue // declared custom target that is not acknowledged: Violation already recorded
		}
		switch t {
		case render.TargetClaude, render.TargetAgents, render.TargetGemini:
			body := render.Compose(res.PackObjs, t)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: render.TargetFile[t], Kind: KindBlock, Hash: render.BodyHash(body), Body: body,
				BlockPacks: render.PackLabels(res.PackObjs),
			})
		case render.TargetGovernance:
			content := render.Governance(res.PackObjs)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: render.TargetFile[t], Kind: KindFile, Hash: esc.HashBytes([]byte(content)), Body: content,
			})
		case render.TargetSkills:
			for _, p := range res.PackObjs {
				for _, rel := range p.Manifest.Skills {
					src := filepath.Join(p.Dir, filepath.FromSlash(rel))
					h, err := pack.DirHash(src)
					if err != nil {
						return nil, err
					}
					name := "esc-" + p.Manifest.Name + "-" + filepath.Base(rel)
					res.Artifacts = append(res.Artifacts, Artifact{
						Path: filepath.ToSlash(filepath.Join(".claude", "skills", name)),
						Kind: KindDir, Hash: h, SrcDir: src,
					})
				}
			}
		case render.TargetMCP:
			servers := map[string]map[string]any{}
			for _, p := range res.PackObjs {
				for k, v := range p.Manifest.MCP.Servers {
					servers[k] = v
				}
			}
			if len(servers) > 0 {
				h, err := render.DesiredMCPHash(servers)
				if err != nil {
					return nil, err
				}
				keys := make([]string, 0, len(servers))
				for k := range servers {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				res.Artifacts = append(res.Artifacts, Artifact{
					Path: ".mcp.json", Kind: KindJSONKeys, Hash: h, Keys: keys, Servers: servers,
				})
			}
		default:
			return nil, fmt.Errorf("config: unknown target %q", t)
		}
	}

	// Constraint validation against everything that will land on disk —
	// merged agent files, skill file contents, and owned MCP entries. Skills
	// and MCP configs are the highest-risk payloads; they don't get a pass.
	var cs []pack.Constraints
	for _, p := range res.PackObjs {
		cs = append(cs, p.Manifest.Constraints)
	}
	for _, a := range res.Artifacts {
		switch a.Kind {
		case KindBlock, KindFile:
			merged, err := prospectiveContent(root, a)
			if err != nil {
				return nil, err
			}
			res.Violations = append(res.Violations, render.Validate(a.Path, merged, cs)...)
		case KindDir:
			err := filepath.WalkDir(a.SrcDir, func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(a.SrcDir, path)
				label := a.Path + "/" + filepath.ToSlash(rel)
				res.Violations = append(res.Violations, render.Validate(label, content, cs)...)
				return nil
			})
			if err != nil {
				return nil, err
			}
		case KindJSONKeys:
			canon, err := render.CanonicalServers(a.Servers)
			if err != nil {
				return nil, err
			}
			res.Violations = append(res.Violations, render.Validate(a.Path, canon, cs)...)
		}
	}
	return res, nil
}

// verifyTrust enforces the signing policy for one source.
func verifyTrust(ctx context.Context, cfg *config.Config, ref config.PackRef, fetched *source.FetchResult, root string) error {
	if ref.Trust == "unsigned" {
		return nil
	}
	if fetched.RepoDir == "" { // plain local dir: nothing to verify
		return fmt.Errorf("%w: local pack %s cannot be signature-verified; set `trust: unsigned` explicitly to accept it", esc.ErrSignature, ref.Source)
	}
	signers := cfg.AllowedSignersFile
	if signers != "" && !filepath.IsAbs(signers) {
		signers = filepath.Join(root, signers)
	}
	return source.VerifyRef(ctx, fetched.RepoDir, ref.Ref, signers)
}

// checkVersionRef enforces tag↔manifest version agreement for version tags.
func checkVersionRef(ref, version, src string) error {
	if ref == "" || ref == version || ref == "v"+version {
		return nil
	}
	if strings.HasPrefix(ref, "v") && strings.Contains(ref, ".") {
		return fmt.Errorf("%w: source %s: ref %s does not match manifest version %s", esc.ErrManifest, src, ref, version)
	}
	return nil // branch or SHA refs are exempt
}

// prospectiveContent computes what an artifact's file would contain after
// apply, without writing.
func prospectiveContent(root string, a Artifact) ([]byte, error) {
	existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Path)))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	switch a.Kind {
	case KindBlock:
		return render.Splice(existing, a.Body, render.BlockMeta{Packs: a.BlockPacks})
	case KindFile:
		return []byte(a.Body), nil
	}
	return existing, nil
}

// custom is one acknowledged/declared pack custom target during planning.
type custom struct {
	owner *pack.Pack
	name  string
	file  string
}

// collectCustomTargets validates every pack's custom targets (§2.1) and
// enforces cross-pack single-owner collisions (§2.2). It returns the declared
// targets keyed by name and a lower-cased declared-name set (used to validate
// the config targets filter). All failures are constraint errors (exit 1).
func collectCustomTargets(packs []*pack.Pack) (map[string]custom, map[string]bool, error) {
	byName := map[string]custom{}
	declared := map[string]bool{}
	seenName := map[string]string{} // lower name -> owning pack
	seenFile := map[string]string{} // lower file -> owning pack
	for _, p := range packs {
		for _, ct := range p.Manifest.CustomTargets {
			if err := targets.ValidateCustom(ct.Name, ct.File); err != nil {
				return nil, nil, fmt.Errorf("%w: pack %s: custom target %q: %v", esc.ErrConstraint, p.Manifest.Name, ct.Name, err)
			}
			ln, lf := strings.ToLower(ct.Name), strings.ToLower(ct.File)
			if other, ok := seenName[ln]; ok {
				return nil, nil, fmt.Errorf("%w: custom target name %q is declared by both %s and %s; a custom target belongs to exactly one pack", esc.ErrConstraint, ct.Name, other, p.Manifest.Name)
			}
			if other, ok := seenFile[lf]; ok {
				return nil, nil, fmt.Errorf("%w: custom target file %q is declared by both %s and %s; a custom target belongs to exactly one pack", esc.ErrConstraint, ct.File, other, p.Manifest.Name)
			}
			seenName[ln], seenFile[lf] = p.Manifest.Name, p.Manifest.Name
			byName[ct.Name] = custom{owner: p, name: ct.Name, file: ct.File}
			declared[ln] = true
		}
	}
	return byName, declared, nil
}

func isBuiltInTarget(name string) bool {
	for _, t := range allTargets {
		if t == name {
			return true
		}
	}
	return false
}

// sortedNames returns the map keys sorted, for deterministic target ordering.
func sortedNames(m map[string]custom) []string {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
