// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"strings"
	"testing"
)

func testConfig() *Config {
	return &Config{
		Docs: []string{"golang", "make"},
		AgentsMD: AgentsMD{
			Languages: map[string]Language{
				"golang": {Default: "auto"},
				"make":   {Default: "auto"},
				"canary": {Default: "true"},
				"spec":   {Default: "true"},
			},
		},
	}
}

func TestDocNamesInOrder(t *testing.T) {
	cfg := testConfig()
	got := docNamesInOrder(cfg)
	want := []string{"golang", "make", "canary", "spec"} // docs: order, then sorted rest
	if len(got) != len(want) {
		t.Fatalf("docNamesInOrder = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("docNamesInOrder = %v, want %v", got, want)
		}
	}
	// must be stable across calls (issue 011)
	for range 10 {
		again := docNamesInOrder(cfg)
		for i := range want {
			if again[i] != got[i] {
				t.Fatal("docNamesInOrder is not deterministic")
			}
		}
	}
}

func TestValidateDocNames(t *testing.T) {
	cfg := testConfig()
	if err := validateDocNames(cfg, []string{"golang", "canary"}); err != nil {
		t.Fatalf("valid names rejected: %v", err)
	}
	err := validateDocNames(cfg, []string{"golang", "bogus"})
	if err == nil {
		t.Fatal("unknown doc name accepted")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("error should name the unknown doc: %v", err)
	}
}

func TestDetectDoc(t *testing.T) {
	dir := t.TempDir()
	if detectDoc(dir, "zig") {
		t.Error("detectDoc(empty, zig) should be false")
	}
	// test zig detection via build.zig
	zigFile := dir + "/build.zig"
	if err := testingWriteFile(zigFile, "const std = @import(\"std\");\n"); err != nil {
		t.Fatal(err)
	}
	if !detectDoc(dir, "zig") {
		t.Error("detectDoc(dir with build.zig, zig) should be true")
	}
}

func testingWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

