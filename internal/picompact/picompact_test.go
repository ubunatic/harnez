package picompact

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testParams = Params{Threshold: 65536, ReserveFraction: 0.4, KeepFraction: 0.25}

const testModels = `{"providers": {"lmcoder": {"baseUrl": "http://x", "models": [
  {"id": "coder", "contextWindow": 32768},
  {"id": "default", "contextWindow": 65536},
  {"id": "big", "contextWindow": 131072},
  {"id": "nowindow"},
  {"id": "zero", "contextWindow": 0}
]}}}`

// guardPiDir points PI_CODING_AGENT_DIR at a temp dir so no test can reach the real ~/.pi.
func guardPiDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PI_CODING_AGENT_DIR", dir)
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readSettings(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestDerive(t *testing.T) {
	got, ok := Derive(32768, testParams)
	if !ok || got != (Override{ReserveTokens: 13107, KeepRecentTokens: 8192}) {
		t.Fatalf("Derive(32768) = %+v, %v; want 13107/8192", got, ok)
	}
	for _, window := range []int{65536, 131072, 0, -1} {
		if got, ok := Derive(window, testParams); ok {
			t.Errorf("Derive(%d) = %+v; want no override", window, got)
		}
	}
}

func TestSyncWritesOnlySmallWindowModels(t *testing.T) {
	dir := guardPiDir(t)
	writeFile(t, dir, "models.json", testModels)
	var warn bytes.Buffer
	if err := Sync(dir, testParams, &warn); err != nil {
		t.Fatal(err)
	}
	if warn.Len() != 0 {
		t.Fatalf("unexpected warning: %q", warn.String())
	}
	overrides := readSettings(t, dir)["compaction"].(map[string]any)["modelOverrides"].(map[string]any)
	if len(overrides) != 1 {
		t.Fatalf("overrides = %v; want only lmcoder/coder", overrides)
	}
	entry := overrides["lmcoder/coder"].(map[string]any)
	if entry["reserveTokens"] != float64(13107) || entry["keepRecentTokens"] != float64(8192) {
		t.Fatalf("lmcoder/coder = %v", entry)
	}
}

func TestSyncKeepsUnrelatedKeysAndHandWrittenOverrides(t *testing.T) {
	dir := guardPiDir(t)
	writeFile(t, dir, "models.json", `{"providers": {"lmcoder": {"models": [
  {"id": "coder", "contextWindow": 32768}, {"id": "strata", "contextWindow": 32768}]}}}`)
	writeFile(t, dir, "settings.json", `{"theme": "dark", "packages": ["a"],
  "compaction": {"enabled": true, "reserveTokens": 16384, "modelOverrides": {
    "lmcoder/coder": {"reserveTokens": 14000, "keepRecentTokens": 8000},
    "other/x": {"reserveTokens": 1}}}}`)
	var warn bytes.Buffer
	if err := Sync(dir, testParams, &warn); err != nil {
		t.Fatal(err)
	}
	s := readSettings(t, dir)
	if s["theme"] != "dark" || len(s["packages"].([]any)) != 1 {
		t.Fatalf("unrelated keys lost: %v", s)
	}
	c := s["compaction"].(map[string]any)
	if c["enabled"] != true || c["reserveTokens"] != float64(16384) {
		t.Fatalf("compaction fields lost: %v", c)
	}
	o := c["modelOverrides"].(map[string]any)
	if e := o["lmcoder/coder"].(map[string]any); e["reserveTokens"] != float64(14000) || e["keepRecentTokens"] != float64(8000) {
		t.Fatalf("hand-written override changed: %v", e)
	}
	if _, ok := o["other/x"]; !ok {
		t.Fatal("unrelated override removed")
	}
	if e := o["lmcoder/strata"].(map[string]any); e["reserveTokens"] != float64(13107) {
		t.Fatalf("lmcoder/strata = %v", e)
	}
	if lines := strings.Count(warn.String(), "\n"); lines != 1 || !strings.Contains(warn.String(), "lmcoder/coder") {
		t.Fatalf("want one warning line naming lmcoder/coder, got %q", warn.String())
	}
}

func TestSyncIsIdempotent(t *testing.T) {
	dir := guardPiDir(t)
	writeFile(t, dir, "models.json", testModels)
	if err := Sync(dir, testParams, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "settings.json")
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	var warn bytes.Buffer
	if err := Sync(dir, testParams, &warn); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) || !info.ModTime().Equal(old) || warn.Len() != 0 {
		t.Fatalf("second sync rewrote settings or warned: %q", warn.String())
	}
}

func TestMissingOverride(t *testing.T) {
	dir := guardPiDir(t)
	writeFile(t, dir, "models.json", `{"providers": {
  "lmcoder": {"models": [{"id": "coder", "contextWindow": 32768}, {"id": "default", "contextWindow": 65536}]},
  "a": {"models": [{"id": "dup", "contextWindow": 32768}, {"id": "solo", "contextWindow": 32768}]},
  "b": {"models": [{"id": "dup", "contextWindow": 32768}]}}}`)
	for _, tc := range []struct {
		provider, name, key string
		missing             bool
	}{
		{"lmcoder", "coder", "lmcoder/coder", true},
		{"lmcoder", "default", "", false},
		{"pi", "solo", "a/solo", true},
		{"pi", "a/dup", "a/dup", true},
		{"pi", "dup", "", false},
		{"claude", "coder", "", false},
	} {
		key, window, missing := MissingOverride(dir, tc.provider, tc.name, testParams)
		if missing != tc.missing || (missing && (key != tc.key || window != 32768)) {
			t.Errorf("MissingOverride(%s, %s) = %q, %d, %v", tc.provider, tc.name, key, window, missing)
		}
	}
	writeFile(t, dir, "settings.json", `{"compaction": {"modelOverrides": {"lmcoder/coder": {"reserveTokens": 1}}}}`)
	if _, _, missing := MissingOverride(dir, "lmcoder", "coder", testParams); missing {
		t.Error("override present but reported missing")
	}
}

func TestSyncMissingIsNoOp(t *testing.T) {
	dir := guardPiDir(t)
	for _, d := range []string{"", filepath.Join(dir, "absent"), dir} {
		var warn bytes.Buffer
		if err := Sync(d, testParams, &warn); err != nil || warn.Len() != 0 {
			t.Fatalf("Sync(%q) = %v, %q; want silent no-op", d, err, warn.String())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "settings.json")); !os.IsNotExist(err) {
		t.Fatal("settings.json created without models.json")
	}
}

func TestSyncMalformedFilesWarnAndKeepSettings(t *testing.T) {
	for name, files := range map[string][2]string{
		"models":   {"{not json", `{"theme": "dark"}`},
		"settings": {testModels, "{not json"},
	} {
		dir := guardPiDir(t)
		writeFile(t, dir, "models.json", files[0])
		writeFile(t, dir, "settings.json", files[1])
		var warn bytes.Buffer
		if err := Sync(dir, testParams, &warn); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !strings.Contains(warn.String(), "Warning:") {
			t.Fatalf("%s: no warning, got %q", name, warn.String())
		}
		got, _ := os.ReadFile(filepath.Join(dir, "settings.json"))
		if string(got) != files[1] {
			t.Fatalf("%s: settings.json touched: %q", name, got)
		}
	}
}
