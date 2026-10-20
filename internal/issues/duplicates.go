package issues

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// CheckDuplicateNumbers rejects colliding filenames regardless of ticket headers.
// Include every claimant so a merge with three or more collisions is repairable.
func CheckDuplicateNumbers(issuesDir string) error {
	files, err := Scan(issuesDir)
	if err != nil {
		return err
	}
	groups := make(map[string][]string)
	for _, f := range files {
		groups[f.Number] = append(groups[f.Number], filepath.ToSlash(filepath.Join("issues", f.RelPath)))
	}
	var numbers []string
	for number, paths := range groups {
		if len(paths) > 1 {
			numbers = append(numbers, number)
		}
	}
	sort.Strings(numbers)
	var conflicts []string
	for _, number := range numbers {
		sort.Strings(groups[number])
		conflicts = append(conflicts, fmt.Sprintf("duplicate ticket number %s: %s", number, strings.Join(groups[number], ", ")))
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%s; renumber local numeric extras with harnez issues mv -d . <file> (run from the repository root); reconcile duplicate GH files preserving their permanent ID", strings.Join(conflicts, "; "))
	}
	return nil
}
