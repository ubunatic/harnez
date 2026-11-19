package gitflow

import (
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/disable"
)

// TestGuardPushPolicy covers the push-to-main policy (issue 1206): only a direct
// push to the default branch is refused, and only when git.protected_main is set.
func TestGuardPushPolicy(t *testing.T) {
	const trunkYAML = "git:\n  mode: team\n  protected_main: true\n  default_branch: trunk\n"
	for _, tc := range []struct {
		name, yaml, ref string
		refused         bool
	}{
		{name: "solo allows main", ref: "refs/heads/main"},
		{name: "unprotected team allows main", yaml: "git:\n  mode: team\n  protected_main: false\n", ref: "refs/heads/main"},
		{name: "protected refuses main", yaml: teamProtectedYAML, ref: "refs/heads/main", refused: true},
		{name: "protected allows AGit main", yaml: teamProtectedYAML, ref: "refs/for/main/topic"},
		{name: "protected allows topic branch", yaml: teamProtectedYAML, ref: "refs/heads/feature"},
		{name: "protected allows tag", yaml: teamProtectedYAML, ref: "refs/tags/v1.0.0"},
		{name: "protected refuses configured default", yaml: trunkYAML, ref: "refs/heads/trunk", refused: true},
		{name: "protected allows main when default is trunk", yaml: trunkYAML, ref: "refs/heads/main"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, stderr := guardRepo(t, "main", tc.yaml)
			err := GuardPush(dir, tc.ref)
			if !tc.refused {
				if err != nil {
					t.Fatalf("GuardPush(%s) = %v, want nil", tc.ref, err)
				}
				if stderr.Len() != 0 {
					t.Fatalf("allowed push must not warn, got %q", stderr)
				}
				return
			}
			if err == nil {
				t.Fatalf("GuardPush(%s) = nil, want refusal", tc.ref)
			}
			for _, want := range []string{"git.protected_main", "refusing a direct push to " + tc.ref, "refs/for/"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}

// TestGuardPushDisableOverride checks that /disable repo-rules push-main unlocks a
// direct push only for the session that recorded it, the same as for commits.
func TestGuardPushDisableOverride(t *testing.T) {
	spec, err := disable.LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, current, recorded string
		allowed                 bool
	}{
		{name: "no suspension", current: "owner"},
		{name: "stale session", current: "worker", recorded: "owner"},
		{name: "matching session", current: "owner", recorded: "owner", allowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, stderr := guardRepo(t, "main", teamProtectedYAML)
			for _, env := range spec.SessionEnv {
				t.Setenv(env, "")
			}
			t.Setenv(spec.SessionEnv[0], tc.current)
			if tc.recorded != "" {
				if err := spec.Write(dir, disable.State{Class: "repo-rules", Item: "push-main", SessionID: tc.recorded}); err != nil {
					t.Fatal(err)
				}
			}
			err := GuardPush(dir, "refs/heads/main")
			if !tc.allowed {
				if err == nil {
					t.Fatal("GuardPush = nil, want refusal")
				}
				return
			}
			if err != nil {
				t.Fatalf("GuardPush with matching suspension = %v, want nil", err)
			}
			if !strings.Contains(stderr.String(), "unlocked by /disable repo-rules push-main") {
				t.Errorf("stderr = %q, want the override noted", stderr)
			}
		})
	}
}
