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
	extra := PermissionsSpec{Allow: []string{"command(cat)", "command(ls)", "write_file(~/projects)"}}
	allow, deny, unmapped := TranslateRules(
		[]string{"Bash(ls *)", "mcp__harnez__*"}, []string{"Bash(git push *)"}, extra, "/h")
	wantAllow := []string{"command(ls)", "command(cat)", "write_file(/h/projects)"}
	if !slices.Equal(allow, wantAllow) {
		t.Errorf("allow = %v, want %v", allow, wantAllow)
	}
	if !slices.Equal(deny, []string{"command(git push)"}) {
		t.Errorf("deny = %v", deny)
	}
	if !slices.Equal(unmapped, []string{"mcp__harnez__*"}) {
		t.Errorf("unmapped = %v", unmapped)
	}
}

func TestEmbeddedSpecAllowsCat(t *testing.T) {
	if !slices.Contains(MustLoadSpec().Permissions.Allow, "command(cat)") {
		t.Fatal("spec/agy_tools.yaml permissions.allow must contain command(cat)")
	}
}

func TestApplyPermissionsMergesAndKeepsUserSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	orig := `{"model":"x","permissions":{"allow":["command(mine)"]},"trustedWorkspaces":["/w"]}`
	if err := os.WriteFile(path, []byte(orig), 0644); err != nil {
		t.Fatal(err)
	}
	allow, deny := []string{"command(ls)", "command(cat)"}, []string{"command(git push)"}
	changed, err := ApplyPermissions(path, allow, deny)
	if err != nil || !changed {
		t.Fatalf("first apply: changed=%v err=%v", changed, err)
	}
	doc := jsonc.Read(path)
	if doc["model"] != "x" || doc["trustedWorkspaces"] == nil {
		t.Errorf("user keys lost: %v", doc)
	}
	if _, ok := doc["toolPermission"]; ok {
		t.Error("apply must not set toolPermission")
	}
	perm := doc["permissions"].(map[string]any)
	if got := stringList(perm["allow"]); !slices.Equal(got, []string{"command(mine)", "command(ls)", "command(cat)"}) {
		t.Errorf("allow = %v", got)
	}
	if got := stringList(perm["deny"]); !slices.Equal(got, deny) {
		t.Errorf("deny = %v", got)
	}
	if missing := PermissionsStatus(path, allow, deny); missing != 0 {
		t.Errorf("status missing = %d, want 0", missing)
	}
	if changed, err := ApplyPermissions(path, allow, deny); err != nil || changed {
		t.Errorf("second apply must be a no-op: changed=%v err=%v", changed, err)
	}

	if changed, err := RemovePermissions(path, allow, deny); err != nil || !changed {
		t.Fatalf("remove: changed=%v err=%v", changed, err)
	}
	perm = jsonc.Read(path)["permissions"].(map[string]any)
	if got := stringList(perm["allow"]); !slices.Equal(got, []string{"command(mine)"}) {
		t.Errorf("after remove allow = %v, want user rule only", got)
	}
	if _, ok := perm["deny"]; ok {
		t.Error("empty deny list should be dropped")
	}
}

func TestApplyPermissionsEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	changed, err := ApplyPermissions(path, []string{"command(ls)"}, nil)
	if err != nil || !changed {
		t.Fatalf("changed=%v err=%v", changed, err)
	}
	if missing := PermissionsStatus(path, []string{"command(ls)"}, nil); missing != 0 {
		t.Errorf("missing = %d", missing)
	}
}

func TestApplyPermissionsRejectsNonObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"permissions":"all"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPermissions(path, []string{"command(ls)"}, nil); err == nil {
		t.Fatal("expected error for non-object permissions")
	}
}
