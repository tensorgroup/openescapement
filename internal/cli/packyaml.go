package cli

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/tensorgroup/openescapement/internal/pack"
)

// appendSkillEntries appends object-form skills: entries to a pack.yaml. A
// structural yaml.Node round trip, not a struct re-marshal: re-marshalling a
// hand-written manifest through Manifest would reorder keys and drop every
// comment. This preserves far more than that — key ordering and every
// comment survive, and the appended entries are the only content-level
// change — but it is NOT byte-preserving outside the skills sequence, and
// the doc comment used to claim it was. yaml.v3's own encoder drops blank
// lines between top-level keys and reflows a folded (">") scalar onto one
// line, in both cases changing bytes this operation never intended to
// touch. None of that changes what the document MEANS: it stays valid
// pack.yaml, every comment and key stays attached to the same key, and the
// rewritten file parses back to content identical to the original except
// for the appended entries. That reparseable-and-semantically-identical
// guarantee is what packyaml_test.go's file-shape cases assert — not byte
// identity, which this function does not provide. The write itself is
// atomic and preserves the file's permission bits (restoreFile, cli.go).
func appendSkillEntries(path string, entries []pack.SkillEntry) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("%s: not a YAML mapping", path)
	}
	root := doc.Content[0]
	var seq *yaml.Node
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "skills" {
			seq = root.Content[i+1]
			break
		}
	}
	if seq == nil {
		seq = &yaml.Node{Kind: yaml.SequenceNode}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "skills"}, seq)
	}
	if seq.Kind != yaml.SequenceNode {
		return fmt.Errorf("%s: skills is not a list", path)
	}
	for _, e := range entries {
		entryNode := &yaml.Node{}
		if err := entryNode.Encode(e); err != nil {
			return err
		}
		seq.Content = append(seq.Content, entryNode)
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return restoreFile(path, buf.Bytes())
}
