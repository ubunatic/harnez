package markdown

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var mdMarkers = struct {
	begin func(string) string
	end   func(string) string
}{
	begin: func(s string) string { return "<!-- claudeconfig:begin " + s + " -->" },
	end:   func(s string) string { return "<!-- claudeconfig:end " + s + " -->" },
}

// SectionBounds finds the line range of a managed block.
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

// Apply updates or inserts a managed section inside a markdown file.
// Returns changed = true if the file was modified, existed = true if the section
// was already present in the file before this call.
func Apply(path, section, content string) (changed bool, existed bool, err error) {
	begin := mdMarkers.begin(section)
	end := mdMarkers.end(section)
	block := begin + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	existingContent := ""
	if data, err := os.ReadFile(path); err == nil {
		existingContent = string(data)
	}

	var newContent string
	_, _, existed = SectionBounds(existingContent, begin, end)
	if ls, le, ok := SectionBounds(existingContent, begin, end); ok {
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

// Diff shows unified diff between current section content in the file and the proposed content.
// Returns true if there is a diff.
func Diff(path, section, content string) (bool, error) {
	begin := mdMarkers.begin(section)
	end := mdMarkers.end(section)
	newBlock := begin + "\n" + strings.TrimRight(content, "\n") + "\n" + end + "\n"

	oldBlock := ""
	if data, err := os.ReadFile(path); err == nil {
		existing := string(data)
		if ls, le, ok := SectionBounds(existing, begin, end); ok {
			oldBlock = existing[ls:le]
		}
	}

	if oldBlock == newBlock {
		return false, nil
	}

	writeTemp := func(s string) (string, error) {
		f, err := os.CreateTemp("", "claudeconfig-diff-*")
		if err != nil {
			return "", err
		}
		_, err = f.WriteString(s)
		f.Close()
		return f.Name(), err
	}

	oldFile, err := writeTemp(oldBlock)
	if err != nil {
		return true, err
	}
	defer os.Remove(oldFile)

	newFile, err := writeTemp(newBlock)
	if err != nil {
		return true, err
	}
	defer os.Remove(newFile)

	label := fmt.Sprintf("%s [%s]", path, section)
	cmd := exec.Command("diff", "-u", "--label", label, "--label", label, oldFile, newFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	return true, nil
}

// Clean removes a managed block from the markdown file.
// If the file becomes empty/only whitespace, it is removed and removed is returned as true.
// Otherwise, cleaned is returned as true if the block was found and removed.
func Clean(path, section string) (removed bool, cleaned bool, err error) {
	begin := mdMarkers.begin(section)
	end := mdMarkers.end(section)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, err
	}
	existing := string(data)

	ls, le, ok := SectionBounds(existing, begin, end)
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

// ContainsSection checks if a file contains the begin marker of a section.
func ContainsSection(path, section string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), "<!-- claudeconfig:begin "+section+" -->")
}
