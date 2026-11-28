package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tensorgroup/openescapement/internal/esc"
	"github.com/tensorgroup/openescapement/internal/pack"
	"github.com/tensorgroup/openescapement/internal/render"
)

// ruleBodyShapes are awkward fragment bodies a rule-file write must survive.
// The "paths" case also proves the frontmatter fence lands at byte 0.
func ruleBodyShapes() map[string]pack.Fragment {
	return map[string]pack.Fragment{
		"no-trailing-newline": {Path: "rules/api.md", Targets: []string{"rules"}, Body: "line one\nline two"},
		"crlf":                {Path: "rules/api.md", Targets: []string{"rules"}, Body: "line one\r\nline two\r\n"},
		"empty":               {Path: "rules/api.md", Targets: []string{"rules"}, Body: ""},
		"paths":               {Path: "rules/api.md", Targets: []string{"rules"}, Paths: []string{"src/**"}, Body: "body\n"},
	}
}

func TestRuleFileWriteAdoptRemoveShapeMatrix(t *testing.T) {
	for name, frag := range ruleBodyShapes() {
		t.Run(name, func(t *testing.T) {
			content := render.RenderRule(frag, "acme")
			relPath := render.RuleFilePath("acme", frag)
			a := Artifact{Path: relPath, Kind: KindFile, Body: content, Hash: esc.HashBytes(render.NormalizeEndings([]byte(content)))}

			// (1) write path: apply writes it; classify reports InSync; the file
			// ends in a newline; a paths-scoped file has its fence at byte 0.
			root := t.TempDir()
			if _, err := Apply(root, &PlanResult{Artifacts: []Artifact{a}}, false); err != nil {
				t.Fatalf("apply write: %v", err)
			}
			if classify(root, a, mustLock(t, root)).State != InSync {
				t.Errorf("written rule file must be InSync")
			}
			got, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(relPath)))
			if len(got) == 0 || got[len(got)-1] != '\n' {
				t.Errorf("rule file must end in a newline: %q", got)
			}
			if len(frag.Paths) > 0 && !bytes.HasPrefix(got, []byte("---\n")) {
				t.Errorf("paths frontmatter must be at byte 0 after write: %q", got)
			}

			// (2) adoption: a REAL CRLF copy on disk with no lock entry is
			// adopted because the compare normalizes endings.
			root2 := t.TempDir()
			p := filepath.Join(root2, filepath.FromSlash(relPath))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(strings.ReplaceAll(content, "\n", "\r\n")), 0o644); err != nil {
				t.Fatal(err)
			}
			res, err := Apply(root2, &PlanResult{Artifacts: []Artifact{a}}, false)
			if err != nil {
				t.Fatalf("apply adopt: %v", err)
			}
			if len(res.Adopted) != 1 {
				t.Errorf("a CRLF copy of identical content must be adopted, got adopted=%v skipped=%+v", res.Adopted, res.Skipped)
			}

			// (3) removal: an orphaned rule file (no plan) whose on-disk copy is
			// CRLF still hashes equal, so it is removed, not declined.
			root3 := t.TempDir()
			if _, err := Apply(root3, &PlanResult{Artifacts: []Artifact{a}}, false); err != nil {
				t.Fatal(err)
			}
			dp := filepath.Join(root3, filepath.FromSlash(relPath))
			onDisk, _ := os.ReadFile(dp)
			if err := os.WriteFile(dp, bytes.ReplaceAll(onDisk, []byte("\n"), []byte("\r\n")), 0o644); err != nil {
				t.Fatal(err)
			}
			resR, err := Apply(root3, &PlanResult{}, false)
			if err != nil {
				t.Fatalf("apply remove: %v", err)
			}
			if len(resR.Skipped) != 0 {
				t.Errorf("unedited orphan rule file must not be declined: %+v", resR.Skipped)
			}
			if _, err := os.Stat(dp); !os.IsNotExist(err) {
				t.Errorf("orphan rule file must be removed")
			}
		})
	}
}
