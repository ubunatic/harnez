package claude

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestManagedDocMarkerlessEditsArePreserved(t *testing.T) {
	for _, variant := range []string{"lite", "full"} {
		t.Run(variant, func(t *testing.T) {
			dst := filepath.Join(t.TempDir(), "Spec.md")
			local := []byte("# New spec\n\n## Widget Boundary\nlocal text\n")
			if err := os.WriteFile(dst, local, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := writeManagedDoc(dst, []byte("# New spec\n"))
			if err == nil || !strings.Contains(err.Error(), "<!-- harnez:stop -->") {
				t.Fatalf("expected actionable gate, got %v", err)
			}
			got, err := os.ReadFile(dst)
			if err != nil || !bytes.Equal(got, local) {
				t.Fatalf("markerless local bytes changed: %q, %v", got, err)
			}
		})
	}
}

func TestManagedDocUnmodifiedUpdatesAndDetectsLaterEdits(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "Spec.md")
	for _, version := range []string{"# Spec v1\n", "# Spec v2\r\n"} {
		if _, err := writeManagedDoc(dst, []byte(version)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(dst)
	if err != nil || !bytes.HasPrefix(got, []byte("# Spec v2\n\n<!-- harnez:stop -->")) {
		t.Fatalf("ordinary update failed: %q, %v", got, err)
	}
	if err := os.WriteFile(dst, append(got, []byte("## Local\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeManagedDoc(dst, []byte("# Spec v3\n")); err != nil {
		t.Fatal(err)
	}
	got, err = os.ReadFile(dst)
	if err != nil || !bytes.Contains(got, []byte("## Local\n")) {
		t.Fatalf("stop-marker local tail lost: %q, %v", got, err)
	}
}

func TestManagedDocStopTailRemainsByteExact(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "Spec.md")
	tail := []byte("<!-- harnez:stop -->\r\n## Local\r\n\x00custom\n")
	if err := os.WriteFile(dst, append([]byte("# old\n"), tail...), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := writeManagedDoc(dst, []byte("# new\n")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || !bytes.HasSuffix(got, tail) {
		t.Fatalf("stop tail changed: %q, %v", got, err)
	}
}

func TestManagedDocAmbiguousLegacyDifferenceWarnsAndReplaces(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "Spec.md")
	if err := os.WriteFile(dst, []byte("# Older bundled text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, warning, err := prepareManagedDoc(dst, []byte("# New bundled text\n"))
	if err != nil || !strings.Contains(warning, "review the git diff") {
		t.Fatalf("expected visible review warning, got %q, %v", warning, err)
	}
	if string(got) != "# New bundled text\n\n<!-- harnez:stop -->\n" {
		t.Fatalf("unexpected replacement: %q", got)
	}
}

func TestInitSpecMarkerlessLocalSectionBothVariants(t *testing.T) {
	for _, variant := range []string{"lite", "full"} {
		t.Run(variant, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# project\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(filepath.Join(dir, "docs"), 0o755); err != nil {
				t.Fatal(err)
			}
			dst := filepath.Join(dir, "docs", "Spec.md")
			local := []byte("# New spec\n\n## Widget Boundary\nlocal\n")
			if err := os.WriteFile(dst, local, 0o644); err != nil {
				t.Fatal(err)
			}
			cfg := &Config{FS: fstest.MapFS{"spec.md": &fstest.MapFile{Data: []byte("# New spec\n")}},
				AgentsMD: AgentsMD{Languages: map[string]Language{"spec": {Source: "spec.md", Local: "./docs/Spec.md"}}}}
			err := RunInitWithVariant(dir, cfg, []string{"spec"}, "", true, false, false, false, nil, false, false, variant)
			if err == nil || !strings.Contains(err.Error(), "appended local section") {
				t.Fatalf("expected init to gate local section, got %v", err)
			}
			got, err := os.ReadFile(dst)
			if err != nil || !bytes.Equal(got, local) {
				t.Fatalf("init changed local content: %q, %v", got, err)
			}
		})
	}
}
