// SPDX-FileCopyrightText: 2026 Uwe Jugel
// SPDX-License-Identifier: AGPL-3.0-or-later

package claude

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func joinLines(lines []string) string {
	return "  - " + strings.Join(lines, "\n  - ")
}

var (
	headingNumberPrefixRE = regexp.MustCompile(`^\d+\.\s*`)
	headingWordRE         = regexp.MustCompile(`[A-Za-z']{4,}`)
)

// headingCovered reports whether a full doc's heading survives, at least in
// coarse outline, somewhere in the lite doc. A from-scratch dense rewrite
// (issue 363) can legitimately rename a heading entirely while keeping its
// rule content (e.g. "Header & Strict Mode" -> "Header"; "Build dependency
// pattern" -> "build + action targets"), so this check only catches a
// section deleted outright (zero of its significant words survive anywhere)
// — one surviving word is nearly free to satisfy and proves nothing about
// whether the section's actual rule content survived, only that its name
// wasn't fully erased. Real content fidelity is carried almost entirely by
// the behavioral bench (`harnez bench run --docs lite`), which actually runs an
// isolated agent against the doc and lints its output; this test is a
// last-resort tripwire for the one failure mode the canary can't see
// (a section silently deleted rather than reworded), not a fidelity check.
func headingCovered(heading, liteText string) bool {
	heading = headingNumberPrefixRE.ReplaceAllString(strings.TrimSpace(heading), "")
	words := headingWordRE.FindAllString(heading, -1)
	if len(words) == 0 {
		return containsFold(liteText, heading)
	}
	for _, w := range words {
		if containsFold(liteText, w) {
			return true
		}
	}
	return false
}

// TestLiteDocStructuralGate is the stopgap structural gate for issue 359/361:
// for every (full, lite) doc pair with a lite_source, it asserts every ##
// top-level heading present in the full doc has a corresponding entry
// somewhere in the lite doc. It catches omission (a section silently
// dropped), not wrongness (a rule paraphrased incorrectly). A CLI `harnez
// docs variant --check` verb (issue 361) does not exist yet; this test is
// the stopgap until that's built.
func TestLiteDocStructuralGate(t *testing.T) {
	cases := []struct {
		name string
		full string
		lite string
	}{
		{"agentic-loop", "../../docs/practices/AgenticLoop.md", "../../docs/practices/AgenticLoop.lite.md"},
		{"bash", "../../docs/lang/Bash.md", "../../docs/lang/Bash.lite.md"},
		{"make", "../../docs/lang/Make.md", "../../docs/lang/Make.lite.md"},
		{"gorelease", "../../docs/practices/GoRelease.md", "../../docs/practices/GoRelease.lite.md"},
		{"issue-tracking", "../../docs/practices/IssueTracking.md", "../../docs/practices/IssueTracking.lite.md"},
	}
	headingRE := regexp.MustCompile(`(?m)^##\s+(.+)$`)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			full, err := os.ReadFile(c.full)
			if err != nil {
				t.Fatalf("read full doc: %v", err)
			}
			lite, err := os.ReadFile(c.lite)
			if err != nil {
				t.Fatalf("read lite doc: %v", err)
			}
			liteText := string(lite)

			var missing []string
			for _, m := range headingRE.FindAllStringSubmatch(string(full), -1) {
				if !headingCovered(m[1], liteText) {
					missing = append(missing, "## "+m[1])
				}
			}
			if len(missing) > 0 {
				t.Errorf("%s: lite doc omits %d top-level heading(s) present in the full doc:\n%s",
					c.name, len(missing), joinLines(missing))
			}
		})
	}
}
