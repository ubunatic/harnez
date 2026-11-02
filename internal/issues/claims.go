package issues

import (
	"fmt"
	"io"
	"os"
	"strings"

	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/prwait"
)

// ClaimOptions configures Claims.
type ClaimOptions struct {
	// Offline skips the forge API scan of open pull requests. Remote-tracking refs are still read.
	Offline bool
	// Warn receives one line per forge that could not be scanned. A nil Warn discards them.
	Warn io.Writer
}

// Claims returns the ticket files that other refs or open pull requests already claim, so a
// new number must avoid them. It merges two sources:
//   - remote-tracking refs (RemoteTicketFiles), read without fetching;
//   - unless Offline, the ticket files of open pull requests on every configured forge remote.
//
// Closed and merged pull requests are not listed: a merged ticket is on main, which the
// remote-tracking refs cover, and a closed-unmerged one is abandoned and reserves nothing.
// A forge that cannot be reached only warns; the result then lacks that forge's open PRs.
func Claims(dir string, opts ClaimOptions) ([]IssueFile, error) {
	claims, err := RemoteTicketFiles(dir)
	if err != nil {
		return nil, err
	}
	if opts.Offline {
		return claims, nil
	}
	spec, err := prwait.LoadSpec()
	if err != nil {
		return nil, fmt.Errorf("load forge spec: %w", err)
	}
	lister := forge.NewPullLister(spec.OpenPRScan, forge.SystemTokenSource(os.Getenv).For)
	return append(claims, openPRFiles(dir, spec, lister, opts.Warn)...), nil
}

// openPRFiles lists the ticket files of open pull requests on each remote whose host is a
// configured forge. A repository is scanned once even if two remotes point at it, and a file
// path counts once across remotes.
func openPRFiles(dir string, spec prwait.Spec, lister forge.PullLister, warn io.Writer) []IssueFile {
	if warn == nil {
		warn = io.Discard
	}
	names, err := gitStdout(dir, "remote")
	if err != nil {
		return nil // not a git work tree: there are no forges to ask
	}
	seenRepo := map[string]bool{}
	seenFile := map[string]bool{}
	var files []IssueFile
	for _, name := range strings.Fields(names) {
		rawURL, err := gitStdout(dir, "remote", "get-url", name)
		if err != nil {
			continue
		}
		host, owner, repo, ok := forge.ParseRemoteURL(rawURL)
		if !ok {
			continue
		}
		fg, ok := spec.ForgeFor(host)
		if !ok {
			continue
		}
		key := strings.ToLower(host + "/" + owner + "/" + repo)
		if seenRepo[key] {
			continue
		}
		seenRepo[key] = true
		pulls, err := lister.OpenPulls(fg, owner, repo, spec.OpenPRScan)
		if err != nil {
			fmt.Fprintf(warn, "open PRs not checked on %s (%v); a ticket number taken only by an open PR may collide\n", name, err)
			continue
		}
		for _, p := range pulls {
			ref := fmt.Sprintf("%s PR #%d", name, p.Number)
			for _, f := range ticketFilesFromPaths(p.Files, ref) {
				if seenFile[f.RelPath] {
					continue
				}
				seenFile[f.RelPath] = true
				files = append(files, f)
			}
		}
	}
	return files
}
