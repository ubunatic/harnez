package subagent

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/modelcatalog"
)

func TestPolicyLoadErrorPath(t *testing.T) {
	if os.Getenv("TEST_983_CHILD") == "1" {
		_, err := ResolveModelForRole("developer", "smart")
		if err == nil || !strings.Contains(err.Error(), "models.yaml") || !strings.Contains(err.Error(), "developer") || !strings.Contains(err.Error(), "cycle") {
			t.Fatalf("error %v", err)
		}
		return
	}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "harnez"), 0700); err != nil {
		t.Fatal(err)
	}
	data := "version: 1\ndefault_enabled: true\nenabled: []\nmodels: {}\nroles: {developer: {models: {smart: [tui]}}}\n"
	if err := os.WriteFile(filepath.Join(dir, "harnez", "models.yaml"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestPolicyLoadErrorPath$")
	cmd.Env = append(os.Environ(), "TEST_983_CHILD=1", "XDG_CONFIG_HOME="+dir, "XDG_STATE_HOME="+t.TempDir())
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("output=%s err=%v", out, err)
	}
}

func TestMergeRolePolicy(t *testing.T) {
	base, err := agentSpecOnce()
	if err != nil {
		t.Fatal(err)
	}
	aliases, err := modelAliasMap()
	if err != nil {
		t.Fatal(err)
	}
	policy := &modelcatalog.Policy{Roles: map[string]modelcatalog.RolePolicy{"developer": {Models: map[string][]string{"smart": {"opus:med"}, "custom": {"luna:med"}, "ai": {}}}}, ModelTraits: map[string]string{"custom": "Custom work"}}
	got, err := mergeRolePolicy(base, policy, aliases, "/tmp/models.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Roles["developer"].Models["smart"], []string{"opus:med"}) || len(got.Roles["developer"].Models["ai"]) != 0 || !reflect.DeepEqual(got.Roles["developer"].Models["custom"], []string{"luna:med"}) || len(got.Roles["developer"].Models["lean"]) == 0 {
		t.Fatal("replacement, unset, new trait or inheritance lost")
	}
	if len(base.Roles["developer"].Models["smart"]) != 2 || got.ModelTraits["custom"] != "Custom work" || got.ModelTraits["smart"] != base.ModelTraits["smart"] {
		t.Fatal("base or descriptions changed")
	}
	selection, err := resolveRoleSelection(got, "developer", "smart", func(target string) (Model, error) { return resolveModelIn(aliases, target) }, func(Model) string { return "" })
	if err != nil || selection.Target != "opus:med" {
		t.Fatalf("selection=%+v err=%v", selection, err)
	}
	picks := modelPicksIn(got, func(role, trait string) (Model, error) {
		result, err := resolveRoleSelection(got, role, trait, func(target string) (Model, error) { return resolveModelIn(aliases, target) }, func(Model) string { return "" })
		return result.Model, err
	})
	found := false
	for _, pick := range picks {
		if pick.Role == "developer" && pick.Trait == "smart" {
			found = reflect.DeepEqual(pick.Chain, []string{"opus:med"}) && strings.Contains(pick.Resolved, "opus")
		}
	}
	if !found {
		t.Fatalf("missing overridden picks: %+v", picks)
	}
	p2 := &modelcatalog.Policy{Roles: map[string]modelcatalog.RolePolicy{"reviewer": {Models: map[string][]string{"newtrait": {"luna:med"}}}}}
	if _, err := mergeRolePolicy(base, p2, aliases, "/tmp/models.yaml"); err != nil {
		t.Fatalf("new trait without description: %v", err)
	}
}

func TestMergeRolePolicyRejectsInvalid(t *testing.T) {
	base, err := agentSpecOnce()
	if err != nil {
		t.Fatal(err)
	}
	aliases, err := modelAliasMap()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, role, trait string
		values            []string
		want              string
	}{
		{"role", "unknown", "smart", []string{"sol:med"}, "unknown role"},
		{"cycle", "developer", "smart", []string{"tui"}, "cycle"},
		{"unknown", "developer", "smart", []string{"missing-model"}, "target"},
		{"tier", "developer", "low", []string{"sol:med"}, "invalid trait"},
		{"collision", "developer", "sol", []string{"sol:med"}, "collides"},
		{"duplicate", "developer", "smart", []string{"sol:med", "sol:med"}, "duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &modelcatalog.Policy{Roles: map[string]modelcatalog.RolePolicy{tc.role: {Models: map[string][]string{tc.trait: tc.values}}}}
			_, err := mergeRolePolicy(base, p, aliases, "/tmp/models.yaml")
			if err == nil || !strings.Contains(err.Error(), "/tmp/models.yaml") || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), tc.role) {
				t.Fatalf("error: %v", err)
			}
		})
	}
	p := &modelcatalog.Policy{Roles: map[string]modelcatalog.RolePolicy{"developer": {Models: map[string][]string{"lean": {"std"}, "std": {"smart"}, "smart": {"tui"}, "tui": {"cheap"}}}}}
	if _, err := mergeRolePolicy(base, p, aliases, "/tmp/models.yaml"); err == nil || !strings.Contains(err.Error(), "depth exceeds 4") || !strings.Contains(err.Error(), "developer") {
		t.Fatalf("depth error: %v", err)
	}
}

func TestPolicyRoleAliasNamesCanonical(t *testing.T) {
	base, err := agentSpecOnce()
	if err != nil {
		t.Fatal(err)
	}
	aliases, err := modelAliasMap()
	if err != nil {
		t.Fatal(err)
	}
	_, err = mergeRolePolicy(base, &modelcatalog.Policy{Roles: map[string]modelcatalog.RolePolicy{"dev": {Models: map[string][]string{"smart": {"sol:med"}}}}}, aliases, "/tmp/models.yaml")
	if err == nil || !strings.Contains(err.Error(), "/tmp/models.yaml") || !strings.Contains(err.Error(), "dev") || !strings.Contains(err.Error(), "developer") {
		t.Fatalf("error=%v", err)
	}
}
