package hookdispatch

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

func TestLoadBudgetReadsEmbeddedSpec(t *testing.T) {
	got, err := LoadBudget()
	if err != nil {
		t.Fatal(err)
	}
	if got != 40*time.Second {
		t.Fatalf("LoadBudget = %s, want 40s", got)
	}
}

// The handler budget must exceed every inner budget a handler can spend,
// so an inner timeout always fires first and the dispatcher only catches hangs.
func TestHandlerBudgetExceedsInnerDecideTimeouts(t *testing.T) {
	budget, err := LoadBudget()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := harnez.DefaultFS.ReadFile("spec/decide.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var decide struct {
		Backends map[string]struct {
			Timeout string `yaml:"timeout"`
		} `yaml:"backends"`
	}
	if err := yaml.Unmarshal(raw, &decide); err != nil {
		t.Fatal(err)
	}
	if len(decide.Backends) == 0 {
		t.Fatal("spec/decide.yaml has no backends to compare against")
	}
	for name, b := range decide.Backends {
		inner, err := time.ParseDuration(b.Timeout)
		if err != nil {
			t.Fatalf("backend %s timeout %q: %v", name, b.Timeout, err)
		}
		if budget <= inner {
			t.Errorf("handler_timeout %s must exceed backend %s timeout %s", budget, name, inner)
		}
	}
}
