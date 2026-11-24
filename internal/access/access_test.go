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
}

func TestValidateRejectsBadManifest(t *testing.T) {
	cases := map[string]string{
		"unknown root": `version: 1
entries:
  - {id: a, root: etc, path: x, access: read, purpose: p}
`,
		"absolute path": `version: 1
entries:
  - {id: a, root: home, path: /x, access: read, purpose: p}
`,
		"duplicate id": `version: 1
entries:
  - {id: a, root: home, path: x, access: read, purpose: p}
  - {id: a, root: home, path: y, access: read, purpose: p}
`,
		"retired modes key": `version: 1
entries:
  - {id: a, root: home, path: x, access: read, modes: [host], purpose: p}
`,
		"retired mode_probe key": `version: 1
mode_probe: a
entries:
  - {id: a, root: home, path: x, access: read, purpose: p}
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

func TestRowsCoverEveryEntry(t *testing.T) {
	spec, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	rows := spec.Rows(t.TempDir())
	if len(rows) != len(spec.Entries) {
		t.Fatalf("rows = %d, want %d", len(rows), len(spec.Entries))
	}
	for _, r := range rows {
		if r.Status == "" || r.Path == "" {
			t.Fatalf("row without status or path: %+v", r)
		}
	}
}

func TestPathRowReportsStoreAndSpool(t *testing.T) {
	dir := t.TempDir()
	store := filepath.Join(dir, "harnez-agents")
	if err := os.Mkdir(store, 0o700); err != nil {
		t.Fatal(err)
	}
	r := PathRow("store", "readwrite", store, "session registry")
	if r.ID != "store" || r.Status != "ok" || r.Path != store {
		t.Fatalf("store row = %+v, want ok", r)
	}
	r = PathRow("spool", "readwrite", filepath.Join(dir, "spool"), "pointers")
	if r.Status != "create" {
		t.Fatalf("missing spool status = %s, want create", r.Status)
	}
}

func TestRenderWritesHeaderAndRows(t *testing.T) {
	var buf bytes.Buffer
	rows := []Row{{ID: "x", Access: "read", Status: "ok", Path: "/p", Purpose: "why"}}
	if err := Render(&buf, rows); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	want := "id\taccess\tstatus\tpath\tpurpose\nx\tread\tok\t/p\twhy\n"
	if got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
	if strings.Count(got, "\n") != 2 {
		t.Fatalf("unexpected line count in %q", got)
	}
}
