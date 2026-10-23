package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/index"
	"ubunatic.com/harnez/internal/issues"
)

type rebaseOwnedTicket struct {
	Path   string `json:"path"`
	Number string `json:"number"`
}

type rebaseState struct {
	Upstream string              `json:"upstream"`
	Head     string              `json:"head"`
	Owned    []rebaseOwnedTicket `json:"owned"`
	Phase    string              `json:"phase"`
	Repairs  []rebaseRepair      `json:"repairs"`
	Staged   string              `json:"staged_patch,omitempty"`
}

type rebaseRepair struct {
	OldPath   string `json:"old_path"`
	NewPath   string `json:"new_path"`
	OldNumber string `json:"old_number"`
	NewNumber string `json:"new_number"`
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func rebaseStatePath(dir string) (string, error) {
	common, err := gitOutput(dir, "rev-parse", "--git-path", "harnez/issues-rebase.json")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(dir, common)
	}
	return filepath.Join(filepath.Clean(common), "harnez", "issues-rebase.json"), nil
}

func writeRebaseState(path string, state rebaseState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readRebaseState(path string) (rebaseState, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return rebaseState{}, false, nil
	}
	if err != nil {
		return rebaseState{}, false, err
	}
	var state rebaseState
	if err := json.Unmarshal(data, &state); err != nil {
		return rebaseState{}, false, err
	}
	return state, true, nil
}

func rebaseInProgress(dir string) bool {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		path, err := gitOutput(dir, "rev-parse", "--git-path", name)
		if err == nil {
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, path)
			}
			if _, statErr := os.Stat(path); statErr == nil {
				return true
			}
		}
	}
	return false
}

func trackerDirty(status string) bool {
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		path := strings.TrimSpace(line[3:])
		if arrow := strings.LastIndex(path, " -> "); arrow >= 0 {
			path = path[arrow+4:]
		}
		if path == ".gitattributes" || path == "issues" || strings.HasPrefix(path, "issues/") {
			return true
		}
	}
	return false
}

func collectLocalTickets(dir, upstream string) (rebaseState, error) {
	head, err := gitOutput(dir, "rev-parse", "HEAD")
	if err != nil {
		return rebaseState{}, err
	}
	commits, err := gitOutput(dir, "rev-list", "--reverse", upstream+"..HEAD")
	if err != nil {
		return rebaseState{}, fmt.Errorf("resolve upstream %q: %w", upstream, err)
	}
	state := rebaseState{Upstream: upstream, Head: head}
	seen := map[string]bool{}
	for _, commit := range strings.Fields(commits) {
		paths, err := gitOutput(dir, "diff-tree", "--no-commit-id", "--name-only", "--diff-filter=A", "-r", commit, "--", "issues")
		if err != nil {
			return rebaseState{}, err
		}
		for _, path := range strings.Fields(paths) {
			base := filepath.Base(path)
			if path == "issues/README.md" || !strings.HasSuffix(base, ".md") || len(base) < 4 || base[3] != '-' {
				continue
			}
			if _, err := strconv.Atoi(base[:3]); err != nil || seen[path] {
				continue
			}
			seen[path] = true
			state.Owned = append(state.Owned, rebaseOwnedTicket{Path: path, Number: base[:3]})
		}
	}
	return state, nil
}

func runIssuesRebase(w io.Writer, dir, upstream string, dryRun bool) error {
	if _, err := gitOutput(dir, "symbolic-ref", "-q", "HEAD"); err != nil {
		return fmt.Errorf("issues rebase: detached HEAD is unsupported; check out a branch and retry")
	}
	driver, err := gitOutput(dir, "config", "--local", "--get", "merge.harnez-issues-index.driver")
	if err != nil || strings.TrimSpace(driver) == "" {
		return fmt.Errorf("issues rebase: issue Git integration is not enabled; run 'harnez init --issues-git' and retry")
	}
	attribute, err := gitOutput(dir, "check-attr", "merge", "--", "issues/README.md")
	if err != nil || !strings.HasSuffix(attribute, ": harnez-issues-index") {
		return fmt.Errorf("issues rebase: issue Git integration is not enabled; run 'harnez init --issues-git' and retry")
	}
	statePath, err := rebaseStatePath(dir)
	if err != nil {
		return fmt.Errorf("issues rebase: %w", err)
	}
	state, resume, err := readRebaseState(statePath)
	if err != nil {
		return fmt.Errorf("issues rebase: read recovery state: %w", err)
	}
	if resume {
		upstream = state.Upstream
		if rebaseInProgress(dir) {
			return fmt.Errorf("issues rebase: replay is still in progress; resolve the substantive conflict and run 'git rebase --continue', or run 'git rebase --abort'; then rerun this command")
		}
		currentHead, headErr := gitOutput(dir, "rev-parse", "HEAD")
		if headErr != nil {
			return fmt.Errorf("issues rebase: inspect recovery HEAD: %w", headErr)
		}
		if state.Phase == "replay" && currentHead != state.Head {
			state.Phase = "repair"
			if err := writeRebaseState(statePath, state); err != nil {
				return fmt.Errorf("issues rebase: advance recovery state: %w", err)
			}
		}
	}
	status, err := gitOutput(dir, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("issues rebase: %w", err)
	}
	if resume && state.Phase == "restore" {
		if err := restoreStagedChanges(dir, state); err != nil {
			return fmt.Errorf("issues rebase: staged-state recovery failed; state retained at %s: %w", statePath, err)
		}
		if err := runIssuesLint(w, dir); err != nil {
			return err
		}
		return os.Remove(statePath)
	}
	if resume && state.Phase == "repair" {
		if err := repairRebasedTickets(w, dir, state, statePath); err != nil {
			return fmt.Errorf("issues rebase: repair retry failed; state retained at %s: %w", statePath, err)
		}
		return os.Remove(statePath)
	}
	if trackerDirty(status) {
		return fmt.Errorf("issues rebase: tracker paths are dirty; commit or stash .gitattributes/issues changes and retry (unrelated changes are preserved automatically)")
	}
	if !resume {
		state, err = collectLocalTickets(dir, upstream)
		if err != nil {
			return fmt.Errorf("issues rebase: %w", err)
		}
		stagedCmd := exec.Command("git", "-C", dir, "diff", "--cached", "--binary")
		staged, stagedErr := stagedCmd.Output()
		if stagedErr != nil {
			return fmt.Errorf("issues rebase: capture staged changes: %w", stagedErr)
		}
		state.Staged = string(staged)
	}
	max, collisions, err := plannedCollisions(dir, upstream, state)
	if err != nil {
		return fmt.Errorf("issues rebase: %w", err)
	}
	for i, owned := range collisions {
		fmt.Fprintf(w, "%s\t%s -> %03d\n", owned.Path, owned.Number, max+i+1)
		if !resume {
			newNumber := fmt.Sprintf("%03d", max+i+1)
			base := filepath.Base(owned.Path)
			state.Repairs = append(state.Repairs, rebaseRepair{
				OldPath: owned.Path, NewPath: filepath.ToSlash(filepath.Join(filepath.Dir(owned.Path), newNumber+strings.TrimPrefix(base, owned.Number))),
				OldNumber: owned.Number, NewNumber: newNumber,
			})
		}
	}
	if dryRun {
		fmt.Fprintf(w, "next: git rebase %s, then harnez repairs and commits the listed paths\n", upstream)
		return nil
	}
	state.Phase = "replay"
	if err := writeRebaseState(statePath, state); err != nil {
		return fmt.Errorf("issues rebase: persist recovery state: %w", err)
	}
	rebaseArgs := []string{"-C", dir, "rebase"}
	if status != "" {
		rebaseArgs = append(rebaseArgs, "--autostash")
	}
	rebaseArgs = append(rebaseArgs, upstream)
	cmd := exec.Command("git", rebaseArgs...)
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("issues rebase: replay stopped; resolve the substantive conflict and run git rebase --continue, or recover with git rebase --abort (state retained at %s): %w", statePath, err)
	}
	state.Phase = "repair"
	if err := writeRebaseState(statePath, state); err != nil {
		return fmt.Errorf("issues rebase: persist repair state: %w", err)
	}
	if err := repairRebasedTickets(w, dir, state, statePath); err != nil {
		return fmt.Errorf("issues rebase: replay completed but repair failed; state retained at %s; fix the diagnostic and rerun 'harnez issues rebase %s': %w", statePath, upstream, err)
	}
	if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("issues rebase: remove completed state: %w", err)
	}
	return nil
}

func plannedCollisions(dir, upstream string, state rebaseState) (int, []rebaseOwnedTicket, error) {
	files, err := issues.Scan(filepath.Join(dir, "issues"))
	if err != nil {
		return 0, nil, err
	}
	max := 0
	counts := map[string]int{}
	for _, f := range files {
		n, _ := strconv.Atoi(f.Number)
		if n > max {
			max = n
		}
		counts[f.Number]++
	}
	remote, err := gitOutput(dir, "ls-tree", "-r", "--name-only", upstream, "--", "issues")
	if err != nil {
		return 0, nil, err
	}
	for _, p := range strings.Fields(remote) {
		base := filepath.Base(p)
		if len(base) >= 4 && base[3] == '-' {
			if n, e := strconv.Atoi(base[:3]); e == nil {
				if n > max {
					max = n
				}
				counts[base[:3]]++
			}
		}
	}
	var collisions []rebaseOwnedTicket
	for _, owned := range state.Owned {
		if counts[owned.Number] > 1 {
			collisions = append(collisions, owned)
		}
	}
	return max, collisions, nil
}

func repairRebasedTickets(w io.Writer, dir string, state rebaseState, statePath string) error {
	var stage []string
	mutated := false
	for _, repair := range state.Repairs {
		old := filepath.Join(dir, filepath.FromSlash(repair.OldPath))
		newPath := filepath.Join(dir, filepath.FromSlash(repair.NewPath))
		oldInfo, oldErr := os.Stat(old)
		newInfo, newErr := os.Stat(newPath)
		if oldErr != nil && !os.IsNotExist(oldErr) {
			return oldErr
		}
		if newErr != nil && !os.IsNotExist(newErr) {
			return newErr
		}
		if oldInfo == nil && newInfo != nil {
			content, readErr := os.ReadFile(newPath)
			if readErr != nil || !strings.HasPrefix(string(content), "# "+repair.NewNumber+" ") {
				return fmt.Errorf("repair destination %s is incomplete or has the wrong header", repair.NewPath)
			}
			stage = append(stage, repair.OldPath, repair.NewPath)
			continue
		}
		if oldInfo == nil {
			return fmt.Errorf("repair source and destination are both missing for %s -> %s; restore the source and retry", repair.OldPath, repair.NewPath)
		}
		content, err := os.ReadFile(old)
		if err != nil {
			return err
		}
		rewritten, err := issues.RewriteHeaderNumber(string(content), repair.NewNumber)
		if err != nil {
			return fmt.Errorf("%s: %w", repair.OldPath, err)
		}
		tempID := sha256.Sum256([]byte(state.Head + "\x00" + repair.OldPath + "\x00" + repair.NewPath))
		tmpPath := fmt.Sprintf("%s.harnez-%x.tmp", newPath, tempID[:6])
		if newInfo != nil {
			existing, readErr := os.ReadFile(newPath)
			if readErr == nil && string(existing) == rewritten {
				if err := os.Remove(old); err != nil {
					return err
				}
				_ = os.Remove(tmpPath)
				mutated = true
				stage = append(stage, repair.OldPath, repair.NewPath)
				continue
			}
			return fmt.Errorf("repair destination %s was claimed with unexpected content; refusing to overwrite it", repair.NewPath)
		}
		tmpContent, tmpErr := os.ReadFile(tmpPath)
		if tmpErr == nil {
			if string(tmpContent) != rewritten {
				return fmt.Errorf("transaction temp %s has unexpected content; refusing to replace it", tmpPath)
			}
		} else if !os.IsNotExist(tmpErr) {
			return tmpErr
		} else {
			scratchPattern := tmpPath + ".scratch-*"
			staleScratch, _ := filepath.Glob(scratchPattern)
			for _, scratch := range staleScratch {
				_ = os.Remove(scratch)
			}
			fd, createErr := os.CreateTemp(filepath.Dir(tmpPath), filepath.Base(tmpPath)+".scratch-")
			if createErr != nil {
				return fmt.Errorf("create transaction scratch for %s: %w", repair.NewPath, createErr)
			}
			scratchPath := fd.Name()
			if _, createErr = fd.WriteString(rewritten); createErr != nil {
				fd.Close()
				_ = os.Remove(scratchPath)
				return createErr
			}
			if createErr = fd.Close(); createErr != nil {
				_ = os.Remove(scratchPath)
				return createErr
			}
			if createErr = os.Link(scratchPath, tmpPath); createErr != nil {
				_ = os.Remove(scratchPath)
				return fmt.Errorf("publish transaction temp for %s: %w", repair.NewPath, createErr)
			}
			_ = os.Remove(scratchPath)
		}
		if err := os.Link(tmpPath, newPath); err != nil {
			return fmt.Errorf("atomically claim %s without overwrite: %w", repair.NewPath, err)
		}
		if err := os.Remove(tmpPath); err != nil {
			return fmt.Errorf("remove completed transaction temp %s: %w", tmpPath, err)
		}
		if err := os.Remove(old); err != nil {
			return err
		}
		mutated = true
		fmt.Fprintf(w, "renumbered %s -> %s\n", repair.OldPath, repair.NewPath)
		stage = append(stage, repair.OldPath, repair.NewPath)
	}
	readme := filepath.Join(dir, "issues", "README.md")
	changed, err := index.UpdateIssuesReadme(readme, filepath.Join(dir, "issues"))
	if err != nil {
		return err
	}
	if len(state.Repairs) == 0 && !changed {
		fmt.Fprintln(w, "issues rebase: no repair needed")
		return nil
	}
	stage = append(stage, issueIndexRelPaths()...)
	if !mutated && !changed {
		pathStatus, err := gitOutput(dir, append([]string{"status", "--porcelain", "--"}, stage...)...)
		if err != nil {
			return err
		}
		if pathStatus == "" {
			return runIssuesLint(w, dir)
		}
	}
	if _, err := gitOutput(dir, append([]string{"add", "--"}, stage...)...); err != nil {
		return err
	}
	commitArgs := []string{"commit", "--only", "-m", "docs(issues): repair ticket numbers after rebase", "--"}
	commitArgs = append(commitArgs, stage...)
	if _, err := gitOutput(dir, commitArgs...); err != nil {
		return err
	}
	state.Phase = "restore"
	if err := writeRebaseState(statePath, state); err != nil {
		return fmt.Errorf("persist staged-state recovery phase: %w", err)
	}
	if err := restoreStagedChanges(dir, state); err != nil {
		return err
	}
	return runIssuesLint(w, dir)
}

func restoreStagedChanges(dir string, state rebaseState) error {
	cmd := exec.Command("git", "-C", dir, "diff", "--cached", "--binary")
	current, err := cmd.Output()
	if err != nil {
		return err
	}
	if string(current) == state.Staged || state.Staged == "" {
		return nil
	}
	if len(current) != 0 {
		return fmt.Errorf("index contains unexpected staged changes; recovery state retained")
	}
	cmd = exec.Command("git", "-C", dir, "apply", "--cached", "--whitespace=nowarn", "-")
	cmd.Stdin = strings.NewReader(state.Staged)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restore unrelated staged changes: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func runIssuesLint(w io.Writer, dir string) error {
	return runIssuesLintMode(w, dir, false, nil)
}

type issueIndexSnapshot map[string]fileSnapshot

type fileSnapshot struct {
	content []byte
	existed bool
}

func issueIndexRelPaths() []string {
	return []string{"issues/README.md", "issues/BACKLOG.md", "issues/CLOSED.md"}
}

func snapshotIssueIndexes(issuesDir string) (issueIndexSnapshot, error) {
	snapshot := make(issueIndexSnapshot, 3)
	for _, name := range []string{"README.md", "BACKLOG.md", "CLOSED.md"} {
		content, err := os.ReadFile(filepath.Join(issuesDir, name))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		snapshot[name] = fileSnapshot{content: content, existed: err == nil}
	}
	return snapshot, nil
}

func restoreIssueIndexes(issuesDir string, snapshot issueIndexSnapshot) error {
	for name, file := range snapshot {
		path := filepath.Join(issuesDir, name)
		if file.existed {
			if err := os.WriteFile(path, file.content, 0o644); err != nil {
				return err
			}
		} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func runIssuesLintMode(w io.Writer, dir string, cached bool, untrackedTickets []string) error {
	issuesDir := filepath.Join(dir, "issues")
	files, err := issues.Scan(issuesDir)
	if err != nil {
		return fmt.Errorf("issues lint: %w", err)
	}
	seen := map[string]string{}
	for _, f := range files {
		if f.Canonical == issues.StatusUnknown {
			return fmt.Errorf("issues lint: %s has unknown status %q and cannot appear in the three lifecycle indexes", f.RelPath, f.RawStatus)
		}
		if prior := seen[f.Number]; prior != "" {
			return fmt.Errorf("issues lint: duplicate ticket number %s: %s and %s", f.Number, prior, f.RelPath)
		}
		seen[f.Number] = f.RelPath
		content, err := os.ReadFile(filepath.Join(issuesDir, f.RelPath))
		if err != nil {
			return err
		}
		title, _, _ := issues.ParseIssueFile(string(content))
		fields := strings.Fields(title)
		if len(fields) > 0 {
			_, parseErr := issues.NormalizeID(fields[0])
			if parseErr == nil && fields[0] != f.Number {
				return fmt.Errorf("issues lint: filename/header mismatch in issues/%s (header %q)", f.RelPath, title)
			}
		}
	}
	readme := filepath.Join(issuesDir, "README.md")
	orig, err := os.ReadFile(readme)
	if err != nil {
		return fmt.Errorf("issues lint: read index: %w", err)
	}
	generated, err := index.RenderIssuesReadme(string(orig), issuesDir, readme)
	if err != nil {
		return fmt.Errorf("issues lint: %w", err)
	}
	if generated != string(orig) {
		if cached {
			if len(untrackedTickets) > 0 {
				return fmt.Errorf("issues lint --cached: generated index differs from the staged ticket set; untracked ticket files are not part of this check: %s; stage or remove them, then regenerate issues/README.md", strings.Join(untrackedTickets, ", "))
			}
			return fmt.Errorf("issues lint --cached: generated index differs from the staged ticket set; inspect the staged ticket files and regenerate issues/README.md")
		}
		return fmt.Errorf("issues lint: issues/README.md is generated index drift; run 'harnez index'")
	}
	indexes, err := index.IssueIndexFiles(issuesDir)
	if err != nil {
		return fmt.Errorf("issues lint: %w", err)
	}
	for _, name := range []string{"BACKLOG.md", "CLOSED.md"} {
		path := filepath.Join(issuesDir, name)
		actual, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("issues lint: read index %s: %w", path, err)
		}
		if string(actual) != indexes[name] {
			return fmt.Errorf("issues lint: issues/%s is generated index drift; run 'harnez index'", name)
		}
	}
	docsReadme := filepath.Join(dir, "docs", "README.md")
	docsContent, err := os.ReadFile(docsReadme)
	if err == nil {
		generated, err := index.RenderDocsReadme(string(docsContent), filepath.Join(dir, "docs"), docsReadme)
		if err != nil {
			return fmt.Errorf("issues lint: %w", err)
		}
		if generated != string(docsContent) {
			return fmt.Errorf("issues lint: docs/README.md is generated studies index drift; run 'harnez index'")
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("issues lint: read docs index: %w", err)
	}
	fmt.Fprintln(w, "issues tracker valid")
	if err := warnActiveTicketCap(w, issuesDir); err != nil {
		return fmt.Errorf("issues lint: %w", err)
	}
	return nil
}

func warnActiveTicketCap(w io.Writer, issuesDir string) error {
	cap, source, err := claude.ActiveIssueCapForRepo(filepath.Dir(issuesDir))
	if err != nil {
		return err
	}
	open, _, draft, _, _, err := index.StatusCounts(issuesDir)
	if err != nil {
		return err
	}
	active := open + draft
	if active > cap {
		fmt.Fprintf(w, "warning: %d active tickets exceeds configured cap %d (%s)\n", active, cap, source)
	}
	return nil
}

func runIssuesStatus(w io.Writer, dir string) error {
	issuesDir := filepath.Join(dir, "issues")
	open, closed, draft, backlog, unknown, err := index.StatusCounts(issuesDir)
	if err != nil {
		return fmt.Errorf("issues status: %w", err)
	}
	fmt.Fprintf(w, "open=%d draft=%d backlog=%d closed=%d unknown=%d\n", open, draft, backlog, closed, unknown)
	return warnActiveTicketCap(w, issuesDir)
}

func runIssuesLintCached(w io.Writer, dir string) error {
	tmp, err := os.MkdirTemp("", "harnez-issues-index-")
	if err != nil {
		return fmt.Errorf("issues lint --cached: %w", err)
	}
	defer os.RemoveAll(tmp)
	prefix := tmp + string(os.PathSeparator)
	if _, err := gitOutput(dir, "checkout-index", "--all", "--prefix="+prefix); err != nil {
		return fmt.Errorf("issues lint --cached: materialize index: %w", err)
	}
	status, _ := gitOutput(dir, "status", "--short", "--untracked-files=all")
	var untrackedTickets []string
	for _, line := range strings.Split(status, "\n") {
		if strings.HasPrefix(line, "?? ") && strings.HasPrefix(strings.TrimSpace(line[3:]), "issues/") {
			untrackedTickets = append(untrackedTickets, strings.TrimSpace(line[3:]))
		}
	}
	return runIssuesLintMode(w, tmp, true, untrackedTickets)
}

// runIssuesMergeDriver retains the two-path helper used by unit tests.
func runIssuesMergeDriver(currentPath, targetPath string) error {
	return runIssuesMergeDriverVersions("", currentPath, "", targetPath)
}

// runIssuesMergeDriverVersions rebuilds a generated index from its source
// files. For partial files it refuses a clean merge when both sides changed
// the hand-written content outside the generated block.
func runIssuesMergeDriverVersions(ancestorPath, currentPath, otherPath, targetPath string) error {
	if strings.TrimSpace(currentPath) == "" {
		return fmt.Errorf("issues merge-driver: Git did not provide the current-version path")
	}
	orig, err := os.ReadFile(currentPath)
	if err != nil {
		return fmt.Errorf("issues merge-driver: current version %s: %w", currentPath, err)
	}
	root, err := gitOutput(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("issues merge-driver: locate worktree: %w", err)
	}
	target := filepath.ToSlash(filepath.Clean(filepath.FromSlash(targetPath)))
	if filepath.IsAbs(target) || target == ".." || strings.HasPrefix(target, "../") {
		return fmt.Errorf("issues merge-driver: invalid target path %q", targetPath)
	}
	issuesDir := filepath.Join(root, "issues")
	var ancestor, other string
	if ancestorPath != "" && otherPath != "" {
		ancestorBytes, readErr := os.ReadFile(ancestorPath)
		if readErr != nil {
			return fmt.Errorf("issues merge-driver: common ancestor %s: %w", ancestorPath, readErr)
		}
		otherBytes, readErr := os.ReadFile(otherPath)
		if readErr != nil {
			return fmt.Errorf("issues merge-driver: other version %s: %w", otherPath, readErr)
		}
		ancestor, other = string(ancestorBytes), string(otherBytes)
	}
	var generated string
	switch target {
	case "issues/README.md":
		if index.HasConflictOutsideManagedBlock(string(orig), "issues-active-index") {
			return fmt.Errorf("issues merge-driver: unresolved conflict outside generated block in %s", target)
		}
		if ancestorPath != "" && otherPath != "" {
			generated, err = index.MergeIssuesReadme(ancestor, string(orig), other)
		} else {
			generated, err = index.RenderIssuesReadme(string(orig), issuesDir, target)
		}
	case "issues/BACKLOG.md":
		fallthrough
	case "issues/CLOSED.md":
		if ancestorPath != "" && otherPath != "" {
			generated, err = index.MergeFullIssueIndex(ancestor, string(orig), other, filepath.Base(target))
		} else {
			var indexes map[string]string
			indexes, err = index.IssueIndexFiles(issuesDir)
			generated = indexes[filepath.Base(target)]
		}
	case "docs/README.md":
		if ancestorPath != "" && otherPath != "" {
			if index.HasConcurrentEditsOutsideManagedBlock(ancestor, string(orig), other, "docs-studies-index") {
				if !index.HasConflictOutsideManagedBlock(string(orig), "docs-studies-index") {
					if writeErr := writeMergeConflictMarkers(currentPath, string(orig), other); writeErr != nil {
						return fmt.Errorf("issues merge-driver: preserve hand-written conflict in %s: %w", target, writeErr)
					}
				}
				return fmt.Errorf("issues merge-driver: conflicting hand-written content outside generated block in %s", target)
			}
		}
		if index.HasConflictOutsideManagedBlock(string(orig), "docs-studies-index") {
			return fmt.Errorf("issues merge-driver: unresolved conflict outside generated block in %s", target)
		}
		if ancestorPath != "" && otherPath != "" {
			generated, err = index.MergeDocsReadme(ancestor, string(orig), other, filepath.Join(root, "docs"), target)
		} else {
			generated, err = index.RenderDocsReadme(string(orig), filepath.Join(root, "docs"), target)
		}
	default:
		return fmt.Errorf("issues merge-driver: unsupported generated path %q", target)
	}
	if err != nil {
		return fmt.Errorf("issues merge-driver: regenerate %s: %w", target, err)
	}
	if err := os.WriteFile(currentPath, []byte(generated), 0o644); err != nil {
		return fmt.Errorf("issues merge-driver: write %s: %w", currentPath, err)
	}
	return nil
}

func writeMergeConflictMarkers(path, ours, theirs string) error {
	conflict := "<<<<<<< ours\n" + strings.TrimRight(ours, "\r\n") + "\n=======\n" + strings.TrimRight(theirs, "\r\n") + "\n>>>>>>> theirs\n"
	return os.WriteFile(path, []byte(conflict), 0o644)
}
