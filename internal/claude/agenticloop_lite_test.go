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

// TestAgenticLoopLiteStructuralGate is the stopgap structural gate for issue
// 359: it asserts every bolded rule-name and ### heading present in the full
// AgenticLoop.md doc has a corresponding entry in AgenticLoop.lite.md. It
// catches omission (a rule silently dropped), not wrongness (a rule
// paraphrased incorrectly) — see the ticket for why omission is the failure
// mode this pilot guards against. A CLI `harnez docs variant --check` verb
// (proposed by the ticket) does not exist yet; this test is the stopgap
// until that's built as a follow-up.
func TestAgenticLoopLiteStructuralGate(t *testing.T) {
	full, err := os.ReadFile("../../docs/practices/AgenticLoop.md")
	if err != nil {
		t.Fatalf("read full doc: %v", err)
	}
	lite, err := os.ReadFile("../../docs/practices/AgenticLoop.lite.md")
	if err != nil {
		t.Fatalf("read lite doc: %v", err)
	}
	liteText := string(lite)

	headingRE := regexp.MustCompile(`(?m)^###\s+(.+)$`)
	boldRE := regexp.MustCompile(`\*\*([^*]+)\*\*`)

	var missing []string
	for _, m := range headingRE.FindAllStringSubmatch(string(full), -1) {
		name := m[1]
		if !containsFold(liteText, name) {
			missing = append(missing, "### "+name)
		}
	}
	for _, m := range boldRE.FindAllStringSubmatch(string(full), -1) {
		name := m[1]
		if !containsFold(liteText, name) {
			missing = append(missing, "**"+name+"**")
		}
	}
	if len(missing) > 0 {
		t.Errorf("AgenticLoop.lite.md omits %d heading/rule-name(s) present in the full doc:\n%s",
			len(missing), joinLines(missing))
	}
}
