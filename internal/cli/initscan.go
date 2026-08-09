package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tensorgroup/openescapement/internal/render"
)

// Detected is one instruction file or directory already present in a repo
// before escapement was introduced.
type Detected struct {
	Target         string // a render.Target* constant
	Path           string // repo-relative, slash-separated
	Bytes, Lines   int
	HasBlock       bool // already carries a managed block
	HasPlaceholder bool // already carries the placement marker
}

// detectExisting reports the instruction files a repo already has. It never
// modifies anything. The result is sorted by Path and is empty, not nil, when
// nothing is found.
func detectExisting(root string) ([]Detected, error) {
	out := []Detected{}
	for target, name := range render.TargetFile {
		content, err := os.ReadFile(filepath.Join(root, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		d := Detected{
			Target:         target,
			Path:           name,
			Bytes:          len(content),
			Lines:          bytes.Count(content, []byte("\n")),
			HasPlaceholder: bytes.Contains(content, []byte(render.Placeholder)),
		}
		if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
			d.Lines++
		}
		// A corrupt block still counts as present: the point is to warn the
		// user, and `esc sync` will report the corruption properly.
		if b, err := render.Extract(content); err != nil || b != nil {
			d.HasBlock = true
		}
		out = append(out, d)
	}
	if info, err := os.Stat(filepath.Join(root, ".mcp.json")); err == nil && !info.IsDir() {
		out = append(out, Detected{Target: render.TargetMCP, Path: ".mcp.json", Bytes: int(info.Size())})
	}
	if entries, err := os.ReadDir(filepath.Join(root, ".claude", "skills")); err == nil && len(entries) > 0 {
		out = append(out, Detected{Target: render.TargetSkills, Path: ".claude/skills"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// targetOrder is the fixed presentation order for detected targets: the
// file-based targets in render.TargetFile's declaration order, then mcp,
// then skills. detectExisting's own result is sorted by Path for stable
// lookups; this order is what the user actually reads and what
// configTemplate's targets list is built from.
var targetOrder = []string{
	render.TargetClaude, render.TargetAgents, render.TargetGemini, render.TargetGovernance,
	render.TargetMCP, render.TargetSkills,
}

// orderDetected returns detected re-ordered per targetOrder. detectExisting
// never produces more than one Detected per target, so this is a reorder,
// not a merge.
func orderDetected(detected []Detected) []Detected {
	byTarget := make(map[string]Detected, len(detected))
	for _, d := range detected {
		byTarget[d.Target] = d
	}
	out := make([]Detected, 0, len(detected))
	for _, t := range targetOrder {
		if d, ok := byTarget[t]; ok {
			out = append(out, d)
		}
	}
	return out
}

// detectedTargets returns the config `targets:` values to pre-fill from
// detected, in targetOrder, deduplicated (detectExisting itself never
// duplicates a target, but this stays defensive since it feeds directly into
// the written config). Callers must pass a nil/empty result straight through
// to configTemplate rather than synthesizing a non-empty slice: empty means
// "all targets", the correct default for a repo where nothing was found.
func detectedTargets(detected []Detected) []string {
	ordered := orderDetected(detected)
	if len(ordered) == 0 {
		return nil
	}
	out := make([]string, len(ordered))
	for i, d := range ordered {
		out[i] = d.Target
	}
	return out
}

// isFileTarget reports whether target renders into one of the whole
// instruction files (CLAUDE.md, AGENTS.md, GEMINI.md, GOVERNANCE.md), as
// opposed to mcp or skills, which have no managed block and no placement
// hint.
func isFileTarget(target string) bool {
	_, ok := render.TargetFile[target]
	return ok
}

// detectedItemLabel is how one Detected reads in the "Found ..." summary
// line: file-based targets show a line count, mcp and skills just their path.
func detectedItemLabel(d Detected) string {
	if !isFileTarget(d.Target) {
		return d.Path
	}
	return fmt.Sprintf("%s (%d %s)", d.Path, d.Lines, plural(d.Lines, "line", "lines"))
}

// detectedSyncClause is the amendment-model description of what `esc sync`
// will do to one detected item. A file that already carries a block or a
// placeholder must not be told it will get one inserted at the top: that
// would promise something sync will not do. Promising an insert that never
// happens is worse than describing what is actually there.
func detectedSyncClause(d Detected) string {
	switch d.Target {
	case render.TargetMCP:
		return "add pack MCP servers alongside your existing ones"
	case render.TargetSkills:
		return "add pack skills alongside yours in .claude/skills"
	default:
		switch {
		case d.HasBlock:
			return "update the managed block already in " + d.Path
		case d.HasPlaceholder:
			return "insert a managed block at the placeholder already in " + d.Path
		default:
			return "insert a managed block at the top of " + d.Path
		}
	}
}

// joinList renders items as an English list: "a", "a and b", "a, b and c".
func joinList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
	}
}

// explainDetection prints, in the amendment model's vocabulary, what esc
// init found and what the first `esc sync` will do to it. Called only when
// detected is non-empty; the empty case keeps the original "Initialized ...
// Add pack sources" message unchanged.
func explainDetection(w io.Writer, detected []Detected) {
	ordered := orderDetected(detected)
	labels := make([]string, len(ordered))
	clauses := make([]string, len(ordered))
	needsPlacementHint := false
	for i, d := range ordered {
		labels[i] = detectedItemLabel(d)
		clauses[i] = detectedSyncClause(d)
		if isFileTarget(d.Target) && !d.HasBlock && !d.HasPlaceholder {
			needsPlacementHint = true
		}
	}
	fmt.Fprintf(w, "Found %s.\n\n", joinList(labels))
	fmt.Fprintf(w, "`esc sync` will %s. Your current content is preserved byte for byte and reported as a local amendment.\n",
		joinList(clauses))
	if needsPlacementHint {
		fmt.Fprintf(w, "\nTo place the block somewhere else, put %s where you want it before syncing.\n", render.Placeholder)
	}
}
