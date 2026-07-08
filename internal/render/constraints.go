package render

import (
	"fmt"
	"regexp"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// Violation is one failed merged-file constraint.
type Violation struct {
	Path string
	Rule string
}

func (v Violation) String() string { return fmt.Sprintf("%s: %s", v.Path, v.Rule) }

// Validate checks the final merged file content at path against every pack's
// constraints. Patterns are case-insensitive; invalid patterns are reported
// as violations (a pack that can't be enforced must not pass silently).
func Validate(path string, merged []byte, cs []pack.Constraints) []Violation {
	var out []Violation
	for _, c := range cs {
		if c.MaxFileBytes > 0 && len(merged) > c.MaxFileBytes {
			out = append(out, Violation{path, fmt.Sprintf("file is %d bytes, exceeds max_file_bytes %d", len(merged), c.MaxFileBytes)})
		}
		for _, p := range c.ForbiddenPatterns {
			re, err := regexp.Compile("(?i)" + p)
			if err != nil {
				out = append(out, Violation{path, fmt.Sprintf("invalid forbidden_pattern %q: %v", p, err)})
				continue
			}
			if re.Match(merged) {
				out = append(out, Violation{path, fmt.Sprintf("matches forbidden_pattern %q", p)})
			}
		}
	}
	return out
}
