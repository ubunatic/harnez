package modelcatalog_test

import (
	"testing"

	"ubunatic.com/harnez/modelcatalog"
)

// TestLoadThroughPublicPackage loads the catalog through the public import path
// from outside internal/. XDG dirs point at temp dirs so the host account's
// machine state and policy cannot change the result.
func TestLoadThroughPublicPackage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	catalog, err := modelcatalog.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if catalog == nil {
		t.Fatal("Load returned a nil catalog without an error")
	}
	defaults := 0
	for _, model := range catalog.Models() {
		if model.Source == "spec-default" {
			defaults++
		}
	}
	if defaults == 0 {
		t.Fatalf("Load returned no spec-default models; got %d models in total", len(catalog.Models()))
	}
}
