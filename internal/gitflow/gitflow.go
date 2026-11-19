// Package gitflow reads a repo's git workflow mode from .harnez/repo.yaml and
// answers which branch counts as the default (and so as protected). Defaults
// and fallback branch names live in spec/git.yaml, not in Go.
package gitflow

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
	"ubunatic.com/harnez/internal/disable"
	"ubunatic.com/harnez/internal/forge"
	"ubunatic.com/harnez/internal/prwait"
)

// Mode is the repo's git workflow: solo commits on the default branch, team
// and fork use topic branches and pull requests.
type Mode string

const (
	ModeSolo Mode = "solo"
	ModeFork Mode = "fork"
	ModeTeam Mode = "team"
)

// Config is the `git:` key of .harnez/repo.yaml.
type Config struct {
	Mode          Mode   `yaml:"mode"`
	ProtectedMain bool   `yaml:"protected_main"`
	DefaultBranch string `yaml:"default_branch,omitempty"`
}

type specDefaults struct {
	Mode             Mode     `yaml:"mode"`
	ProtectedMain    bool     `yaml:"protected_main"`
	FallbackBranches []string `yaml:"fallback_branches"`
}

var loadSpec = sync.OnceValues(func() (specDefaults, error) {
	data, err := fs.ReadFile(harnez.DefaultFS, "spec/git.yaml")
	if err != nil {
		return specDefaults{}, fmt.Errorf("read spec/git.yaml: %w", err)
	}
	schemaData, err := fs.ReadFile(harnez.DefaultFS, "spec/schemas/git.schema.json")
	if err != nil {
		return specDefaults{}, fmt.Errorf("read git schema: %w", err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(data, &document); err != nil {
		return specDefaults{}, fmt.Errorf("parse spec/git.yaml: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(schemaData, &schema); err != nil {
		return specDefaults{}, fmt.Errorf("parse git schema: %w", err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return specDefaults{}, fmt.Errorf("resolve git schema: %w", err)
	}
	if err := resolved.Validate(document); err != nil {
		return specDefaults{}, fmt.Errorf("validate spec/git.yaml: %w", err)
	}
	var spec specDefaults
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return specDefaults{}, fmt.Errorf("decode spec/git.yaml: %w", err)
	}
	return spec, nil
})

var gitKeys = []string{"mode", "protected_main", "default_branch"}

// Load reads only the `git` key of dir/.harnez/repo.yaml. A missing file or
// key yields the spec defaults. Unknown keys, an unknown mode and
// protected_main on a solo repo fail with an error naming the file and key, so
// callers that never run the full repo.yaml schema still reject them.
func Load(dir string) (Config, error) {
	path := filepath.Join(dir, ".harnez", "repo.yaml")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		spec, err := loadSpec()
		if err != nil {
			return Config{}, err
		}
		return Config{Mode: spec.Mode, ProtectedMain: spec.ProtectedMain}, nil
	}
	if err != nil {
		return Config{}, err
	}
	return LoadBytes(path, data)
}

// LoadBytes runs Load's checks on repo.yaml content that is not on disk yet,
// so a writer can validate a candidate before replacing the file. path is only
// used to name the file in errors.
func LoadBytes(path string, data []byte) (Config, error) {
	spec, err := loadSpec()
	if err != nil {
		return Config{}, err
	}
	cfg := Config{Mode: spec.Mode, ProtectedMain: spec.ProtectedMain}
	var doc struct {
		Git map[string]any `yaml:"git"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	for key := range doc.Git {
		if !slices.Contains(gitKeys, key) {
			return Config{}, fmt.Errorf("%s: unknown key git.%s (want one of %s)", path, key, strings.Join(gitKeys, ", "))
		}
	}
	if v, ok := doc.Git["mode"]; ok {
		s, _ := v.(string)
		switch m := Mode(s); m {
		case ModeSolo, ModeFork, ModeTeam:
			cfg.Mode = m
		default:
			return Config{}, fmt.Errorf("%s: git.mode %v is not one of solo, fork, team", path, v)
		}
	}
	if v, ok := doc.Git["protected_main"]; ok {
		b, isBool := v.(bool)
		if !isBool {
			return Config{}, fmt.Errorf("%s: git.protected_main must be true or false, got %v", path, v)
		}
		cfg.ProtectedMain = b
	}
	if v, ok := doc.Git["default_branch"]; ok {
		s, _ := v.(string)
		if s == "" {
			return Config{}, fmt.Errorf("%s: git.default_branch must be a non-empty branch name, got %v", path, v)
		}
		cfg.DefaultBranch = s
	}
	if cfg.Mode == ModeSolo && cfg.ProtectedMain {
		return Config{}, fmt.Errorf("%s: git.protected_main requires git.mode team or fork, not solo", path)
	}
	return cfg, nil
}

// DefaultBranch returns the repo's default branch: git.default_branch when
// set, else the branch origin/HEAD points to. ok is false when neither is known.
func DefaultBranch(dir string, c Config) (string, bool) {
	if c.DefaultBranch != "" {
		return c.DefaultBranch, true
	}
	out, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD").Output()
	if err != nil {
		return "", false
	}
	branch := strings.TrimPrefix(strings.TrimSpace(string(out)), "origin/")
	return branch, branch != ""
}

// ProtectedBranches lists the branches that count as the default branch: the
// detected one, or every spec fallback name when detection fails (the
// conservative choice). Load fails first when the spec is unreadable, so an
// error here yields no fallback names only on a path that skipped Load.
func ProtectedBranches(dir string, c Config) []string {
	if branch, ok := DefaultBranch(dir, c); ok {
		return []string{branch}
	}
	spec, _ := loadSpec()
	return slices.Clone(spec.FallbackBranches)
}

// Stderr receives the guard's warnings. Tests replace it.
var Stderr io.Writer = os.Stderr

// CurrentBranch returns the checked-out branch. detached is true when HEAD
// points at a commit; ok is false when dir is not a git work tree.
func CurrentBranch(dir string) (branch string, detached, ok bool) {
	out, err := exec.Command("git", "-C", dir, "symbolic-ref", "-q", "--short", "HEAD").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), false, true
	}
	if exec.Command("git", "-C", dir, "rev-parse", "--git-dir").Run() != nil {
		return "", false, false
	}
	return "", true, true
}

// DetachedHeadRef returns a human-readable ref or commit description for a
// detached HEAD (such as an exact tag or short commit hash).
func DetachedHeadRef(dir string) string {
	out, err := exec.Command("git", "-C", dir, "describe", "--tags", "--exact-match", "HEAD").Output()
	if err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	out, err = exec.Command("git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	return "HEAD"
}

// CheckDetachedHead checks whether dir is in a detached HEAD state.
// If detached, it returns a descriptive warning string; otherwise it returns empty string.
func CheckDetachedHead(dir string) string {
	_, detached, ok := CurrentBranch(dir)
	if !ok || !detached {
		return ""
	}
	ref := DetachedHeadRef(dir)
	return fmt.Sprintf("WARNING: HEAD is detached at %s; agent commits will land off-branch. Switch to a branch (git switch -c <topic> or git switch <branch>) before committing.", ref)
}

// GuardRelease is GuardCommit for `harnez release`, which may run on a protected
// default branch: it is an explicit request that adds only the bump commit and
// tag, and the forge's branch protection decides whether the push is accepted.
func GuardRelease(dir string) error {
	cfg, err := Load(dir)
	if err != nil {
		return err
	}
	if cfg.ProtectedMain {
		branch, detached, ok := CurrentBranch(dir)
		if ok && !detached && slices.Contains(ProtectedBranches(dir, cfg), branch) {
			fmt.Fprintf(Stderr, "git.protected_main: releasing on protected %s: trying direct push first; falls back to an AGit PR if the remote rejects it (harnez release --continue)\n", branch)
			return nil
		}
	}
	return GuardCommit(dir, "release")
}

// GuardCommit is the one check every harnez path that stages or commits runs
// first, before it writes anything. action names the operation for the
// message ("close ticket", "release"). Solo repos always pass. Team and fork
// repos warn when committing on the default branch or a detached HEAD; with
// protected_main they refuse, naming the command that fixes it.
func GuardCommit(dir, action string) error {
	return guardCommit(dir, action, nil)
}

// IssueCommitOptions enables owner-authorized overrides only for issue commits.
type IssueCommitOptions struct {
	PushMain bool
	Stderr   io.Writer
}

func GuardIssueCommit(dir, action string, opts IssueCommitOptions) error {
	return guardCommit(dir, action, &opts)
}

func guardCommit(dir, action string, opts *IssueCommitOptions) error {
	cfg, err := Load(dir)
	if err != nil {
		return err
	}
	if cfg.Mode == ModeSolo {
		return nil
	}
	branch, detached, ok := CurrentBranch(dir)
	if !ok {
		return nil // not a git work tree: the commit step reports that itself
	}
	if detached {
		if cfg.ProtectedMain {
			return fmt.Errorf("git.protected_main is set in .harnez/repo.yaml: refusing to %s on a detached HEAD; check out a branch (git switch -c topic or git switch <branch>)", action)
		}
		fmt.Fprintf(Stderr, "git.mode=%s: committing on a detached HEAD; check out a branch (git switch -c topic or git switch <branch>)\n", cfg.Mode)
		return nil
	}
	if !slices.Contains(ProtectedBranches(dir, cfg), branch) {
		return nil
	}
	if cfg.ProtectedMain {
		if opts != nil {
			writer := opts.Stderr
			if writer == nil {
				writer = Stderr
			}
			if opts.PushMain {
				fmt.Fprintln(writer, "git.protected_main: ticket commit unlocked by --push-main flag")
				return nil
			}
			sessionID, applied, err := pushMainOverride(dir)
			if err != nil {
				return err
			}
			if applied {
				fmt.Fprintf(writer, "git.protected_main: ticket commit unlocked by /disable repo-rules push-main for session %s\n", sessionID)
				return nil
			}
			return fmt.Errorf("git.protected_main is set in .harnez/repo.yaml: refusing to %s on '%s'; use --push-main or /disable repo-rules push-main, or run: git switch -c issue/<n>-<slug>", action, branch)
		}
		return fmt.Errorf("git.protected_main is set in .harnez/repo.yaml: refusing to %s on '%s'; run: git switch -c issue/<n>-<slug>", action, branch)
	}
	fmt.Fprintf(Stderr, "git.mode=%s: committing on %s; use a topic branch\n", cfg.Mode, branch)
	return nil
}

// pushMainOverride reports whether `/disable repo-rules push-main` is applied
// to the repository at dir, and for which session. Both commit and push guards
// honour it, so the owner's override means the same thing on both paths.
func pushMainOverride(dir string) (sessionID string, applied bool, err error) {
	spec, err := disable.LoadSpec()
	if err != nil {
		return "", false, err
	}
	root, err := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", false, fmt.Errorf("resolve repository root: %w", err)
	}
	status := spec.Check(strings.TrimSpace(string(root)), "repo-rules", "push-main", nil)
	if !status.Applied {
		return "", false, nil
	}
	return status.State.SessionID, true, nil
}

// GuardPush is GuardPushTarget for a push with no remote context: the policy
// applies to a direct push to the default branch on any remote.
func GuardPush(dir, remoteRef string) error {
	return GuardPushTarget(dir, "", "", remoteRef)
}

// GuardPushTarget is the push-side check: the pre-push hook and `harnez
// push-guard` call it once per destination ref (remoteRef, such as
// refs/heads/main) before git updates it. remote and url name the destination;
// the hook passes git's $1 and $2. Both empty means the target is unknown and
// the policy applies. It follows GuardCommit's rules. With git.protected_main,
// a direct push to the default branch on an AGit-capable remote is refused and
// the error names the AGit ref to use instead (see pushTargetGuarded). Everything
// else passes: AGit refs/for/ pushes, tags, topic branches, GitHub and other
// mirrors, and every repo without protected_main (solo included).
func GuardPushTarget(dir, remote, url, remoteRef string) error {
	cfg, err := Load(dir)
	if err != nil {
		return err
	}
	if !cfg.ProtectedMain || !pushTargetGuarded(remote, url) {
		return nil
	}
	branch, isHead := strings.CutPrefix(remoteRef, "refs/heads/")
	if !isHead || !slices.Contains(ProtectedBranches(dir, cfg), branch) {
		return nil
	}
	sessionID, applied, err := pushMainOverride(dir)
	if err != nil {
		return err
	}
	if applied {
		fmt.Fprintf(Stderr, "git.protected_main: direct push to %s unlocked by /disable repo-rules push-main for session %s\n", remoteRef, sessionID)
		return nil
	}
	return fmt.Errorf("git.protected_main is set in .harnez/repo.yaml: refusing a direct push to %s; push through AGit instead: git push origin HEAD:refs/for/%s/<topic> (or /disable repo-rules push-main)", remoteRef, branch)
}

// pushTargetGuarded reports whether protected_main covers a push to remote at
// url. It covers AGit-capable destinations: origin, unless its URL names a
// GitHub host, and any remote whose URL names a kind: forgejo host in
// spec/pr.yaml. GitHub has no AGit, so a GitHub URL passes, and so does a named
// remote that is neither origin nor a Forgejo URL, such as a fork or a mirror.
// A release mirrors the merged default branch to those remotes after its AGit
// PR, so their direct push must pass. An unknown target (both empty) is guarded.
func pushTargetGuarded(remote, url string) bool {
	if remote == "" && url == "" {
		return true
	}
	if host, _, _, ok := forge.ParseRemoteURL(url); ok {
		spec, err := prwait.LoadSpec()
		if err != nil {
			return true
		}
		if fg, known := spec.ForgeFor(host); known {
			return fg.Kind == "forgejo"
		}
	}
	return remote == "origin"
}

// ProtectedReleaseBranch reports the branch when a release runs on a protected
// default branch, so harnez can try a direct push and fall back to an AGit PR.
func ProtectedReleaseBranch(dir string) (string, bool) {
	cfg, err := Load(dir)
	if err != nil || !cfg.ProtectedMain {
		return "", false
	}
	branch, detached, ok := CurrentBranch(dir)
	if !ok || detached || !slices.Contains(ProtectedBranches(dir, cfg), branch) {
		return "", false
	}
	return branch, true
}
