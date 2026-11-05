package modelblock

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

var t0 = time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)

func TestSpecValidatesAgainstSchema(t *testing.T) {
	data, err := fs.ReadFile(harnez.DefaultFS, specPath)
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	raw, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/model_blocks.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolved.Validate(doc); err != nil {
		t.Fatalf("spec violates schema: %v", err)
	}
	if _, err := LoadSpec(); err != nil {
		t.Fatal(err)
	}
}

func TestDetectKnownQuotaErrors(t *testing.T) {
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, provider, text string
		want                 time.Time
		ok                   bool
	}{
		{"agy default duration", "agy", "Error: Individual quota reached for claude-sonnet-5-5", t0.Add(5 * time.Hour), true},
		{"codex relative reset", "codex", "You've hit your usage limit. Try again in 3 hours 20 minutes.", t0.Add(3*time.Hour + 20*time.Minute), true},
		{"codex absolute reset", "codex", "usage limit reached, try again at 2026-11-05T18:00:00Z", time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC), true},
		{"codex usage_limit code", "codex", `{"type":"usage_limit_reached"}`, t0.Add(5 * time.Hour), true},
		{"unrelated error", "codex", "network unreachable", time.Time{}, false},
		{"pattern is per provider", "claude", "Individual quota reached", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := s.Detect(c.provider, c.text, t0)
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("%s: Detect = %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestStoreAddActiveRemoveAndExpiry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blocks.json")
	if got := Active(path, t0); len(got) != 0 {
		t.Fatalf("missing file should mean no blocks, got %v", got)
	}
	if err := Add(path, Block{Provider: "agy", Model: "m", Until: t0.Add(time.Hour), Reason: "429"}, t0); err != nil {
		t.Fatal(err)
	}
	if got := Active(path, t0); len(got) != 1 || got[0].Reason != "429" {
		t.Fatalf("Active = %v", got)
	}
	if got := Active(path, t0.Add(time.Hour)); len(got) != 0 {
		t.Fatalf("block must expire at its Until, got %v", got)
	}
	// Re-adding replaces rather than duplicates.
	if err := Add(path, Block{Provider: "agy", Model: "m", Until: t0.Add(2 * time.Hour)}, t0); err != nil {
		t.Fatal(err)
	}
	if got := Active(path, t0); len(got) != 1 || !got[0].Until.Equal(t0.Add(2*time.Hour)) {
		t.Fatalf("replace failed: %v", got)
	}
	if ok, err := Remove(path, "agy", "m"); err != nil || !ok {
		t.Fatalf("Remove = %v, %v", ok, err)
	}
	if ok, _ := Remove(path, "agy", "m"); ok {
		t.Fatal("second Remove should report nothing removed")
	}
}

func TestAutoBlockWritesOnlyForKnownQuotaErrors(t *testing.T) {
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "blocks.json")
	if _, ok, err := AutoBlock(path, s, "codex", "gpt-5", "connection reset", t0); err != nil || ok {
		t.Fatalf("unrelated error blocked: %v %v", ok, err)
	}
	b, ok, err := AutoBlock(path, s, "agy", "claude-sonnet-5-5", "Individual quota reached\nmore", t0)
	if err != nil || !ok {
		t.Fatalf("AutoBlock = %v %v", ok, err)
	}
	got := Active(path, t0)
	if len(got) != 1 || got[0].Model != "claude-sonnet-5-5" || !got[0].Until.Equal(b.Until) || got[0].Reason != "auto: Individual quota reached" {
		t.Fatalf("recorded = %+v", got)
	}
}

func TestParseUntil(t *testing.T) {
	if got, err := ParseUntil("90m", t0); err != nil || !got.Equal(t0.Add(90*time.Minute)) {
		t.Fatalf("duration: %v %v", got, err)
	}
	if got, err := ParseUntil("2026-11-06T00:00:00Z", t0); err != nil || got.Day() != 6 {
		t.Fatalf("rfc3339: %v %v", got, err)
	}
	for _, bad := range []string{"soon", "-1h", ""} {
		if _, err := ParseUntil(bad, t0); err == nil {
			t.Errorf("ParseUntil(%q) should fail", bad)
		}
	}
}
