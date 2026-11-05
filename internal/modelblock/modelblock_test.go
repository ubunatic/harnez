package modelblock

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
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
		{"agy default duration", "agy", "agy: RESOURCE_EXHAUSTED (code 429): Individual quota reached", t0.Add(5 * time.Hour), true},
		{"agy status form", "agy", `agy reported status "ERROR": RESOURCE_EXHAUSTED (code 429): Individual quota reached. Resets in 90 minutes`, t0.Add(90 * time.Minute), true},
		{"codex relative reset", "codex", "codex turn failed: You've hit your usage limit. Try again in 3 hours 20 minutes.", t0.Add(3*time.Hour + 20*time.Minute), true},
		{"codex absolute reset", "codex", "codex turn failed: You've hit your usage limit, try again at 2026-11-05T18:00:00Z", time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC), true},
		{"codex curly apostrophe", "codex", "codex turn failed: You’ve hit your usage limit.", t0.Add(5 * time.Hour), true},
		{"unrelated error", "codex", "network unreachable", time.Time{}, false},
		{"pattern is per provider", "claude", "agy: RESOURCE_EXHAUSTED (code 429): Individual quota reached", time.Time{}, false},
		// Ordinary failures that merely mention a quota phrase must not block.
		{"codex ordinary failure mentions usage limit", "codex", "codex turn failed: tool crashed while documenting the usage limit page", time.Time{}, false},
		{"codex quoted limit mid-text", "codex", "exit status 1: stderr: You've hit your usage limit", time.Time{}, false},
		{"codex usage_limit token only", "codex", `{"type":"usage_limit_reached"}`, time.Time{}, false},
		{"agy ordinary failure mentions quota", "agy", "agy: 503 service unavailable (see Individual quota reached docs)", time.Time{}, false},
	}
	for _, c := range cases {
		got, ok := s.Detect(c.provider, c.text, t0)
		if ok != c.ok || !got.Equal(c.want) {
			t.Errorf("%s: Detect = %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

// The repo's provider fixture says "Resets in 2h"; the reset must be read.
func TestDetectReadsResetFromAgyFixture(t *testing.T) {
	data, err := os.ReadFile("../subagent/testdata/agy_quota_error.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct{ Error string }
	if err := json.Unmarshal(data, &v); err != nil || v.Error == "" {
		t.Fatalf("fixture: %v %q", err, v.Error)
	}
	s, err := LoadSpec()
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"agy: " + v.Error, `agy reported status "ERROR": ` + v.Error, "agy resume: " + v.Error} {
		got, ok := s.Detect("agy", text, t0)
		if !ok || !got.Equal(t0.Add(2*time.Hour)) {
			t.Errorf("Detect(%q) = %v, %v; want %v", text, got, ok, t0.Add(2*time.Hour))
		}
	}
	other, _ := os.ReadFile("../subagent/testdata/agy_unlisted_error.json")
	var o struct{ Error string }
	_ = json.Unmarshal(other, &o)
	if _, ok := s.Detect("agy", "agy: "+o.Error, t0); ok {
		t.Error("unlisted agy error blocked")
	}
}

func TestSpecRejectsUnanchoredMatch(t *testing.T) {
	_, err := parseSpec([]byte("default_duration: 1h\npatterns:\n  - provider: x\n    match: 'usage limit'\n"))
	if err == nil {
		t.Fatal("unanchored match accepted")
	}
}

// Concurrent block/unblock must neither lose updates nor fail on a shared temp
// file; run with -race.
func TestConcurrentAddAndRemoveKeepAllBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "blocks.json")
	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, 2*n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			model := fmt.Sprintf("m%d", i)
			errs <- Add(path, Block{Provider: "p", Model: model, Until: t0.Add(time.Hour)}, t0)
			if i%2 == 0 {
				_, err := Remove(path, "p", model)
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := Active(path, t0); len(got) != n/2 {
		t.Fatalf("%d blocks survive, want %d (odd models): %+v", len(got), n/2, got)
	}
	left, _ := filepath.Glob(path + ".*.tmp")
	if len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
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
	b, ok, err := AutoBlock(path, s, "agy", "claude-sonnet-5-5", "agy: RESOURCE_EXHAUSTED (code 429): Individual quota reached\nmore", t0)
	if err != nil || !ok {
		t.Fatalf("AutoBlock = %v %v", ok, err)
	}
	got := Active(path, t0)
	if len(got) != 1 || got[0].Model != "claude-sonnet-5-5" || !got[0].Until.Equal(b.Until) || got[0].Reason != "auto: agy: RESOURCE_EXHAUSTED (code 429): Individual quota reached" {
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
