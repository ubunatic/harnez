// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"sort"
	"testing"

	"ubunatic.com/harnez/internal/readdiscipline"
)

// liteDocsOverReadLimit lists lite docs that are still too long to be read natively. Trim the
// doc, then delete its entry: the test fails for an entry that no longer applies (issue 1224).
var liteDocsOverReadLimit = map[string]bool{
	"docs/lang/Make.lite.md":           true,
	"docs/practices/GoRelease.lite.md": true,
}

// TestLiteDocsStayUnderReadLimit keeps lite docs readable on demand. The read guard blocks a
// native read that covers max_lines or more lines (spec/reading_discipline.yaml); a session gave
// up on docs/Bash.md when its installed copy had 104 lines (issue 1221).
func TestLiteDocsStayUnderReadLimit(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec, err := readdiscipline.Load()
	if err != nil {
		t.Fatal(err)
	}

	var sources []string
	for _, lang := range cfg.AgentsMD.Languages {
		if lang.LiteSource != "" {
			sources = append(sources, lang.LiteSource)
		}
	}
	sort.Strings(sources)
	if len(sources) == 0 {
		t.Fatal("no lite sources found; config or layout changed?")
	}

	for _, src := range sources {
		bundled, err := fs.ReadFile(cfg.FS, src)
		if err != nil {
			t.Fatalf("read %s: %v", src, err)
		}
		// Count the copy as init installs it into a project that does not have it yet.
		installed, _, err := prepareManagedDoc(filepath.Join(t.TempDir(), filepath.Base(src)), bundled)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		lines := bytes.Count(installed, []byte("\n"))
		if len(installed) > 0 && installed[len(installed)-1] != '\n' {
			lines++
		}
		over := lines >= spec.MaxLines
		switch {
		case over && !liteDocsOverReadLimit[src]:
			t.Errorf("%s installs as %d lines; the read guard blocks native reads of %d or more. Shorten it.",
				src, lines, spec.MaxLines)
		case !over && liteDocsOverReadLimit[src]:
			t.Errorf("%s installs as %d lines and is under the limit now; delete it from liteDocsOverReadLimit", src, lines)
		}
	}
	for src := range liteDocsOverReadLimit {
		if i := sort.SearchStrings(sources, src); i == len(sources) || sources[i] != src {
			t.Errorf("liteDocsOverReadLimit names %s, which is not a lite source in config.yaml", src)
		}
	}
}
