package claude

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A legacy install of a resource-backed skill (docup) holds references/ next
// to SKILL.md. Adoption must accept harnez's own unmodified resources and keep
// refusing extra or edited files.
func TestSkillAdoptionStatusResources(t *testing.T) {
	resources := []skillResourceContent{{target: filepath.Join("references", "Guide.md"), data: []byte("shipped\n")}}
	setup := func(t *testing.T, files map[string]string) string {
		t.Helper()
		dir := t.TempDir()
		files["SKILL.md"] = "---\nname: docup\n---\nold body\n"
		for rel, body := range files {
			p := filepath.Join(dir, rel)
			if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	cases := []struct {
		name   string
		files  map[string]string
		adopt  bool
		reason string
	}{
		{"shipped resource", map[string]string{"references/Guide.md": "shipped\n"}, true, ""},
		{"only SKILL.md", map[string]string{}, true, ""},
		{"edited resource", map[string]string{"references/Guide.md": "local edit\n"}, false, "differs from the shipped version"},
		{"extra file", map[string]string{"references/Guide.md": "shipped\n", "references/Mine.md": "x"}, false, "beyond SKILL.md"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := setup(t, tc.files)
			adopt, reason := skillAdoptionStatus(dir, "docup", resources)
			if adopt != tc.adopt || !strings.Contains(reason, tc.reason) {
				t.Errorf("adopt=%v reason=%q, want adopt=%v reason containing %q", adopt, reason, tc.adopt, tc.reason)
			}
		})
	}
}
