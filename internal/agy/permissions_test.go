package agy

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"ubunatic.com/harnez/internal/jsonc"
)

func TestTranslateClaudeRule(t *testing.T) {
	cases := []struct {
		rule, want string
		ok         bool
	}{
		{"Bash(ls *)", "command(ls)", true},
		{"Bash(git:*)", "command(git)", true},
		{"Bash(sudo tune2fs -l *)", "command(sudo tune2fs -l)", true},
		{"Bash(ssh -V)", "command(ssh -V)", true},
		{"Bash(git * --help)", "", false},
		{"Read(~/projects/**)", "read_file(/h/projects)", true},
		{"Read(~/.bashrc)", "read_file(/h/.bashrc)", true},
		{"Read(//etc/**)", "read_file(/etc)", true},
		{"Read(src/**)", "", false},
		{"Read(~/x/*.go)", "", false},
		{"Edit(~/projects/**)", "write_file(/h/projects)", true},
		{"mcp__harnez__*", "", false},
		{"WebFetch(domain:example.com)", "", false},
	}
	for _, c := range cases {
		got, ok := TranslateClaudeRule(c.rule, "/h")
		if got != c.want || ok != c.ok {
			t.Errorf("TranslateClaudeRule(%q) = %q, %v; want %q, %v", c.rule, got, ok, c.want, c.ok)
		}
	}
}

func TestTranslateRulesAddsExtrasAndReportsUnmapped(t *testing.T) {
	extra := PermissionsSpec{
		ToolPermission: "always-proceed",
		Allow:          []string{"command(cat)", "command(ls)", "write_file(~/projects)"},
	}
	r, unmapped := TranslateRules(
		[]string{"Bash(ls *)", "mcp__harnez__*"}, []string{"Bash(git push *)"}, extra, "/h")
	wantAllow := []string{"command(ls)", "command(cat)", "write_file(/h/projects)"}
	if !slices.Equal(r.Allow, wantAllow) {
		t.Errorf("allow = %v, want %v", r.Allow, wantAllow)
	}
	if !slices.Equal(r.Deny, []string{"command(git push)"}) {
		t.Errorf("deny = %v", r.Deny)
	}
	if r.ToolPermission != "always-proceed" {
		t.Errorf("toolPermission = %q", r.ToolPermission)
	}
	if !slices.Equal(unmapped, []string{"mcp__harnez__*"}) {
		t.Errorf("unmapped = %v", unmapped)
	}
}

func TestEmbeddedSpecPermissions(t *testing.T) {
	p := MustLoadSpec().Permissions
	if !slices.Contains(p.Allow, "command(cat)") {
		t.Error("spec/agy_tools.yaml permissions.allow must contain command(cat)")
	}
	if p.ToolPermission != "always-proceed" {
		t.Errorf("spec tool_permission = %q, want always-proceed", p.ToolPermission)
	}
}

func TestApplyPermissionsMergesAndKeepsUserSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	orig := `{"model":"x","permissions":{"allow":["command(mine)"]},"trustedWorkspaces":["/w"]}`
	if err := os.WriteFile(path, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	r := Rules{
		Allow:          []string{"command(ls)", "command(cat)"},
		Deny:           []string{"command(git push)"},
		ToolPermission: "always-proceed",
	}
	changed, err := ApplyPermissions(path, r)
	if err != nil || !changed {
		t.Fatalf("first apply: changed=%v err=%v", changed, err)
	}
	doc := jsonc.Read(path)
	if doc["model"] != "x" || doc["trustedWorkspaces"] == nil {
		t.Errorf("user keys lost: %v", doc)
	}
	if doc["toolPermission"] != "always-proceed" {
		t.Errorf("toolPermission = %v", doc["toolPermission"])
	}
	perm := doc["permissions"].(map[string]any)
	if got := stringList(perm["allow"]); !slices.Equal(got, []string{"command(mine)", "command(ls)", "command(cat)"}) {
		t.Errorf("allow = %v", got)
	}
	if got := stringList(perm["deny"]); !slices.Equal(got, r.Deny) {
		t.Errorf("deny = %v", got)
	}
	if missing := PermissionsStatus(path, r); missing != 0 {
		t.Errorf("status missing = %d, want 0", missing)
	}
	if changed, err := ApplyPermissions(path, r); err != nil || changed {
		t.Errorf("second apply must be a no-op: changed=%v err=%v", changed, err)
	}

	if changed, err := RemovePermissions(path, r); err != nil || !changed {
		t.Fatalf("remove: changed=%v err=%v", changed, err)
	}
	doc = jsonc.Read(path)
	perm = doc["permissions"].(map[string]any)
	if got := stringList(perm["allow"]); !slices.Equal(got, []string{"command(mine)"}) {
		t.Errorf("after remove allow = %v, want user rule only", got)
	}
	if _, ok := perm["deny"]; ok {
		t.Error("empty deny list should be dropped")
	}
	if _, ok := doc["toolPermission"]; ok {
		t.Error("remove must drop harnez's toolPermission")
	}
}

func TestApplyPermissionsKeepsUserToolPermission(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"toolPermission":"ask"}`), 0644); err != nil {
		t.Fatal(err)
	}
	r := Rules{Allow: []string{"command(ls)"}, ToolPermission: "always-proceed"}
	if _, err := ApplyPermissions(path, r); err != nil {
		t.Fatal(err)
	}
	if got := jsonc.Read(path)["toolPermission"]; got != "ask" {
		t.Errorf("toolPermission = %v, want the user's ask", got)
	}
	if missing := PermissionsStatus(path, r); missing != 0 {
		t.Errorf("a user-chosen toolPermission is not missing: %d", missing)
	}
	if _, err := RemovePermissions(path, r); err != nil {
		t.Fatal(err)
	}
	if got := jsonc.Read(path)["toolPermission"]; got != "ask" {
		t.Errorf("remove must keep the user's toolPermission, got %v", got)
	}
}

func TestApplyPermissionsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	r := Rules{Allow: []string{"command(ls)"}}
	changed, err := ApplyPermissions(path, r)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if missing := PermissionsStatus(path, r); missing != 0 {
		t.Errorf("missing = %d", missing)
	}
	if _, ok := jsonc.Read(path)["toolPermission"]; ok {
		t.Error("empty ToolPermission must not write the key")
	}
}

func TestApplyPermissionsRejectsNonObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"permissions":"all"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPermissions(path, Rules{Allow: []string{"command(ls)"}}); err == nil {
		t.Fatal("expected error for non-object permissions")
	}
}
