package main

import (
	"os"
	"path/filepath"
	"testing"

	"ubunatic.com/harnez/internal/subagent"
	"ubunatic.com/harnez/modelcatalog"
)

// enableJulesForTest opts this test into the shipped-disabled Jules model (issue 1242).
func enableJulesForTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	policy := "version: 1\ndefault_enabled: true\nenabled: [jules:jules]\nmodels: {}\n"
	if err := os.WriteFile(filepath.Join(dir, "models.yaml"), []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	catalog, err := modelcatalog.LoadFromDirs(t.TempDir(), dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(subagent.UseModelCatalogForTest(catalog))
}
