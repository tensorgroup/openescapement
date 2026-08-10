package pack

import (
	"fmt"
	"path"

	"gopkg.in/yaml.v3"
)

// SkillEntry is one skills: entry in pack.yaml — a union type. A plain
// string entry ("skills/vault-usage") keeps today's on-disk naming exactly,
// prefix included, so existing packs and lockfiles see no change. An object
// entry ({path: skills/brainstorming, name: brainstorming}) sets the on-disk
// directory name explicitly, which is how a vendored suite whose skills
// reference each other by canonical name survives syncing (spec §3).
type SkillEntry struct {
	Path string // pack-relative skill directory (slash-separated)
	Name string // explicit on-disk name; "" = legacy esc-<pack>-<base> prefix
}

// DirName resolves the on-disk directory name under .claude/skills/.
func (s SkillEntry) DirName(packName string) string {
	if s.Name != "" {
		return s.Name
	}
	return "esc-" + packName + "-" + path.Base(s.Path)
}

// UnmarshalYAML accepts the union: a scalar (legacy) or a {path, name}
// mapping. Unknown mapping keys are rejected by hand because a custom
// unmarshaller bypasses the decoder's KnownFields enforcement.
func (s *SkillEntry) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		return value.Decode(&s.Path)
	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			key := value.Content[i].Value
			switch key {
			case "path":
				if err := value.Content[i+1].Decode(&s.Path); err != nil {
					return err
				}
			case "name":
				if err := value.Content[i+1].Decode(&s.Name); err != nil {
					return err
				}
			default:
				return fmt.Errorf("skill entry: unknown key %q (want path, name)", key)
			}
		}
		if s.Path == "" {
			return fmt.Errorf("skill entry: path is required")
		}
		return nil
	default:
		return fmt.Errorf("skill entry: must be a string or a {path, name} mapping")
	}
}

// MarshalYAML emits the narrowest form that round-trips: a plain string for
// legacy entries, a mapping when a name override is present. A {path} object
// with no name is semantically identical to the plain string and marshals as
// one.
func (s SkillEntry) MarshalYAML() (any, error) {
	if s.Name == "" {
		return s.Path, nil
	}
	return struct {
		Path string `yaml:"path"`
		Name string `yaml:"name"`
	}{s.Path, s.Name}, nil
}
