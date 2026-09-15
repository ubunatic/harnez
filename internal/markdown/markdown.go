package markdown

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Markers defines the begin/end comment style for managed sections.
type Markers struct {
	Begin func(string) string
	End   func(string) string
}

// MDMarkers uses HTML comments — suitable for Markdown files.
var MDMarkers = Markers{
	Begin: func(s string) string { return "<!-- harnez:begin " + s + " -->" },
	End:   func(s string) string { return "<!-- harnez:end " + s + " -->" },
}

// DocStopMarker and LegacyDocStopMarker define stop markers for harnez-managed docs in docs/.
// Content before the stop marker is harnez-managed upstream content; content after is repo-local customization.
const (
	DocStopMarker       = "<!-- harnez:stop -->"
	DocEndMarker        = "<!-- harnez:end -->"
	LegacyDocStopMarker = "<!-- claudeconfig:stop -->"
	LegacyDocEndMarker  = "<!-- claudeconfig:end -->"
)

// ExtractManagedDocContent returns the content up to the first stop marker if present,
// with trailing newlines normalized.
func ExtractManagedDocContent(content string) string {
	for _, marker := range []string{DocStopMarker, DocEndMarker, LegacyDocStopMarker, LegacyDocEndMarker} {
		if idx := strings.Index(content, marker); idx >= 0 {
			return strings.TrimRight(content[:idx], "\r\n") + "\n"
		}
	}
	return strings.TrimRight(content, "\r\n") + "\n"
}

// VariantMarkerLite marks a copyable doc's content as the "lite" variant
// (issue 357's lite_source). It is recognized only as the first non-blank
// line of a doc's content, so it round-trips through installation without
// colliding with YAML frontmatter appearing later in the file.
const VariantMarkerLite = "<!-- harnez:variant=lite -->"

// ParseVariantMarker returns "lite" if content's first non-blank line is the
// harnez:variant=lite marker, else "" (implicit full variant).
func ParseVariantMarker(content string) string {
	line, _, _ := strings.Cut(strings.TrimLeft(content, "\r\n"), "\n")
	if strings.TrimSpace(line) == VariantMarkerLite {
		return "lite"
	}
	return ""
}

// stopMarkerIndex returns the byte offset of the first stop marker in content,
// or -1 if none is present.
func stopMarkerIndex(content string) int {
	idx := -1
	for _, marker := range []string{DocStopMarker, DocEndMarker, LegacyDocStopMarker, LegacyDocEndMarker} {
		if i := strings.Index(content, marker); i >= 0 && (idx < 0 || i < idx) {
			idx = i
		}
	}
	return idx
}

// MergeManagedDoc combines freshly bundled managed content with any repo-local
// customization already present at dst. If dst exists and contains a stop
// marker, everything from the marker onward is preserved verbatim and the
// bundled content replaces only the managed portion above it. If dst does
// not exist or has no stop marker, the bundled content is returned as-is —
// there is no repo-local tail to preserve.
func MergeManagedDoc(dst string, bundled []byte) []byte {
	existing, err := os.ReadFile(dst)
	if err != nil {
		return bundled
	}
	idx := stopMarkerIndex(string(existing))
	if idx < 0 {
		return bundled
	}
	managed := strings.TrimRight(string(bundled), "\r\n")
	local := string(existing)[idx:]
	return []byte(managed + "\n\n" + local)
}

// MKMarkers uses shell comments — suitable for Makefiles.
var MKMarkers = Markers{
	Begin: func(s string) string { return "# harnez:begin " + s },
	End:   func(s string) string { return "# harnez:end " + s },
}

// LegacyMDMarkers matches the previous claudeconfig HTML comment markers for backward compatibility.
var LegacyMDMarkers = Markers{
	Begin: func(s string) string { return "<!-- claudeconfig:begin " + s + " -->" },
	End:   func(s string) string { return "<!-- claudeconfig:end " + s + " -->" },
}

// LegacyMKMarkers matches the previous claudeconfig Makefile comment markers for backward compatibility.
var LegacyMKMarkers = Markers{
	Begin: func(s string) string { return "# claudeconfig:begin " + s },
	End:   func(s string) string { return "# claudeconfig:end " + s },
}

// SectionBounds finds the byte range of a managed block defined by begin/end markers.
func SectionBounds(existing, begin, end string) (lineStart, lineEnd int, found bool) {
	bi := strings.Index(existing, begin)
	ei := strings.Index(existing, end)
	if bi < 0 || ei < 0 || ei <= bi {
		return 0, 0, false
	}
	ls := strings.LastIndex(existing[:bi], "\n") + 1
	le := ei + len(end)
	if le < len(existing) && existing[le] == '\n' {
		le++
	}
	return ls, le, true
}

func locateSection(existingContent, section string, m Markers) (lineStart, lineEnd int, found bool) {
	if ls, le, ok := SectionBounds(existingContent, m.Begin(section), m.End(section)); ok {
		return ls, le, true
	}
	// Fallback to legacy markers for backward compatibility
	sample := m.Begin("x")
	if strings.Contains(sample, "<!--") {
		return SectionBounds(existingContent, LegacyMDMarkers.Begin(section), LegacyMDMarkers.End(section))
	} else if strings.HasPrefix(sample, "#") {
		return SectionBounds(existingContent, LegacyMKMarkers.Begin(section), LegacyMKMarkers.End(section))
	}
	return 0, 0, false
}

func applySection(path, section, content string, m Markers) (changed bool, existed bool, err error) {
	begin := m.Begin(section)
	end := m.End(section)
	block := begin + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	existingContent := ""
	if data, err := os.ReadFile(path); err == nil {
		existingContent = string(data)
	}

	var newContent string
	ls, le, ok := locateSection(existingContent, section, m)
	existed = ok
	if ok {
		newContent = existingContent[:ls] + block + existingContent[le:]
	} else {
		tail := existingContent
		if tail != "" && !strings.HasSuffix(tail, "\n") {
			tail += "\n"
		}
		newContent = tail + block
	}

	if newContent == existingContent {
		return false, existed, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return false, existed, err
	}
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return false, existed, err
	}

	return true, existed, nil
}

func diffSection(path, section, content string, m Markers) (bool, error) {
	begin := m.Begin(section)
	end := m.End(section)
	newBlock := begin + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	oldBlock := ""
	if data, err := os.ReadFile(path); err == nil {
		existing := string(data)
		if ls, le, ok := locateSection(existing, section, m); ok {
			oldBlock = existing[ls:le]
		}
	}

	if oldBlock == newBlock {
		return false, nil
	}

	writeTemp := func(s string) (string, error) {
		f, err := os.CreateTemp("", "harnez-diff-*")
		if err != nil {
			return "", err
		}
		_, err = f.WriteString(s)
		f.Close()
		return f.Name(), err
	}

	oldFile, err := writeTemp(oldBlock)
	if err != nil {
		return false, err
	}
	defer os.Remove(oldFile)

	newFile, err := writeTemp(newBlock)
	if err != nil {
		return false, err
	}
	defer os.Remove(newFile)

	label := fmt.Sprintf("%s [%s]", path, section)
	cmd := exec.Command("diff", "-u", "--label", label, "--label", label, oldFile, newFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("diff %s: %w", label, err)
	}
	return true, nil
}

func cleanSection(path, section string, m Markers) (removed bool, cleaned bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	existing := string(data)

	ls, le, ok := locateSection(existing, section, m)
	if !ok {
		return false, false, nil
	}
	result := existing[:ls] + existing[le:]

	if strings.TrimSpace(result) == "" {
		err = os.Remove(path)
		return true, false, err
	}
	err = os.WriteFile(path, []byte(result), 0644)
	return false, true, err
}

// Apply updates or inserts a managed section inside a Markdown file.
func Apply(path, section, content string) (changed bool, existed bool, err error) {
	return applySection(path, section, content, MDMarkers)
}

// Diff shows a unified diff for a managed section in a Markdown file.
func Diff(path, section, content string) (bool, error) {
	return diffSection(path, section, content, MDMarkers)
}

// Clean removes a managed section from a Markdown file.
func Clean(path, section string) (removed bool, cleaned bool, err error) {
	return cleanSection(path, section, MDMarkers)
}

// ContainsSection checks if a Markdown file contains the begin marker of a section.
func ContainsSection(path, section string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, MDMarkers.Begin(section)) || strings.Contains(content, LegacyMDMarkers.Begin(section))
}

// ApplyMK updates or inserts a managed section inside a Makefile.
func ApplyMK(path, section, content string) (changed bool, existed bool, err error) {
	return applySection(path, section, content, MKMarkers)
}

// DiffMK shows a unified diff for a managed section in a Makefile.
func DiffMK(path, section, content string) (bool, error) {
	return diffSection(path, section, content, MKMarkers)
}

// CleanMK removes a managed section from a Makefile.
func CleanMK(path, section string) (removed bool, cleaned bool, err error) {
	return cleanSection(path, section, MKMarkers)
}

// ContainsSectionMK checks if a Makefile contains the begin marker of a section.
func ContainsSectionMK(path, section string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	content := string(data)
	return strings.Contains(content, MKMarkers.Begin(section)) || strings.Contains(content, LegacyMKMarkers.Begin(section))
}
