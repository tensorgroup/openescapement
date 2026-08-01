// Package targets owns instruction-file target metadata: the compiled-in
// built-in table and validation for pack-defined custom targets. It is the
// single source of truth shared by the CLI and (in Plan 2) the portal, and
// imports only the standard library.
package targets

import (
	"fmt"
	"path"
	"regexp"
	"strings"
)

// Built-in target names.
const (
	NameClaude     = "claude"
	NameAgents     = "agents"
	NameGemini     = "gemini"
	NameGovernance = "governance"
	NameSkills     = "skills"
	NameMCP        = "mcp"
)

// Target kinds.
const (
	KindManagedBlock = "managed-block"
	KindWholeFile    = "whole-file"
	KindSkillsDir    = "skills-dir"
	KindMCPConfig    = "mcp-config"
)

// Info is one target's metadata. For Plan 1 only Name/File/Kind/BuiltIn/
// OwnerPack are populated for built-ins; Readers/DocURLs/Description/Suggestion
// exist for Plan 2 (the portal metadata table) and stay empty here.
type Info struct {
	Name        string   // "claude"
	File        string   // "CLAUDE.md" (empty for skills/mcp)
	Kind        string   // one of the Kind* constants
	Readers     []string // agent tools that read it (Plan 2)
	DocURLs     []string // official vendor documentation (Plan 2)
	Description string   // one paragraph (Plan 2)
	Suggestion  string   // esc's guidance (Plan 2)
	BuiltIn     bool
	OwnerPack   string // defining pack name; empty for built-ins
}

var builtIns = []Info{
	{Name: NameClaude, File: "CLAUDE.md", Kind: KindManagedBlock, BuiltIn: true},
	{Name: NameAgents, File: "AGENTS.md", Kind: KindManagedBlock, BuiltIn: true},
	{Name: NameGemini, File: "GEMINI.md", Kind: KindManagedBlock, BuiltIn: true},
	{Name: NameGovernance, File: "GOVERNANCE.md", Kind: KindWholeFile, BuiltIn: true},
	{Name: NameSkills, File: "", Kind: KindSkillsDir, BuiltIn: true},
	{Name: NameMCP, File: "", Kind: KindMCPConfig, BuiltIn: true},
}

// BuiltIns returns a copy of the compiled-in target table.
func BuiltIns() []Info {
	out := make([]Info, len(builtIns))
	copy(out, builtIns)
	return out
}

// ByName returns the built-in with the given name.
func ByName(name string) (Info, bool) {
	for _, in := range builtIns {
		if in.Name == name {
			return in, true
		}
	}
	return Info{}, false
}

// IsBuiltInName reports whether name is a built-in target name.
func IsBuiltInName(name string) bool {
	_, ok := ByName(name)
	return ok
}

// IsFragmentTarget reports whether name is a built-in target a fragment may
// name in front-matter (the markdown targets: claude, agents, gemini,
// governance; not skills/mcp).
func IsFragmentTarget(name string) bool {
	in, ok := ByName(name)
	return ok && (in.Kind == KindManagedBlock || in.Kind == KindWholeFile)
}

var customName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// fileChars matches the whole file string when every rune is in the allowed
// ASCII set. A single disallowed rune (including any non-ASCII, backslash, or
// space) makes it fail.
var fileChars = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// controlDirs are first path segments a custom target file may not use. The
// carve-out is .github/*.md (two segments), which is not in this set;
// .github/workflows/*.md is three segments and rejected by the depth rule.
var controlDirs = map[string]bool{
	".git": true, ".escapement": true, ".claude": true, ".gemini": true,
	".cursor": true, ".codex": true, ".agent": true, ".agents": true,
	".vscode": true, ".idea": true,
}

// ValidateCustom enforces every §2.1 name and file rule. It returns a plain
// descriptive error (no sentinel); the caller wraps it with esc.ErrConstraint,
// and the portal (Plan 2) surfaces the message verbatim. All comparisons are
// case-folded; NFC is a no-op because file is ASCII-only.
func ValidateCustom(name, file string) error {
	if !customName.MatchString(name) {
		return fmt.Errorf("target name %q must match ^[a-z][a-z0-9-]{0,31}$", name)
	}
	if IsBuiltInName(strings.ToLower(name)) {
		return fmt.Errorf("target name %q collides with a built-in target", name)
	}

	if file == "" {
		return fmt.Errorf("target %q: file is required", name)
	}
	if !fileChars.MatchString(file) {
		return fmt.Errorf("target file %q: only ASCII [A-Za-z0-9._/-] is allowed (no spaces, backslashes, or non-ASCII)", file)
	}
	// Clean must be a no-op: no ., no .., no //, no trailing slash. path.Clean
	// operates on slash paths, which is the required separator here.
	if path.Clean(file) != file {
		return fmt.Errorf("target file %q must be a clean, relative, slash-separated path", file)
	}
	segs := strings.Split(file, "/")
	for _, s := range segs {
		if s == "" {
			return fmt.Errorf("target file %q must not contain empty path segments", file)
		}
		if s == ".." {
			return fmt.Errorf("target file %q must not contain %q segments", file, "..")
		}
		if strings.HasSuffix(s, ".") || strings.HasSuffix(s, " ") {
			return fmt.Errorf("target file %q: no path segment may end with a dot or space", file)
		}
	}
	if strings.HasPrefix(file, "/") {
		return fmt.Errorf("target file %q must be relative (no leading slash)", file)
	}
	if len(segs) > 2 {
		return fmt.Errorf("target file %q may have at most 2 path segments", file)
	}
	if !strings.HasSuffix(strings.ToLower(file), ".md") {
		return fmt.Errorf("target file %q must end in .md", file)
	}
	if controlDirs[strings.ToLower(segs[0])] {
		return fmt.Errorf("target file %q: %q is a control directory and may not hold custom targets", file, segs[0])
	}
	lf := strings.ToLower(file)
	for _, in := range builtIns {
		if in.File != "" && strings.ToLower(in.File) == lf {
			return fmt.Errorf("target file %q collides with built-in target file %q", file, in.File)
		}
	}
	return nil
}
