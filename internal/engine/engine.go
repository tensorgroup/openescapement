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
)

// Artifact is one desired output on disk.
type Artifact struct {
	Path string // relative to repo root
	Kind string // block | file | dir | json-keys
	Hash string // canonical hash of the managed content
	Keys []string
	// Body is the managed-block body (kind=block) or full content (kind=file).
	Body string
	// SrcDir is the pack directory to copy from (kind=dir).
	SrcDir string
	// Servers are the owned MCP entries (kind=json-keys).
	Servers map[string]map[string]any
}

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
		if locked := lock.Pack(ref.Source, ref.Ref); locked != nil {
			if locked.Hash != hash || (locked.Commit != "" && fetched.Commit != "" && locked.Commit != fetched.Commit) {
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

	targets := cfg.Targets
	if len(targets) == 0 {
		targets = allTargets
	}
	for _, t := range targets {
		switch t {
		case render.TargetClaude, render.TargetAgents, render.TargetGemini:
			body := render.Compose(res.PackObjs, t)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: render.TargetFile[t], Kind: "block", Hash: render.BodyHash(body), Body: body,
			})
		case render.TargetGovernance:
			content := render.Governance(res.PackObjs)
			res.Artifacts = append(res.Artifacts, Artifact{
				Path: render.TargetFile[t], Kind: "file", Hash: esc.HashBytes([]byte(content)), Body: content,
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
						Kind: "dir", Hash: h, SrcDir: src,
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
					Path: ".mcp.json", Kind: "json-keys", Hash: h, Keys: keys, Servers: servers,
				})
			}
		default:
			return nil, fmt.Errorf("config: unknown target %q", t)
		}
	}

	// Constraint validation against prospective merged files.
	var cs []pack.Constraints
	for _, p := range res.PackObjs {
		cs = append(cs, p.Manifest.Constraints)
	}
	for _, a := range res.Artifacts {
		if a.Kind != "block" && a.Kind != "file" {
			continue
		}
		merged, err := prospectiveContent(root, a, res.PackObjs)
		if err != nil {
			return nil, err
		}
		res.Violations = append(res.Violations, render.Validate(a.Path, merged, cs)...)
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
func prospectiveContent(root string, a Artifact, packs []*pack.Pack) ([]byte, error) {
	existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(a.Path)))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	switch a.Kind {
	case "block":
		return render.Splice(existing, a.Body, render.BlockMeta{Packs: render.PackLabels(packs)})
	case "file":
		return []byte(a.Body), nil
	}
	return existing, nil
}

