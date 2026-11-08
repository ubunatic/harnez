package disable

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func mustSpec(t *testing.T) Spec {
	t.Helper()
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestEmbeddedSpecMatchesSchema(t *testing.T) {
	data, err := harnez.DefaultFS.ReadFile("spec/schemas/disable.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	raw, err := harnez.DefaultFS.ReadFile("spec/disable.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]any
	if err := yaml.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	for _, key := range schema.Required {
		if _, ok := values[key]; !ok {
			t.Errorf("spec/disable.yaml lacks required %q", key)
		}
	}
	for key := range values {
		if _, ok := schema.Properties[key]; !ok {
			t.Errorf("spec/disable.yaml key %q is not in the schema", key)
		}
	}
	s := mustSpec(t)
	for _, want := range [][2]string{{"process", "quota-1"}, {"triggered-skill", "lean-sprint"}, {"repo-rules", "push-main"}} {
		if !s.Known(want[0], want[1]) {
			t.Errorf("Known(%s, %s) = false", want[0], want[1])
		}
	}
	if s.Known("repo-rules", "force-push") || s.Known("process", "../x") {
		t.Error("Known accepted an unlisted or unsafe item")
	}
}

func TestCheckAppliedStaleAndCleared(t *testing.T) {
	s := mustSpec(t)
	root := t.TempDir()
	if got := s.Check(root, "process", "quota-1", env(nil)); got.Applied || got.Stale {
		t.Fatalf("no state: got %+v", got)
	}
	st := State{Class: "process", Item: "quota-1", SetAt: time.Now(), SessionID: "s1"}
	if err := s.Write(root, st); err != nil {
		t.Fatal(err)
	}
	if got := s.Check(root, "process", "quota-1", env(map[string]string{"CLAUDE_CODE_SESSION_ID": "s1"})); !got.Applied {
		t.Errorf("same session: got %+v, want applied", got)
	}
	if got := s.Check(root, "process", "quota-1", env(map[string]string{"CLAUDE_CODE_SESSION_ID": "s2"})); got.Applied || !got.Stale {
		t.Errorf("other session: got %+v, want stale", got)
	}
	if got := s.Check(root, "process", "quota-1", env(nil)); got.Applied || !got.Stale {
		t.Errorf("no session id: got %+v, want stale", got)
	}
	list, err := s.List(root)
	if err != nil || len(list) != 1 {
		t.Fatalf("List = %v, %v", list, err)
	}
	if err := s.Remove(root, "process", "quota-1"); err != nil {
		t.Fatal(err)
	}
	if got := s.Check(root, "process", "quota-1", env(map[string]string{"CLAUDE_CODE_SESSION_ID": "s1"})); got.Applied || got.Stale {
		t.Errorf("after Remove: got %+v", got)
	}
	if err := s.Remove(root, "process", "quota-1"); err != nil {
		t.Errorf("second Remove: %v", err)
	}
}

func TestEmptyStateSessionIsNeverApplied(t *testing.T) {
	s := mustSpec(t)
	root := t.TempDir()
	if err := s.Write(root, State{Class: "process", Item: "quota-1", SetAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if got := s.Check(root, "process", "quota-1", env(nil)); got.Applied {
		t.Errorf("empty session ids matched: %+v", got)
	}
}

// A harnez agent worker inherits the host's CLAUDE_CODE_SESSION_ID but gets
// its own HARNEZ_SESSION_ID; the host's suspension must not apply to it.
func TestHostSuspensionDoesNotReachWorker(t *testing.T) {
	s := mustSpec(t)
	root := t.TempDir()
	if err := s.Write(root, State{Class: "process", Item: "quota-1", SetAt: time.Now(), SessionID: "host"}); err != nil {
		t.Fatal(err)
	}
	worker := env(map[string]string{"CLAUDE_CODE_SESSION_ID": "host", "HARNEZ_SESSION_ID": "worker-1"})
	if got := s.Check(root, "process", "quota-1", worker); got.Applied {
		t.Fatalf("worker saw the host suspension as applied: %+v", got)
	}
}

func TestRemoveAndCheckRejectUnsafeNames(t *testing.T) {
	s := mustSpec(t)
	root := t.TempDir()
	outside := root + "/../escape-x.yaml"
	if err := os.WriteFile(outside, []byte("class: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })
	if err := s.Remove(root, "../../..", "x"); err == nil {
		t.Error("Remove accepted a path-traversal class")
	}
	if got := s.Check(root, "../x", "y", env(nil)); got.Applied || got.Stale {
		t.Errorf("Check accepted an unsafe name: %+v", got)
	}
}

func TestWriteRefusesUnknownItem(t *testing.T) {
	s := mustSpec(t)
	root := t.TempDir()
	err := s.Write(root, State{Class: "repo-rules", Item: "force-push", SessionID: "s1"})
	if err == nil || !strings.Contains(err.Error(), "not a suspendable item") {
		t.Fatalf("Write(force-push) err = %v", err)
	}
	if _, statErr := os.Stat(s.Path(root, "repo-rules", "force-push")); statErr == nil {
		t.Error("refused item left a state file")
	}
}
