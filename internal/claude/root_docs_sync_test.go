// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/markdown"
)

// TestRootDocCopiesMatchSources guards this repo's own docs/*.md copies of
// copyable docs: each init-managed copy (one with a harnez:stop marker) must
// equal what `harnez init` would write from its source (variant chosen by the
// copy's marker, local harnez:stop sections kept).
// A root-only edit is silently lost on the next init (see issue 666 fallout).
func TestRootDocCopiesMatchSources(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	names := make([]string, 0, len(cfg.AgentsMD.Languages))
	for name := range cfg.AgentsMD.Languages {
		names = append(names, name)
	}
	sort.Strings(names)

	checked := 0
	for _, name := range names {
		lang := cfg.AgentsMD.Languages[name]
		if !strings.HasPrefix(lang.Local, "./docs/") || lang.Source == "" {
			continue
		}
		local := filepath.Join(root, lang.Local)
		existing, err := os.ReadFile(local)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if !hasDocStopMarker(existing) {
			continue // hand-placed copy; init does not manage or overwrite it
		}
		src := lang.SourceFor(markdown.ParseVariantMarker(string(existing)))
		bundled, err := fs.ReadFile(cfg.FS, src)
		if err != nil {
			t.Fatalf("%s: read source %s: %v", name, src, err)
		}
		want, _, err := prepareManagedDoc(local, bundled)
		if err != nil {
			t.Errorf("%s: %v", lang.Local, err)
			continue
		}
		checked++
		if !bytes.Equal(existing, want) {
			t.Errorf("%s diverges from its source %s: edit the source, then run `harnez init -d .` "+
				"(after `make install`) and commit both; `git checkout --` unrelated rewrites", lang.Local, src)
		}
	}
	if checked == 0 {
		t.Fatal("no root doc copies checked; config or layout changed?")
	}
}
