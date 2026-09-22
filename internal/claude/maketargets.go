package claude

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// defaultPhonySentinel is the sentinel token harnez uses to mark its
// own targets as always-phony without listing each one in a .PHONY list.
const defaultPhonySentinel = "⚙️"

// sentinelVariants are tokens seen in the wild for the same idea: the gear
// emoji in its emoji-presentation form (⚙️, U+2699 U+FE0F), its bare form
// (⚙, U+2699), and its text-presentation form (⚙︎, U+2699 U+FE0E).
var sentinelVariants = []string{"⚙️", "⚙︎", "⚙"}

var phonyLineRe = regexp.MustCompile(`(?m)^\.PHONY:[^\n]*\n?`)

// targetBlockRe matches a target header (an unindented "name:" line) plus
// every following tab-indented recipe line. Target names are the usual
// Make identifier charset; ".PHONY" itself matches too, which is fine —
// it is handled separately via phonyLineRe before target blocks are scanned.
func targetBlockRe(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `:[^\n]*\n(?:\t[^\n]*\n?)*`)
}

// detectSentinel picks the sentinel token to use: an explicit config override
// wins; otherwise, whichever known variant already appears in the file wins
// (so we don't fight a project's existing convention); otherwise the default.
func detectSentinel(content, configured string) string {
	if configured != "" {
		return configured
	}
	for _, v := range sentinelVariants {
		if strings.Contains(content, v) {
			return v
		}
	}
	return defaultPhonySentinel
}

func headerHasSentinel(headerLine, sentinel string) bool {
	return strings.Contains(headerLine, sentinel)
}

// lineDiffCount does a small LCS-based line diff and returns the number of
// changed lines (added + removed) — used to distinguish "close enough,
// offer to replace" from "user rolled their own, leave it alone".
func lineDiffCount(a, b string) int {
	al := strings.Split(strings.TrimRight(a, "\n"), "\n")
	bl := strings.Split(strings.TrimRight(b, "\n"), "\n")
	n, m := len(al), len(bl)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if al[i] == bl[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	return (n - lcs[0][0]) + (m - lcs[0][0])
}

// firstTargetName returns the name of the first target header found in
// content, e.g. "help" from a MakeTargets.mk snippet.
func firstTargetHeaderLine(block string) string {
	line, _, _ := strings.Cut(block, "\n")
	return line
}

// PromptFunc asks a yes/no question and returns the user's answer.
type PromptFunc func(question string) bool

// StdinPrompt asks on stdin, defaulting to "no" on empty input or read error.
func StdinPrompt(question string) bool {
	fmt.Printf("%s [y/N] ", question)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

// smallDiffThreshold: at or below this many changed lines, a hand-written
// target is considered "close enough" to prompt about replacing; above it,
// we assume the user rolled their own and leave it alone.
const smallDiffThreshold = 4

// legacyTargetsMarkers are the shell-comment markers used by
// the old marker-based injection (pre-2026-07-03). Any block
// bounded by them is a migration artifact and should be removed before the
// structural reconciliation runs.
var legacyTargetsMarkers = [][2]string{
	{"# harnez:begin targets", "# harnez:end targets"},
	{"# claudeconfig:begin targets", "# claudeconfig:end targets"},
}

// stripLegacyTargetsBlock removes a legacy # harnez:begin/end targets or
// # claudeconfig:begin/end targets block from content, returning the cleaned string and whether anything changed.
func stripLegacyTargetsBlock(content string) (string, bool) {
	for _, m := range legacyTargetsMarkers {
		bi := strings.Index(content, m[0])
		ei := strings.Index(content, m[1])
		if bi >= 0 && ei >= 0 && ei > bi {
			end := ei + len(m[1])
			if end < len(content) && content[end] == '\n' {
				end++
			}
			// eat a preceding blank line so we don't leave a double blank
			start := bi
			if start >= 2 && content[start-1] == '\n' && content[start-2] == '\n' {
				start--
			}
			return content[:start] + content[end:], true
		}
	}
	return content, false
}

// ReconcileMakeTargets merges the target(s) defined in oursContent (typically
// docs/templates/MakeTargets.mk) into the Makefile at dest, using structural
// detection of "our" targets (via the ⚙️ sentinel) instead of HTML/shell
// comment markers. For each of our targets:
//   - missing in dest       → append it
//   - present, sentinel-tagged → keep in sync silently (already ours)
//   - present, foreign, small diff → prompt (or auto-yes) to replace
//   - present, foreign, big diff   → leave alone, tell the user how to adopt ours
//
// It also ensures the sentinel .PHONY line exists, per cfg.Make.PhonyFix.
// Legacy targets blocks are removed automatically.
func ReconcileMakeTargets(dest, oursContent string, cfg MakeConfig, assumeYes bool, prompt PromptFunc) (changed bool, err error) {
	if prompt == nil {
		prompt = StdinPrompt
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		return false, err
	}
	content := string(data)

	// Migration: remove old marker-based block if present.
	if stripped, ok := stripLegacyTargetsBlock(content); ok {
		content = stripped
		changed = true
		fmt.Printf("  removed legacy targets block from %s\n", dest)
	}
	sentinel := detectSentinel(content, cfg.PhonySentinel)

	oursTargets := extractTargetBlocks(oursContent)

	for _, t := range oursTargets {
		re := targetBlockRe(t.name)
		loc := re.FindStringIndex(content)
		if loc == nil {
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			if !strings.HasSuffix(content, "\n\n") {
				content += "\n"
			}
			content += t.block
			changed = true
			fmt.Printf("  added target: %s\n", t.name)
			continue
		}

		existing := content[loc[0]:loc[1]]
		if existing == t.block {
			continue
		}
		existingHeader := firstTargetHeaderLine(existing)
		diff := lineDiffCount(existing, t.block)

		isManaged := headerHasSentinel(existingHeader, "🤖")
		isManual := false
		if !isManaged {
			isManual = headerHasSentinel(existingHeader, sentinel)
			if !isManual {
				for _, v := range sentinelVariants {
					if headerHasSentinel(existingHeader, v) {
						isManual = true
						break
					}
				}
			}
		}

		if isManaged {
			// Fully managed: update silently without confirmation
			content = content[:loc[0]] + t.block + content[loc[1]:]
			changed = true
			fmt.Printf("  updated target: %s (managed)\n", t.name)
			continue
		}

		if isManual {
			// Upgrade path: if diff is small (drift or migration), or if it is the legacy help target, upgrade from ⚙️ to 🤖
			isLegacyHelp := t.name == "help" &&
				strings.Contains(existing, "grep") &&
				strings.Contains(existing, "awk") &&
				strings.Contains(existing, "$$1") &&
				strings.Contains(existing, "$$2")
			if diff <= smallDiffThreshold || isLegacyHelp {
				content = content[:loc[0]] + t.block + content[loc[1]:]
				changed = true
				fmt.Printf("  upgraded target to managed: %s\n", t.name)
			} else {
				fmt.Printf("  skip target: %s (manually defined/customized)\n", t.name)
			}
			continue
		}

		if diff > smallDiffThreshold {
			fmt.Printf("  skip target: %s (differs significantly from harnez's version)\n", t.name)
			fmt.Printf("    delete it from %s and re-run init to adopt ours\n", dest)
			continue
		}

		fmt.Printf("  target %q in %s differs from harnez's version:\n", t.name, dest)
		fmt.Printf("--- existing\n%s+++ harnez\n%s", indentBlock(existing), indentBlock(t.block))
		replace := assumeYes
		if !assumeYes {
			replace = prompt(fmt.Sprintf("  replace target %q with harnez's version?", t.name))
		}
		if replace {
			content = content[:loc[0]] + t.block + content[loc[1]:]
			changed = true
			fmt.Printf("  replaced target: %s\n", t.name)
		} else {
			fmt.Printf("  kept target: %s (unchanged)\n", t.name)
		}
	}

	phonyChanged := false
	content, phonyChanged = ensurePhonySentinel(content, sentinel, cfg.phonyFixOrDefault())
	changed = changed || phonyChanged

	colorVarsChanged := false
	content, colorVarsChanged = ensureColorVariables(content, sentinel)
	changed = changed || colorVarsChanged

	if changed {
		if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
			return false, err
		}
	}
	return changed, nil
}

type makeTarget struct {
	name  string
	block string
}

// extractTargetBlocks parses a MakeTargets.mk-style snippet into its
// individual named targets, skipping the .PHONY declaration line(s).
func extractTargetBlocks(content string) []makeTarget {
	content = phonyLineRe.ReplaceAllString(content, "")
	var targets []makeTarget
	headerRe := regexp.MustCompile(`(?m)^([A-Za-z0-9_.-]+):[^\n]*\n`)
	matches := headerRe.FindAllStringIndex(content, -1)
	for i, m := range matches {
		end := len(content)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		block := strings.TrimRight(content[m[0]:end], "\n") + "\n"
		name := headerRe.FindStringSubmatch(content[m[0]:m[1]])[1]
		targets = append(targets, makeTarget{name: name, block: block})
	}
	return targets
}

// ensurePhonySentinel makes sure a sentinel .PHONY line exists (per phonyFix)
// and, in "all" mode, folds other explicit .PHONY target lists into the
// sentinel convention by tagging each of those targets' headers instead.
func ensurePhonySentinel(content, sentinel, phonyFix string) (string, bool) {
	if phonyFix == "none" {
		return content, false
	}

	phonyLines := phonyLineRe.FindAllString(content, -1)
	hasSentinelLine := false
	for _, l := range phonyLines {
		if strings.Contains(l, sentinel) && strings.Contains(l, "🤖") {
			hasSentinelLine = true
			break
		}
	}

	changed := false

	if phonyFix == "all" {
		var others []string
		for _, l := range phonyLines {
			if strings.Contains(l, sentinel) || strings.Contains(l, "🤖") {
				continue
			}
			others = append(others, l)
		}
		for _, l := range others {
			for name := range strings.FieldsSeq(strings.TrimPrefix(strings.TrimSpace(l), ".PHONY:")) {
				re := targetBlockRe(name)
				loc := re.FindStringIndex(content)
				if loc == nil {
					continue
				}
				header := firstTargetHeaderLine(content[loc[0]:loc[1]])
				if strings.Contains(header, sentinel) || strings.Contains(header, "🤖") {
					continue
				}
				newHeader := name + ": " + sentinel + strings.TrimPrefix(header, name+":")
				content = content[:loc[0]] + strings.Replace(content[loc[0]:loc[1]], header, newHeader, 1) + content[loc[1]:]
				changed = true
			}
			content = strings.Replace(content, l, "", 1)
			changed = true
		}
	}

	if !hasSentinelLine {
		var oldLine string
		for _, l := range phonyLines {
			if strings.Contains(l, sentinel) || strings.Contains(l, "🤖") {
				oldLine = l
				break
			}
		}

		newSentinelLine := fmt.Sprintf(".PHONY: %s 🤖  # ⚙️ = manual/once, 🤖 = managed\n", sentinel)
		if oldLine != "" {
			content = strings.Replace(content, oldLine, newSentinelLine, 1)
			changed = true
		} else {
			content = newSentinelLine + content
			changed = true
		}
	}

	return content, changed
}

// ensureColorVariables ensures the color helper variables (_prim and _rst) are defined in the file.
//
// The "\033[36m"/"\033[0m" literals below are deliberately NOT sourced from
// spec/colors.yaml (issue 137's audit). They are Make variable *content*
// written into a generated Makefile for other projects' `make help`-style
// output -- a different subsystem, audience, and domain than the harnez
// `usage --watch` TUI spec/colors.yaml governs: the cyan here is Make's own
// help-target color convention, consumed by whatever project's Makefile this
// is, not by harnez's own terminal rendering. Folding it into
// spec/colors.yaml would couple two unrelated color languages (harnez's TUI
// palette vs. an external Makefile's help-text color) for no benefit, so
// it's left as a plain literal here on purpose.
func ensureColorVariables(content, sentinel string) (string, bool) {
	if strings.Contains(content, "_prim :=") {
		return content, false
	}
	colorVars := "\n_prim := \\033[36m\n_rst  := \\033[0m\n"

	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if strings.HasPrefix(l, ".PHONY:") && (strings.Contains(l, sentinel) || strings.Contains(l, "🤖")) {
			newLines := append(lines[:i+1], append([]string{"_prim := \\033[36m", "_rst  := \\033[0m", ""}, lines[i+1:]...)...)
			return strings.Join(newLines, "\n"), true
		}
	}
	return colorVars + content, true
}

func indentBlock(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n") + "\n"
}
