package issues

import (
	"fmt"
	"os/exec"
	"path"
	"strings"
)

// RemoteTicketFiles lists the ticket files under issues/ on every refs/remotes/*
// ref of the git repository at dir. Each entry has Ref set to the short ref name
// (e.g. "origin/main") and RelPath relative to issues/, like Scan. It reads refs
// only and never fetches, so a ticket that no earlier fetch has seen is not
// listed. A dir that is not a git work tree yields no files and no error.
func RemoteTicketFiles(dir string) ([]IssueFile, error) {
	if err := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return nil, nil
	}
	refs, err := gitStdout(dir, "for-each-ref", "--format=%(symref)\t%(refname)", "refs/remotes")
	if err != nil {
		return nil, fmt.Errorf("list remote refs: %w", err)
	}
	var files []IssueFile
	for _, line := range strings.Split(strings.TrimSpace(refs), "\n") {
		// Symbolic refs such as origin/HEAD repeat another ref; skip them.
		symref, ref, _ := strings.Cut(line, "\t")
		if symref != "" || ref == "" {
			continue
		}
		listing, err := gitStdout(dir, "ls-tree", "-r", "-z", "--name-only", ref, "--", "issues")
		if err != nil {
			return nil, fmt.Errorf("list tickets on %s: %w", ref, err)
		}
		short := strings.TrimPrefix(ref, "refs/remotes/")
		for _, p := range strings.Split(listing, "\x00") {
			rel, ok := strings.CutPrefix(p, "issues/")
			if !ok {
				continue
			}
			// Same layout as Scan: tickets directly in issues/ or in issues/archive/.
			if dirPart := path.Dir(rel); dirPart != "." && dirPart != "archive" {
				continue
			}
			if m := issueFileRegex.FindStringSubmatch(path.Base(rel)); m != nil {
				files = append(files, IssueFile{Number: m[1], RelPath: rel, Ref: short})
			}
		}
	}
	return files, nil
}

// gitStdout runs git -C dir args and returns its raw standard output.
func gitStdout(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// location names the ticket file in messages: issues/<RelPath>, prefixed with
// the remote ref when the file is not in the working tree.
func (f IssueFile) location() string {
	if f.Ref != "" {
		return f.Ref + ":issues/" + f.RelPath
	}
	return "issues/" + f.RelPath
}
