package maintenance

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"ubunatic.com/harnez/internal/index"
	issuecore "ubunatic.com/harnez/internal/issues"
)

// defaultReconcileRef is the merged parent when HEAD is a merge commit, else origin/main.
func defaultReconcileRef(dir string) string {
	if _, err := gitOutput(dir, "rev-parse", "--verify", "-q", "HEAD^2"); err == nil {
		return "HEAD^2"
	}
	return "origin/main"
}

// ReconcileOptions configures RunIssuesReconcileOpts.
type ReconcileOptions struct {
	Ref    string // ref merged into HEAD; default: merged parent, else origin/main
	DryRun bool   // print the plan only
	Fetch  bool   // git fetch, then print the plan against the upstream; implies DryRun
	Pull   bool   // git fetch + merge the upstream, then reconcile
}

// RunIssuesReconcileOpts optionally fetches or pulls the upstream first, then reconciles.
func RunIssuesReconcileOpts(w io.Writer, dir string, o ReconcileOptions) error {
	if o.Fetch && o.Pull {
		return fmt.Errorf("issues reconcile: --fetch and --pull are mutually exclusive")
	}
	if o.Fetch || o.Pull {
		if o.Ref != "" {
			return fmt.Errorf("issues reconcile: --fetch/--pull use the branch upstream; do not pass a ref")
		}
		if _, err := gitOutput(dir, "rev-parse", "--verify", "-q", "@{upstream}"); err != nil {
			return fmt.Errorf("issues reconcile: branch has no upstream configured")
		}
		if _, err := gitOutput(dir, "fetch"); err != nil {
			return fmt.Errorf("issues reconcile: %w", err)
		}
		o.Ref = "@{upstream}"
	}
	if o.Fetch {
		o.DryRun = true
	}
	if o.Pull {
		status, err := gitOutput(dir, "status", "--porcelain", "--untracked-files=no")
		if err != nil {
			return fmt.Errorf("issues reconcile: %w", err)
		}
		if status != "" {
			return fmt.Errorf("issues reconcile: working tree has tracked changes; commit or stash them before --pull")
		}
		mergeErr := func() error {
			_, err := gitOutput(dir, "merge", "--no-edit", "@{upstream}")
			return err
		}()
		if mergeErr != nil {
			if _, err := gitOutput(dir, "rev-parse", "--verify", "-q", "MERGE_HEAD"); err == nil {
				return fmt.Errorf("issues reconcile: merge stopped with conflicts; resolve them, commit, then run 'harnez issues reconcile': %w", mergeErr)
			}
			// A failing index hook (duplicate numbers) can still leave the merge commit; reconcile repairs it.
			if _, err := gitOutput(dir, "rev-parse", "--verify", "-q", "HEAD^2"); err != nil {
				return fmt.Errorf("issues reconcile: merge failed: %w", mergeErr)
			}
		}
		// After the merge, upstream..HEAD is exactly the local commits, also on a fast-forward.
		o.Ref = "@{upstream}"
	}
	return RunIssuesReconcile(w, dir, o.Ref, o.DryRun)
}

// RunIssuesReconcile renumbers this branch's own colliding tickets after a
// merge of ref, rewrites references, repairs the indexes and commits once.
// Ownership is shared with rebase: tickets added in ref..HEAD.
func RunIssuesReconcile(w io.Writer, dir, ref string, dryRun bool) error {
	if ref == "" {
		ref = defaultReconcileRef(dir)
	}
	status, err := porcelainStatus(dir)
	if err != nil {
		return fmt.Errorf("issues reconcile: %w", err)
	}
	if !dryRun && trackerDirty(status) {
		return fmt.Errorf("issues reconcile: tracker paths are dirty; commit or stash .gitattributes/issues changes and retry")
	}
	state, err := collectLocalTickets(dir, ref)
	if err != nil {
		return fmt.Errorf("issues reconcile: %w", err)
	}
	var present []rebaseOwnedTicket
	for _, o := range state.Owned {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(o.Path))); err == nil {
			present = append(present, o) // deleted or moved on the branch: nothing to renumber
		}
	}
	state.Owned = present
	max, collisions, err := plannedCollisions(dir, ref, state)
	if err != nil {
		return fmt.Errorf("issues reconcile: %w", err)
	}
	if len(collisions) == 0 {
		fmt.Fprintln(w, "issues reconcile: no collisions")
		return nil
	}
	issuesDir := filepath.Join(dir, "issues")
	var stage []string
	var undo []func()
	fail := func(err error) error {
		for i := len(undo) - 1; i >= 0; i-- {
			undo[i]()
		}
		return fmt.Errorf("issues reconcile: %w (changes rolled back; nothing committed)", err)
	}
	for i, owned := range collisions {
		newNumber := fmt.Sprintf("%03d", max+i+1)
		base := filepath.Base(owned.Path)
		newPath := filepath.ToSlash(filepath.Join(filepath.Dir(owned.Path), newNumber+strings.TrimPrefix(base, owned.Number)))
		edits, err := issuecore.PlanReferences(dir, owned.Path, newPath)
		if err != nil {
			return fail(fmt.Errorf("scan references: %w", err))
		}
		fmt.Fprintf(w, "%s\t%s -> %s\n", owned.Path, owned.Number, newNumber)
		for _, e := range edits {
			rel, _ := filepath.Rel(dir, e.Path)
			fmt.Fprintf(w, "  rewrite references in %s\n", filepath.ToSlash(rel))
		}
		if dryRun {
			continue
		}
		oldAbs := filepath.Join(dir, filepath.FromSlash(owned.Path))
		newAbs := filepath.Join(dir, filepath.FromSlash(newPath))
		content, err := os.ReadFile(oldAbs)
		if err != nil {
			return fail(err)
		}
		rewritten, err := issuecore.RewriteHeaderNumber(string(content), newNumber)
		if err != nil {
			return fail(fmt.Errorf("%s: %w", owned.Path, err))
		}
		rewritten = issuecore.RewriteReferences(rewritten, owned.Path, newPath)
		f, err := os.OpenFile(newAbs, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fail(fmt.Errorf("claim %s: %w", newPath, err))
		}
		_, werr := f.WriteString(rewritten)
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			_ = os.Remove(newAbs)
			return fail(fmt.Errorf("write %s: %w", newPath, werr))
		}
		if err := os.Remove(oldAbs); err != nil {
			_ = os.Remove(newAbs)
			return fail(err)
		}
		undo = append(undo, func() { _ = os.WriteFile(oldAbs, content, 0o644); _ = os.Remove(newAbs) })
		if err := issuecore.WriteReferences(edits, false); err != nil {
			undo = append(undo, func() { _ = issuecore.WriteReferences(edits, true) })
			return fail(fmt.Errorf("rewrite references: %w", err))
		}
		undo = append(undo, func() { _ = issuecore.WriteReferences(edits, true) })
		stage = append(stage, owned.Path, newPath)
		for _, e := range edits {
			rel, _ := filepath.Rel(dir, e.Path)
			stage = append(stage, filepath.ToSlash(rel))
		}
	}
	if dryRun {
		return nil
	}
	snap, snapErr := SnapshotIssueIndexes(issuesDir)
	if snapErr != nil {
		return fail(snapErr)
	}
	undo = append(undo, func() { _ = RestoreIssueIndexes(issuesDir, snap) })
	if _, err := index.UpdateIssuesReadme(filepath.Join(issuesDir, "README.md"), issuesDir); err != nil {
		return fail(fmt.Errorf("update README: %w", err))
	}
	stage = append(stage, issueIndexRelPaths()...)
	if _, err := gitOutput(dir, append([]string{"add", "-A", "--"}, stage...)...); err != nil {
		_, _ = gitOutput(dir, append([]string{"reset", "-q", "--"}, stage...)...)
		return fail(err)
	}
	commit := append([]string{"commit", "--only", "-m", "docs(issues): reconcile ticket numbers after merge", "--"}, stage...)
	if _, err := gitOutput(dir, commit...); err != nil {
		_, _ = gitOutput(dir, append([]string{"reset", "-q", "--"}, stage...)...)
		return fail(err)
	}
	if err := runIssuesLint(w, dir); err != nil {
		return fmt.Errorf("issues reconcile: committed, but lint still reports problems: %w", err)
	}
	return nil
}
