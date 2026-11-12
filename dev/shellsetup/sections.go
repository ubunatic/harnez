package shellsetup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// sectionMarkers returns the begin and end lines of a managed shell section.
func sectionMarkers(owner, section string) (begin, end string) {
	return "# " + owner + ":begin " + section, "# " + owner + ":end " + section
}

// sectionBlock is the whole managed block: markers around body, ending in a newline.
func sectionBlock(owner, section, body string) string {
	begin, end := sectionMarkers(owner, section)
	return begin + "\n" + strings.TrimRight(body, "\n") + "\n" + end + "\n"
}

// sectionBounds returns the byte range of a managed block in content. The
// range starts at the begin line and includes the end line's newline.
func sectionBounds(content, owner, section string) (start, end int, found bool) {
	begin, endMarker := sectionMarkers(owner, section)
	bi := strings.Index(content, begin)
	ei := strings.Index(content, endMarker)
	if bi < 0 || ei < 0 || ei <= bi {
		return 0, 0, false
	}
	start = strings.LastIndex(content[:bi], "\n") + 1
	end = ei + len(endMarker)
	if end < len(content) && content[end] == '\n' {
		end++
	}
	return start, end, true
}

// applySection replaces the section in place, or appends it when missing.
// It reports whether the file changed and whether the section already existed.
func applySection(path, owner, section, body string) (changed, existed bool, err error) {
	block := sectionBlock(owner, section, body)
	existing := ""
	if data, readErr := os.ReadFile(path); readErr == nil {
		existing = string(data)
	}

	var next string
	if start, end, ok := sectionBounds(existing, owner, section); ok {
		existed = true
		next = existing[:start] + block + existing[end:]
	} else {
		tail := existing
		if tail != "" && !strings.HasSuffix(tail, "\n") {
			tail += "\n"
		}
		next = tail + block
	}
	if next == existing {
		return false, existed, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, existed, err
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return false, existed, err
	}
	return true, existed, nil
}

// sectionState returns "ok", "missing" or "drifted" for a managed section.
func sectionState(path, owner, section, body string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "missing"
	}
	content := string(data)
	start, end, ok := sectionBounds(content, owner, section)
	if !ok {
		return "missing"
	}
	if content[start:end] != sectionBlock(owner, section, body) {
		return "drifted"
	}
	return "ok"
}

// cleanSection removes a managed section. It deletes the file when nothing
// else remains. It returns "removed" when the file was deleted, "cleaned" when
// only the section was, and "" when there was nothing to do.
func cleanSection(path, owner, section string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	content := string(data)
	start, end, ok := sectionBounds(content, owner, section)
	if !ok {
		return "", nil
	}
	rest := content[:start] + content[end:]
	if strings.TrimSpace(rest) == "" {
		if err := os.Remove(path); err != nil {
			return "", err
		}
		return "removed", nil
	}
	if err := os.WriteFile(path, []byte(rest), 0o644); err != nil {
		return "", err
	}
	return "cleaned", nil
}

// writeIfChanged writes data to path only when the content differs. A symlink
// at path is replaced by a regular file. It reports whether it wrote.
func writeIfChanged(path string, data []byte) (bool, error) {
	if fi, err := os.Lstat(path); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(path); err != nil {
				return false, err
			}
		} else if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, data) {
			return false, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}

// validName accepts the names that become file names and marker words: letters,
// digits, '-', '_' and '.'. It rejects "" and the relative "." and "..".
func validName(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == '-', r == '_', r == '.':
		default:
			return false
		}
	}
	return true
}
