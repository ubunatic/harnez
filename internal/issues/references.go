package issues

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

type ReferenceEdit struct {
	Path          string
	Before, After []byte
	Mode          fs.FileMode
}

// Plan exact filename references only: bare ticket numbers and longer filenames
// are ambiguous and must survive renumbering unchanged. Do not follow symlinks.
func PlanReferences(dir, oldPath, newPath string) ([]ReferenceEdit, error) {
	var edits []ReferenceEdit
	for _, sub := range []string{"issues", "docs"} {
		root := filepath.Join(dir, sub)
		if _, err := os.Stat(root); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !entry.Type().IsRegular() {
				return nil
			}
			// The moved ticket is rewritten separately. README prose is included;
			// its generated table is refreshed afterward by index.
			rel, err := filepath.Rel(dir, path)
			if err != nil {
				return err
			}
			if filepath.ToSlash(rel) == oldPath {
				return nil
			}
			before, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !utf8.Valid(before) || bytes.ContainsRune(before, 0) {
				return nil
			}
			after := RewriteReferences(string(before), oldPath, newPath)
			if after == string(before) {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			edits = append(edits, ReferenceEdit{path, before, []byte(after), info.Mode().Perm()})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return edits, nil
}

func RewriteReferences(content, oldPath, newPath string) string {
	token := regexp.MustCompile(`[\pL\pN_./-]+`)
	return token.ReplaceAllStringFunc(content, func(match string) string {
		// A sentence's trailing period is punctuation, not a filename suffix.
		name := strings.TrimRight(match, ".")
		punctuation := strings.TrimPrefix(match, name)
		if name == filepath.Base(oldPath) {
			return filepath.Base(newPath) + punctuation
		}
		path := name
		for strings.HasPrefix(path, "../") {
			path = strings.TrimPrefix(path, "../")
		}
		path = strings.TrimPrefix(path, "./")
		if path == oldPath {
			return strings.TrimSuffix(name, oldPath) + newPath + punctuation
		}
		return match
	})
}

func WriteReferences(edits []ReferenceEdit, restore bool) error {
	for _, edit := range edits {
		data := edit.After
		if restore {
			data = edit.Before
		}
		if err := os.WriteFile(edit.Path, data, edit.Mode); err != nil {
			return err
		}
	}
	return nil
}
