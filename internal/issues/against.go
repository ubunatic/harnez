package issues

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// Collision is a ticket file in the working tree whose number a different file on the
// checked ref already uses.
type Collision struct {
	Number string
	Local  string // issues/<path> in the working tree
	Remote string // issues/<path> on the ref
}

// CheckAgainst lists the ticket files of this checkout that reuse a number the ref gives to a
// different file. A file that already exists on the ref under the same path is not new, so it
// is never a collision. ok is false when the ref does not exist, for example before a fetch.
func CheckAgainst(dir, ref string) (collisions []Collision, ok bool, err error) {
	if _, err := gitStdout(dir, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err != nil {
		return nil, false, nil
	}
	local, err := Scan(filepath.Join(dir, "issues"))
	if err != nil {
		return nil, true, err
	}
	listing, err := gitStdout(dir, "ls-tree", "-r", "-z", "--name-only", ref, "--", "issues")
	if err != nil {
		return nil, true, fmt.Errorf("list tickets on %s: %w", ref, err)
	}
	remote := ticketFilesFromPaths(strings.Split(listing, "\x00"), ref)
	onRef := make(map[string]bool, len(remote))
	byNumber := make(map[string][]IssueFile, len(remote))
	for _, r := range remote {
		onRef[r.RelPath] = true
		byNumber[r.Number] = append(byNumber[r.Number], r)
	}
	for _, f := range local {
		if onRef[f.RelPath] {
			continue
		}
		for _, r := range byNumber[f.Number] {
			collisions = append(collisions, Collision{
				Number: f.Number,
				Local:  "issues/" + f.RelPath,
				Remote: "issues/" + r.RelPath,
			})
		}
	}
	return collisions, true, nil
}

// LintAgainst is `harnez issues lint --against <ref>`. A missing ref only warns on w, so an
// offline or not-yet-fetched checkout is not blocked. A confirmed collision is an error that
// names both paths; the caller exits non-zero on it.
func LintAgainst(w io.Writer, dir, ref string) error {
	collisions, ok, err := CheckAgainst(dir, ref)
	if err != nil {
		return fmt.Errorf("issues lint --against %s: %w", ref, err)
	}
	if !ok {
		fmt.Fprintf(w, "issues lint: %s not found; fetch it to check ticket numbers against it\n", ref)
		return nil
	}
	if len(collisions) == 0 {
		return nil
	}
	var lines []string
	for _, c := range collisions {
		lines = append(lines, fmt.Sprintf("ticket %s: %s reuses the number of %s on %s", c.Number, c.Local, c.Remote, ref))
	}
	return fmt.Errorf("issues lint --against %s: %d number collision(s); renumber with `harnez issues mv`:\n  %s",
		ref, len(collisions), strings.Join(lines, "\n  "))
}
