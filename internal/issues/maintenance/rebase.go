package maintenance

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/gitflow"
	"ubunatic.com/harnez/internal/index"
	issuecore "ubunatic.com/harnez/internal/issues"
)

type rebaseOwnedTicket struct {
	Path   string `json:"path"`
	Number string `json:"number"`
	Theirs string `json:"theirs,omitempty"` // the upstream file holding the same number
}

// localTicketFileRegex matches a local ticket file name, of any number of digits
// from three up, as issues.Scan does. GH tickets have no local number to repair.
var localTicketFileRegex = regexp.MustCompile(`^(\d{3,})-.*\.md$`)

func localTicketNumber(path string) (string, bool) {
	match := localTicketFileRegex.FindStringSubmatch(filepath.Base(path))
	if match == nil {
		return "", false
	}
	return match[1], true
}

type rebaseState struct {
	Upstream string              `json:"upstream"`
	Head     string              `json:"head"`
	Owned    []rebaseOwnedTicket `json:"owned"`
	Phase    string              `json:"phase"`
	Repairs  []rebaseRepair      `json:"repairs"`
	Staged   string              `json:"staged_patch,omitempty"`
	// RefPaths lists files whose references to a moved ticket were rewritten, so
	// a retry after a failed commit still stages them.
	RefPaths []string `json:"ref_paths,omitempty"`
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

// GitOutput runs git in dir and returns trimmed standard output.
func GitOutput(dir string, args ...string) (string, error) { return gitOutput(dir, args...) }

func rebaseStatePath(dir string) (string, error) {
	path, err := gitOutput(dir, "rev-parse", "--git-path", "harnez/issues-rebase.json")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(dir, path)
	}
	path = filepath.Clean(path)
	if err := migrateLegacyRebaseState(path); err != nil {
		return "", fmt.Errorf("migrate recovery state: %w", err)
	}
	return path, nil
}

// migrateLegacyRebaseState handles checkouts from when the state file lived at
// <path>/harnez/issues-rebase.json, which left a directory at path.
func migrateLegacyRebaseState(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return nil
	}
	legacy := filepath.Join(path, "harnez", "issues-rebase.json")
	tmp := path + ".migrate"
	hasState := true
	if err := os.Rename(legacy, tmp); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		hasState = false
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	if hasState {
		return os.Rename(tmp, path)
	}
	return nil
}

// rebaseReplayActive reports whether `issues rebase` is replaying commits in the
// worktree at dir, including after the user resumed with `git rebase --continue`.
func rebaseReplayActive(dir string) bool {
	if !rebaseInProgress(dir) {
		return false
	}
	path, err := rebaseStatePath(dir)
	if err != nil {
		return false
	}
	state, found, err := readRebaseState(path)
	return err == nil && found && state.Phase == "replay"
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

// porcelainStatus returns untrimmed `git status --porcelain` output. gitOutput
// trims, which would eat the leading space of " M" on the first line and make
// trackerDirty misparse that path.
func porcelainStatus(dir string) (string, error) {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}
	return string(out), nil
}

func trackerDirty(status string) bool {
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 4 {
			continue
		}
		// A rename names both paths, and either side can be in the tracker.
		for _, path := range strings.Split(strings.TrimSpace(line[3:]), " -> ") {
			if path == "issues/README.md.lock" {
				continue // index sidecar lock, never tracker content
			}
			if path == ".gitattributes" || path == "issues" || strings.HasPrefix(path, "issues/") {
				return true
			}
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
			number, ok := localTicketNumber(path)
			if !ok || seen[path] {
				continue
			}
			seen[path] = true
			state.Owned = append(state.Owned, rebaseOwnedTicket{Path: path, Number: number})
		}
	}
	return state, nil
}

func runIssuesRebase(w io.Writer, dir, upstream string, dryRun bool) error {
	if _, err := gitOutput(dir, "symbolic-ref", "-q", "HEAD"); err != nil {
		return fmt.Errorf("issues rebase: detached HEAD is unsupported; check out a branch and retry")
	}
	attribute, err := gitOutput(dir, "check-attr", "merge", "--", "issues/README.md")
	isUnion := err == nil && (strings.HasSuffix(attribute, ": union") || strings.Contains(attribute, "merge: union"))
	isCustom := err == nil && (strings.HasSuffix(attribute, ": harnez-issues-index") || strings.Contains(attribute, "merge: harnez-issues-index"))
	if !isUnion && !isCustom {
		return fmt.Errorf("issues rebase: issue Git integration is not enabled; run 'harnez init --issues-git' and retry")
	}
	if isCustom {
		driver, err := gitOutput(dir, "config", "--local", "--get", "merge.harnez-issues-index.driver")
		if err != nil || strings.TrimSpace(driver) == "" {
			return fmt.Errorf("issues rebase: issue Git integration is not enabled; run 'harnez init --issues-git' and retry")
		}
	}
	if !dryRun {
		// Every path that later commits repairs runs through here first.
		if err := gitflow.GuardCommit(dir, "repair tickets"); err != nil {
			return fmt.Errorf("issues rebase: %w", err)
		}
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
	status, err := porcelainStatus(dir)
	if err != nil {
		return fmt.Errorf("issues rebase: %w", err)
	}
	if dryRun && resume && (state.Phase == "restore" || state.Phase == "repair") {
		fmt.Fprintf(w, "next: a rebase repair is unfinished (phase %s); run 'harnez issues rebase' without --dry-run to finish it\n", state.Phase)
		return nil
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
		newNumber := fmt.Sprintf("%03d", max+i+1)
		base := filepath.Base(owned.Path)
		newPath := filepath.ToSlash(filepath.Join(filepath.Dir(owned.Path), newNumber+strings.TrimPrefix(base, owned.Number)))
		fmt.Fprintf(w, "%s\t%s -> %s\tcollides with %s\n", owned.Path, owned.Number, newNumber, owned.Theirs)
		edits, err := issuecore.PlanReferences(dir, owned.Path, newPath)
		if err != nil {
			return fmt.Errorf("issues rebase: scan references: %w", err)
		}
		for _, e := range edits {
			rel, _ := filepath.Rel(dir, e.Path)
			fmt.Fprintf(w, "  rewrite references in %s\n", filepath.ToSlash(rel))
		}
		if !resume {
			state.Repairs = append(state.Repairs, rebaseRepair{
				OldPath: owned.Path, NewPath: newPath,
				OldNumber: owned.Number, NewNumber: newNumber,
			})
		}
	}
	if err := warnMovedNumbersInSubjects(w, dir, upstream, collisions); err != nil {
		return fmt.Errorf("issues rebase: %w", err)
	}
	if dryRun {
		if len(collisions) == 0 {
			fmt.Fprintf(w, "next: git rebase %s; no ticket numbers collide\n", upstream)
			return nil
		}
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

// warnMovedNumbersInSubjects names replayed commits whose subject mentions a
// number that moves. Rewriting a subject would change history beyond the rebase
// the user asked for, so this only says so.
func warnMovedNumbersInSubjects(w io.Writer, dir, upstream string, collisions []rebaseOwnedTicket) error {
	if len(collisions) == 0 {
		return nil
	}
	log, err := gitOutput(dir, "log", "--reverse", "--format=%h %s", upstream+"..HEAD")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(log, "\n") {
		for _, owned := range collisions {
			if regexp.MustCompile(`\b` + owned.Number + `\b`).MatchString(line[strings.IndexByte(line, ' ')+1:]) {
				fmt.Fprintf(w, "warning: commit subject names moved number %s and is not rewritten: %s\n", owned.Number, line)
			}
		}
	}
	return nil
}

func plannedCollisions(dir, upstream string, state rebaseState) (int, []rebaseOwnedTicket, error) {
	files, err := issuecore.Scan(filepath.Join(dir, "issues"))
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
	ownedPaths := map[string]bool{}
	for _, owned := range state.Owned {
		ownedPaths[owned.Path] = true
	}
	theirs := map[string]string{}
	for _, p := range strings.Fields(remote) {
		number, ok := localTicketNumber(p)
		if !ok {
			continue
		}
		if n, e := strconv.Atoi(number); e == nil && n > max {
			max = n
		}
		if ownedPaths[p] {
			continue // the same file on both sides (cherry-pick, squash) is no collision
		}
		counts[number]++
		theirs[number] = p
	}
	var collisions []rebaseOwnedTicket
	for _, owned := range state.Owned {
		if counts[owned.Number] > 1 {
			owned.Theirs = theirs[owned.Number]
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
		rewritten, err := issuecore.RewriteHeaderNumber(string(content), repair.NewNumber)
		if err != nil {
			return fmt.Errorf("%s: %w", repair.OldPath, err)
		}
		rewritten = issuecore.RewriteReferences(rewritten, repair.OldPath, repair.NewPath)
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
	refPaths, err := rewriteRepairReferences(w, dir, &state, statePath)
	if err != nil {
		return err
	}
	stage = append(stage, refPaths...)
	readme := filepath.Join(dir, "issues", "README.md")
	changed, err := index.UpdateIssuesReadme(readme, filepath.Join(dir, "issues"))
	if err != nil {
		return err
	}
	if len(state.Repairs) == 0 && !changed {
		fmt.Fprintln(w, "issues rebase: no repair needed")
		return runIssuesLint(w, dir)
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
	// A failed earlier attempt already staged the old path's deletion, and git add
	// rejects a path that is in neither the tree nor the index. git commit still
	// accepts it, because HEAD has it.
	var addable []string
	for _, path := range stage {
		_, statErr := os.Stat(filepath.Join(dir, filepath.FromSlash(path)))
		tracked, err := gitOutput(dir, "ls-files", "--", path)
		if err != nil {
			return err
		}
		if statErr == nil || tracked != "" {
			addable = append(addable, path)
		}
	}
	if _, err := gitOutput(dir, append([]string{"add", "--"}, addable...)...); err != nil {
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

// rewriteRepairReferences points references to each moved ticket at its new path
// and returns every file to stage, including those rewritten by an earlier
// attempt. A file with uncommitted changes is skipped, because committing it
// would also commit the user's unrelated work.
func rewriteRepairReferences(w io.Writer, dir string, state *rebaseState, statePath string) ([]string, error) {
	// Files this repair writes itself count as changed, but are never user work.
	ours := map[string]bool{}
	for _, name := range issueIndexRelPaths() {
		ours[name] = true
	}
	for _, repair := range state.Repairs {
		ours[repair.NewPath] = true
	}
	for _, path := range state.RefPaths {
		ours[path] = true
	}
	for _, repair := range state.Repairs {
		edits, err := issuecore.PlanReferences(dir, repair.OldPath, repair.NewPath)
		if err != nil {
			return nil, fmt.Errorf("scan references: %w", err)
		}
		var clean []issuecore.ReferenceEdit
		for _, e := range edits {
			rel, err := filepath.Rel(dir, e.Path)
			if err != nil {
				return nil, err
			}
			rel = filepath.ToSlash(rel)
			if !ours[rel] {
				pathStatus, err := gitOutput(dir, "status", "--porcelain", "--", rel)
				if err != nil {
					return nil, err
				}
				if pathStatus != "" {
					fmt.Fprintf(w, "warning: %s has uncommitted changes and still names %s; update it by hand\n", rel, repair.OldPath)
					continue
				}
				// Status is also empty for an ignored file, which git add would reject.
				tracked, err := gitOutput(dir, "ls-files", "--", rel)
				if err != nil {
					return nil, err
				}
				if tracked == "" {
					fmt.Fprintf(w, "warning: %s is not tracked and still names %s; update it by hand\n", rel, repair.OldPath)
					continue
				}
				ours[rel] = true
				state.RefPaths = append(state.RefPaths, rel)
			}
			clean = append(clean, e)
		}
		if len(clean) == 0 {
			continue
		}
		if err := writeRebaseState(statePath, *state); err != nil {
			return nil, fmt.Errorf("persist repair references: %w", err)
		}
		if err := issuecore.WriteReferences(clean, false); err != nil {
			return nil, fmt.Errorf("rewrite references: %w", err)
		}
	}
	return state.RefPaths, nil
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

type issueIndexSnapshot map[string]FileSnapshot

// IssueIndexSnapshot captures generated issue indexes for rollback.
type IssueIndexSnapshot = issueIndexSnapshot

// FileSnapshot holds the saved content and existence state of one index file.
type FileSnapshot struct {
	Content []byte
	Existed bool
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
		snapshot[name] = FileSnapshot{Content: content, Existed: err == nil}
	}
	return snapshot, nil
}

func restoreIssueIndexes(issuesDir string, snapshot issueIndexSnapshot) error {
	for name, file := range snapshot {
		path := filepath.Join(issuesDir, name)
		if file.Existed {
			if err := os.WriteFile(path, file.Content, 0o644); err != nil {
				return err
			}
		} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// IssueIndexRelPaths returns the generated issue index paths staged with updates.
func IssueIndexRelPaths() []string { return issueIndexRelPaths() }

// SnapshotIssueIndexes captures the current issue index files.
func SnapshotIssueIndexes(issuesDir string) (IssueIndexSnapshot, error) {
	return snapshotIssueIndexes(issuesDir)
}

// RestoreIssueIndexes restores a snapshot of issue index files.
func RestoreIssueIndexes(issuesDir string, snapshot IssueIndexSnapshot) error {
	return restoreIssueIndexes(issuesDir, snapshot)
}

func runIssuesLintMode(w io.Writer, dir string, cached bool, untrackedTickets []string) error {
	issuesDir := filepath.Join(dir, "issues")
	if _, err := os.Stat(issuesDir); os.IsNotExist(err) {
		// A tracker with no issues/ yet (fresh harnez init) has nothing to lint (issue 1072).
		fmt.Fprintln(w, "issues lint: no issues/ directory yet, nothing to lint")
		return nil
	}
	files, err := issuecore.Scan(issuesDir)
	if err != nil {
		return fmt.Errorf("issues lint: %w", err)
	}
	seen := map[string]string{}
	for _, f := range files {
		if f.Canonical == issuecore.StatusUnknown {
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
		title, _, _ := issuecore.ParseIssueFile(string(content))
		fields := strings.Fields(title)
		if len(fields) > 0 {
			_, parseErr := issuecore.NormalizeID(fields[0])
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
			return fmt.Errorf("issues lint: docs/README.md is generated studies index drift; run 'harnez index --stage'")
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
	defer os.RemoveAll(tmp) //nolint:errcheck // best-effort cleanup; error not actionable
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
	// During an `issues rebase` replay the indexes are regenerated afterwards, so
	// two tickets with one number must not stop the replay before their repair.
	replay := rebaseReplayActive(root)
	var generated string
	switch target {
	case "issues/README.md":
		if index.HasConflictOutsideManagedBlock(string(orig), "issues-active-index") {
			return fmt.Errorf("issues merge-driver: unresolved conflict outside generated block in %s", target)
		}
		if ancestorPath != "" && otherPath != "" && replay {
			generated, err = index.MergeIssuesReadmeReplay(ancestor, string(orig), other)
		} else if ancestorPath != "" && otherPath != "" {
			generated, err = index.MergeIssuesReadme(ancestor, string(orig), other)
		} else {
			generated, err = index.RenderIssuesReadme(string(orig), issuesDir, target)
		}
	case "issues/BACKLOG.md":
		fallthrough
	case "issues/CLOSED.md":
		if ancestorPath != "" && otherPath != "" && replay {
			generated, err = index.MergeFullIssueIndexReplay(ancestor, string(orig), other, filepath.Base(target))
		} else if ancestorPath != "" && otherPath != "" {
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

// RunIssuesRebase repairs ticket-number collisions after a rebase.
func RunIssuesRebase(w io.Writer, dir, upstream string, dryRun bool) error {
	return runIssuesRebase(w, dir, upstream, dryRun)
}

// RunIssuesLint validates issue files and their generated indexes.
func RunIssuesLint(w io.Writer, dir string) error { return runIssuesLint(w, dir) }

// RunIssuesLintCached validates the cached issue tree before a commit.
func RunIssuesLintCached(w io.Writer, dir string) error { return runIssuesLintCached(w, dir) }

// RunIssuesStatus prints issue tracker status and active-ticket warnings.
func RunIssuesStatus(w io.Writer, dir string) error { return runIssuesStatus(w, dir) }

// RunIssuesMergeDriver regenerates a generated index from its source files.
func RunIssuesMergeDriver(currentPath, targetPath string) error {
	return runIssuesMergeDriver(currentPath, targetPath)
}

// RunIssuesMergeDriverVersions merges versions of a generated index.
func RunIssuesMergeDriverVersions(ancestorPath, currentPath, otherPath, targetPath string) error {
	return runIssuesMergeDriverVersions(ancestorPath, currentPath, otherPath, targetPath)
}
