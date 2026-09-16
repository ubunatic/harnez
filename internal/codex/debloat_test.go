package codex

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDebloatRoundTripPreservesUnlistedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := `model = "gpt-5.6-sol"

[features]
apps = true
plugins = false
multi_agent = true

[hooks.harnez]
enabled = true
`
	if err := os.WriteFile(path, []byte(before), 0644); err != nil {
		t.Fatal(err)
	}
	initial, _, err := readDebloatConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"apps": false, "plugins": false}
	changed, err := ApplyDebloat(path, wanted)
	if err != nil || !changed {
		t.Fatalf("ApplyDebloat = changed %v, err %v", changed, err)
	}
	afterApply, _, err := readDebloatConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	features := afterApply["features"].(map[string]any)
	if features["apps"] != false || features["plugins"] != false || features["multi_agent"] != true {
		t.Fatalf("wrong feature values after apply: %v", features)
	}
	if afterApply["model"] != initial["model"] || !reflect.DeepEqual(afterApply["hooks"], initial["hooks"]) {
		t.Fatalf("unlisted config changed: %v", afterApply)
	}
	changed, err = ApplyDebloat(path, wanted)
	if err != nil || changed {
		t.Fatalf("repeat ApplyDebloat = changed %v, err %v", changed, err)
	}
	reverted, err := RevertDebloat(path)
	if err != nil || !reverted {
		t.Fatalf("RevertDebloat = reverted %v, err %v", reverted, err)
	}
	afterRevert, _, err := readDebloatConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(afterRevert, initial) {
		t.Fatalf("round trip changed config:\nbefore=%v\nafter=%v", initial, afterRevert)
	}
	if _, err := os.Stat(debloatRecordPath(path)); !os.IsNotExist(err) {
		t.Fatalf("expected ownership record removed, stat: %v", err)
	}
}

func TestDebloatAbsentFileAndIncrementalSpec(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if _, err := ApplyDebloat(path, map[string]bool{"apps": false}); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDebloat(path, map[string]bool{"apps": false, "plugins": false}); err != nil {
		t.Fatal(err)
	}
	rec, _, err := readDebloatRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Prior) != 2 || rec.Prior["apps"] != nil || rec.Prior["plugins"] != nil {
		t.Fatalf("wrong ownership record: %+v", rec)
	}
	if _, err := RevertDebloat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected original absence restored, stat: %v", err)
	}
}

func TestDebloatSpecRemovalRestoresExcludedFeature(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "[features]\napps = true\nplugins = false\nmulti_agent = true\n"
	if err := os.WriteFile(path, []byte(before), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDebloat(path, map[string]bool{"apps": false, "plugins": false}); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDebloat(path, map[string]bool{"plugins": false}); err != nil {
		t.Fatal(err)
	}
	doc, _, err := readDebloatConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	features := doc["features"].(map[string]any)
	if features["apps"] != true || features["plugins"] != false || features["multi_agent"] != true {
		t.Fatalf("wrong config after removing apps from spec: %v", features)
	}
	rec, _, err := readDebloatRecord(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, tracked := rec.Prior["apps"]; tracked {
		t.Fatalf("excluded apps remains owned: %+v", rec)
	}
	if _, err := ApplyDebloat(path, nil); err != nil {
		t.Fatal(err)
	}
	doc, _, err = readDebloatConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(doc["features"], map[string]any{"apps": true, "plugins": false, "multi_agent": true}) {
		t.Fatalf("wrong config after empty spec: %v", doc["features"])
	}
	if _, err := os.Stat(debloatRecordPath(path)); !os.IsNotExist(err) {
		t.Fatalf("expected ownership record removed after empty spec: %v", err)
	}
}

func TestDebloatRejectsMalformedTOMLWithoutOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := []byte("[features\napps = true\n")
	if err := os.WriteFile(path, before, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyDebloat(path, map[string]bool{"apps": false}); err == nil {
		t.Fatal("expected invalid TOML error")
	}
	after, err := os.ReadFile(path)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("invalid config changed: %q, err: %v", after, err)
	}
	if _, err := os.Stat(debloatRecordPath(path)); !os.IsNotExist(err) {
		t.Fatalf("unexpected ownership record after failed apply: %v", err)
	}
}
