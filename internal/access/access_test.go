package access

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEmbeddedSpecLoads(t *testing.T) {
	spec, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(spec.Entries) == 0 {
		t.Fatal("manifest has no entries")
	}
	if _, ok := spec.entry(spec.ModeProbe); !ok {
		t.Fatalf("mode_probe %q not found", spec.ModeProbe)
	}
}

func TestValidateRejectsBadManifest(t *testing.T) {
	cases := map[string]string{
		"unknown root": `version: 1
mode_probe: a
entries:
  - {id: a, root: etc, path: x, access: read, modes: [host], purpose: p}
`,
		"absolute path": `version: 1
mode_probe: a
entries:
  - {id: a, root: home, path: /x, access: read, modes: [host], purpose: p}
`,
		"probe names no entry": `version: 1
mode_probe: missing
entries:
  - {id: a, root: home, path: x, access: read, modes: [host], purpose: p}
`,
		"duplicate id": `version: 1
mode_probe: a
entries:
  - {id: a, root: home, path: x, access: read, modes: [host], purpose: p}
  - {id: a, root: home, path: y, access: read, modes: [host], purpose: p}
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := decode([]byte(body)); err == nil {
				t.Fatal("decode accepted a bad manifest")
			}
		})
	}
}

func TestDetectHostWhenHomeWritable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	spec := mustLoad(t)
	mode, reason := spec.Detect(t.TempDir())
	if mode != ModeHost {
		t.Fatalf("mode = %s (%s), want host", mode, reason)
	}
}

func TestDetectConfinedWhenHomeReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.Chmod(home, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(home, 0o700) })
	spec := mustLoad(t)
	mode, reason := spec.Detect(t.TempDir())
	if mode != ModeConfined {
		t.Fatalf("mode = %s (%s), want confined", mode, reason)
	}
}

func TestStatusOf(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "there")
	if err := os.Mkdir(existing, 0o700); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, path, access, want string
	}{
		{"existing readwrite", existing, "readwrite", "ok"},
		{"missing readwrite creatable", filepath.Join(dir, "new", "deep"), "readwrite", "create"},
		{"missing read", filepath.Join(dir, "gone"), "read", "absent"},
		{"existing read", existing, "read", "ok"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := statusOf(tc.path, tc.access); got != tc.want {
				t.Fatalf("statusOf(%s, %s) = %s, want %s", tc.path, tc.access, got, tc.want)
			}
		})
	}
}

func TestStatusOfReadOnlyDirIsDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if got := statusOf(filepath.Join(dir, "child"), "readwrite"); got != "denied" {
		t.Fatalf("statusOf in read-only dir = %s, want denied", got)
	}
}

func TestRowsFilterByMode(t *testing.T) {
	spec := mustLoad(t)
	repo := t.TempDir()
	host := spec.Rows(ModeHost, repo)
	confined := spec.Rows(ModeConfined, repo)
	hasID := func(rows []Row, id string) bool {
		for _, r := range rows {
			if r.ID == id {
				return true
			}
		}
		return false
	}
	if !hasID(host, "agent_registry") || hasID(host, "confined_registry") {
		t.Fatalf("host rows wrong: %+v", host)
	}
	if hasID(confined, "agent_registry") || !hasID(confined, "confined_registry") {
		t.Fatalf("confined rows wrong: %+v", confined)
	}
	for _, r := range host {
		if r.Status == "" || r.Path == "" {
			t.Fatalf("row without status or path: %+v", r)
		}
	}
}

func TestRenderWritesHeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	rows := []Row{{ID: "x", Access: "read", Status: "ok", Path: "/p", Purpose: "why"}}
	if err := Render(&buf, ModeHost, "reason", rows); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := "mode\thost\treason\nid\taccess\tstatus\tpath\tpurpose\nx\tread\tok\t/p\twhy\n"
	if got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
	if strings.Count(got, "\n") != 3 {
		t.Fatalf("unexpected line count in %q", got)
	}
}

func mustLoad(t *testing.T) Spec {
	t.Helper()
	spec, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return spec
}
