package claude

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"ubunatic.com/harnez/internal/agy"
	"ubunatic.com/harnez/internal/codex"
	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/jsonc"
	"ubunatic.com/harnez/internal/markdown"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/skillreg"
)

// applyResult is returned by every apply-level operation.
// Callers own printing; nothing inside prints directly.
type applyResult struct {
	changed bool
	notes   []string // detail lines printed indented beneath the action line
}

// applySectionMD updates or inserts a managed section inside a markdown file.
func applySectionMD(path, section, content string) (applyResult, error) {
	changed, existed, err := markdown.Apply(path, section, content)
	if err != nil {
		return applyResult{}, err
	}
	var notes []string
	if changed {
		verb := "added"
		if existed {
			verb = "updated"
		}
		notes = append(notes, section+": "+verb)
	}
	return applyResult{changed: changed, notes: notes}, nil
}

// diffSectionMD prints unified diff of a managed markdown section.
func diffSectionMD(path, section, content string) (bool, error) {
	return markdown.Diff(path, section, content)
}

// cleanSectionMD cleans/removes a managed section and prints status.
func cleanSectionMD(path, section string) error {
	removed, cleaned, err := markdown.Clean(path, section)
	if err != nil {
		return err
	}
	if removed {
		fmt.Printf("  removed %s\n", path)
	} else if cleaned {
		fmt.Printf("  cleaned %s\n", path)
	}
	return nil
}

func ensureSymlink(linkPath, target string) (applyResult, error) {
	changed, err := fsutil.EnsureSymlink(linkPath, target)
	if err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: changed}, nil
}

func writeFileIfChanged(dst string, data []byte) (applyResult, error) {
	changed, err := fsutil.WriteIfChanged(dst, data)
	if err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: changed}, nil
}

func copyFile(src, dst string) (applyResult, error) {
	changed, err := fsutil.Copy(src, dst)
	if err != nil {
		return applyResult{}, err
	}
	return applyResult{changed: changed}, nil
}

// systemdUserUnitDir resolves the standard systemd --user unit directory:
// ~/.config/systemd/user/.
func systemdUserUnitDir() (string, error) {
	if base := os.Getenv("XDG_CONFIG_HOME"); filepath.IsAbs(base) {
		return filepath.Join(base, "systemd", "user"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "systemd", "user"), nil
}

// installSystemdUnit writes (or refreshes) the harnez-agent-collector
// systemd --user unit file (issue 082). It substitutes the unit's
// ExecStart with the absolute path to the currently running harnez binary
// (os.Executable()) rather than a bare `harnez` command name, because a
// systemd --user session does not reliably inherit the interactive shell's
// PATH — a bare command name would silently fail to start regardless of
// whether harnez was installed via `go install` (~/go/bin) or
// `make install-system` (/usr/local/bin).
func installSystemdUnit(fsys fs.FS) (string, applyResult, error) {
	serverConfig, err := server.LoadConfig()
	if err != nil {
		return "", applyResult{}, err
	}
	unitDir, err := systemdUserUnitDir()
	if err != nil {
		return "", applyResult{}, err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", applyResult{}, fmt.Errorf("resolve harnez binary path: %w", err)
	}
	tmpl, err := fs.ReadFile(fsys, serverConfig.SystemdUnitSource)
	if err != nil {
		return "", applyResult{}, fmt.Errorf("read %s: %w", serverConfig.SystemdUnitSource, err)
	}
	content := strings.NewReplacer(
		"{{HARNEZ_BIN}}", exe,
		"{{RESTART_POLICY}}", serverConfig.SystemdRestartPolicy,
		"{{RESTART_DELAY}}", serverConfig.SystemdRestartDelay.String(),
		"{{SSH_AUTH_SOCK_ENV}}", server.SystemdSSHAuthSockEnvironment(),
	).Replace(string(tmpl))
	dst := filepath.Join(unitDir, serverConfig.SystemdUnitName)
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return "", applyResult{}, fmt.Errorf("create %s: %w", unitDir, err)
	}
	r, err := writeFileIfChanged(dst, []byte(content))
	return dst, r, err
}

func installDoc(fsys fs.FS, src, dst string, force bool) (applyResult, error) {
	if !force {
		if _, err := os.Stat(dst); err == nil {
			return applyResult{changed: false}, nil
		}
	}
	data, err := fs.ReadFile(fsys, src)
	if err != nil {
		return applyResult{}, err
	}
	return writeManagedDoc(dst, data)
}

func buildLangConventions(names []string, cfg *Config) string {
	var sb strings.Builder
	sb.WriteString("Adhere to the following conventions.\n\n")
	sb.WriteString("Docs in `./docs/` are managed by harnez. <!-- harnez:bundled -->\n\n")
	for _, name := range names {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok || lang.Ref == "" {
			continue
		}
		displayName := lang.Name
		if displayName == "" {
			displayName = name
		}
		sb.WriteString("- " + displayName + " " + lang.Ref)
		if lang.Hint != "" {
			hint := strings.TrimRight(lang.Hint, "\n")
			lines := strings.Split(hint, "\n")
			sb.WriteString(",\n")
			for _, line := range lines {
				sb.WriteString("  " + line + "\n")
			}
		} else {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func langDocState(fsys fs.FS, src, dst string) string {
	installed, err := os.ReadFile(dst)
	if err != nil {
		return "not installed"
	}
	bundled, err := fs.ReadFile(fsys, src)
	if err != nil {
		return "installed"
	}
	if bytes.Equal(installed, bundled) {
		return "bundled"
	}
	return "custom"
}

func genCommandContent(cmd Command, fsys fs.FS) (string, error) {
	body := cmd.Content
	if cmd.File != "" {
		data, err := fs.ReadFile(fsys, cmd.File)
		if err != nil {
			return "", fmt.Errorf("command %s: %w", cmd.Name, err)
		}
		body = string(data)
	}
	var sb strings.Builder
	if cmd.Description != "" {
		sb.WriteString("---\n")
		fmt.Fprintf(&sb, "description: %q\n", cmd.Description)
		sb.WriteString("---\n")
	}
	sb.WriteString(strings.TrimRight(body, "\n"))
	sb.WriteString("\n")
	return sb.String(), nil
}

type skillResourceContent struct {
	target string
	data   []byte
}

func validateSkillResourceTarget(skillName, target string) (string, error) {
	cleanTarget := path.Clean(target)
	if target == "" || path.IsAbs(target) || cleanTarget == "." || cleanTarget == ".." || strings.HasPrefix(cleanTarget, "../") {
		return "", fmt.Errorf("skill %s: resource target %q must stay inside the skill directory", skillName, target)
	}
	if cleanTarget == "SKILL.md" {
		return "", fmt.Errorf("skill %s: resource target %q is reserved for the generated skill body", skillName, target)
	}
	return filepath.FromSlash(cleanTarget), nil
}

func validateSkillResourceTargets(skill Command) ([]string, error) {
	targets := make([]string, 0, len(skill.Resources))
	for _, resource := range skill.Resources {
		if resource.Source == "" {
			return nil, fmt.Errorf("skill %s: resource source is required", skill.Name)
		}
		target, err := validateSkillResourceTarget(skill.Name, resource.Target)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	sortedTargets := append([]string(nil), targets...)
	sort.Strings(sortedTargets)
	for i := 1; i < len(sortedTargets); i++ {
		if sortedTargets[i] == sortedTargets[i-1] || strings.HasPrefix(sortedTargets[i], sortedTargets[i-1]+string(filepath.Separator)) {
			return nil, fmt.Errorf("skill %s: resource targets overlap at %q", skill.Name, sortedTargets[i])
		}
	}
	return targets, nil
}

func genSkillResources(skill Command, fsys fs.FS) ([]skillResourceContent, error) {
	targets, err := validateSkillResourceTargets(skill)
	if err != nil {
		return nil, err
	}
	resources := make([]skillResourceContent, 0, len(skill.Resources))
	for i, resource := range skill.Resources {
		data, err := fs.ReadFile(fsys, resource.Source)
		if err != nil {
			return nil, fmt.Errorf("skill %s resource %s: %w", skill.Name, resource.Source, err)
		}
		resources = append(resources, skillResourceContent{target: targets[i], data: data})
	}
	return resources, nil
}

func safeSkillPath(skillDir, relativePath string) (string, error) {
	if info, err := os.Lstat(skillDir); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("skill path %s is a symlink", skillDir)
	} else if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	current := skillDir
	for _, part := range strings.Split(relativePath, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				break
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("skill path %s is a symlink", current)
		}
	}
	return filepath.Join(skillDir, relativePath), nil
}

func primeAgentRoot(cfg *Config) string {
	return fsutil.ExpandHome(cfg.PrimeAgentTarget)
}

func piAgentRoot(cfg *Config) string {
	if env := os.Getenv("PI_CODING_AGENT_DIR"); env != "" {
		return fsutil.ExpandHome(env)
	}
	if cfg != nil {
		return fsutil.ExpandHome(cfg.PiAgentTarget)
	}
	return ""
}

// sortedAgentIDs returns the keys of an agents_md.agents map in stable
// (alphabetical) order, so repeated applies iterate agent profiles in a
// deterministic sequence despite Go's randomized map iteration.
func sortedAgentIDs(agents map[string]AgentsMDTarget) []string {
	ids := make([]string, 0, len(agents))
	for id := range agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func appendUniquePath(paths []string, path string) []string {
	if path == "" {
		return paths
	}
	for _, existing := range paths {
		if existing == path {
			return paths
		}
	}
	return append(paths, path)
}

// skillDisabled reports whether skill must be actively removed rather than
// installed: rate_feedback-gated and disabled, or its `requires:` component
// isn't in the resolved selection (issue 142, generalized for issue 491 M5).
func skillDisabled(skill Command, selection Set, disableRateFeedback bool) bool {
	if skill.RateFeedback && disableRateFeedback {
		return true
	}
	for _, req := range skill.Requires {
		if !selection.HasComponent(req) {
			return true
		}
	}
	return false
}

func skillTargets(cfg *Config) []string {
	var targets []string

	if root := fsutil.ExpandHome(cfg.SkillsTarget); root != "" {
		targets = appendUniquePath(targets, root)
	} else if home, err := os.UserHomeDir(); err == nil {
		targets = appendUniquePath(targets, filepath.Join(home, ".gemini", "skills"))
	}

	targets = appendUniquePath(targets, fsutil.ExpandHome(cfg.CodexSkillsTarget))

	if root := fsutil.ExpandHome(cfg.ClaudeSkillsTarget); root != "" {
		targets = appendUniquePath(targets, root)
	} else if home, err := os.UserHomeDir(); err == nil {
		targets = appendUniquePath(targets, filepath.Join(home, ".claude", "skills"))
	}

	if root := primeAgentRoot(cfg); root != "" {
		targets = appendUniquePath(targets, filepath.Join(root, "skills"))
	}

	if root := piAgentRoot(cfg); root != "" {
		targets = appendUniquePath(targets, filepath.Join(root, "skills"))
	}

	return targets
}

func docTargets(target string, lang Language, cfg *Config) []string {
	var targets []string
	if target != "" {
		targets = append(targets, filepath.Join(target, "docs", filepath.Base(lang.Target)))
	} else if lang.Target != "" {
		targets = append(targets, fsutil.ExpandHome(lang.Target))
	}
	if root := primeAgentRoot(cfg); root != "" && lang.Target != "" {
		targets = appendUniquePath(targets, filepath.Join(root, "docs", filepath.Base(lang.Target)))
	}
	return targets
}

// CleanUnmanagedDocs removes docs in ~/.claude/docs/ and ~/.prime/agent/docs/
// that are not present in keepDocs.
func CleanUnmanagedDocs(target string, cfg *Config, keepDocs []string) (int, error) {
	keepSet := make(map[string]struct{}, len(keepDocs))
	for _, name := range expandDocNames(cfg, keepDocs) {
		keepSet[name] = struct{}{}
	}
	removed := 0
	for name, lang := range cfg.AgentsMD.Languages {
		if _, keep := keepSet[name]; keep {
			continue
		}
		for _, dst := range docTargets(target, lang, cfg) {
			if err := os.Remove(dst); err == nil {
				removed++
				fmt.Printf("  removed %s\n", dst)
				_ = os.Remove(filepath.Dir(dst))
			} else if !os.IsNotExist(err) {
				return removed, fmt.Errorf("remove doc %s [%s]: %w", name, dst, err)
			}
		}
	}
	return removed, nil
}

func commandTargets(target string, cfg *Config) []string {
	targets := []string{filepath.Join(target, "commands")}
	if root := primeAgentRoot(cfg); root != "" {
		targets = appendUniquePath(targets, filepath.Join(root, "prompts"))
	}
	return targets
}

func gearExecutable() string {
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}
	return "harnez"
}

// GearSymlinkTargets returns the list of candidate paths where the ⚙ multicall
// symlink should be provisioned for agent environments and PATH execution.
func GearSymlinkTargets(target string, cfg *Config) []string {
	var targets []string
	if target != "" {
		targets = appendUniquePath(targets, filepath.Join(target, "bin", "⚙"))
	}
	if cfg != nil {
		defaultTarget := ExpandTarget("", cfg.TargetDir)
		if target == defaultTarget {
			if home, err := os.UserHomeDir(); err == nil {
				goBin := filepath.Join(home, "go", "bin")
				if fi, err := os.Stat(goBin); err == nil && fi.IsDir() {
					targets = appendUniquePath(targets, filepath.Join(goBin, "⚙"))
				}
				localBin := filepath.Join(home, ".local", "bin")
				if fi, err := os.Stat(localBin); err == nil && fi.IsDir() {
					targets = appendUniquePath(targets, filepath.Join(localBin, "⚙"))
				}
			}
		}
		if root := primeAgentRoot(cfg); root != "" {
			targets = appendUniquePath(targets, filepath.Join(root, "bin", "⚙"))
		}
	}
	return targets
}

// BashShimContent is the guarded bash PATH-shim script (issue 271).
const BashShimContent = `#!/bin/sh
if test "$HARNEZ_INTERCEPTED" = "1"
then exec /bin/bash "$@"
fi
export HARNEZ_INTERCEPTED=1
exec harnez exec -- /bin/bash "$@"
`

// BashShimPath returns the standard location of the guarded bash shim (~/.harnez/shims/bash).
func BashShimPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".harnez", "shims", "bash")
}

// EnsureBashShim writes or refreshes the managed guarded bash shim using the
// same executable-file policy as `harnez apply`.
func EnsureBashShim(home string) (path string, changed bool, err error) {
	if home == "" {
		return "", false, errors.New("empty home directory")
	}
	path = filepath.Join(home, ".harnez", "shims", "bash")
	changed, err = fsutil.WriteExecutableIfChanged(path, []byte(BashShimContent))
	if err != nil {
		return path, false, fmt.Errorf("bash shim %s: %w", path, err)
	}
	return path, changed, nil
}

// HarnezAgyLauncherContent installs harnez-agy, which resolves agy using the
// caller's PATH before adding the guarded bash shim directory.
const HarnezAgyLauncherContent = `#!/bin/sh
set -f
meter=0
if test "${1-}" = "--meter"
then
    meter=1
    shift
fi
home=$HOME
shim_dir="$home/.harnez/shims"
old_path=${PATH-}
agy_path=
old_ifs=$IFS
IFS=:
for entry in $old_path
do
    test -n "$entry" || entry=.
    candidate="$entry/agy"
    if test -x "$candidate" && test ! -d "$candidate" && test "$candidate" != "$0"
    then
        agy_path=$candidate
        break
    fi
done
IFS=$old_ifs
if test -z "$agy_path"
then
    printf '%s\n' 'harnez-agy: agy not found on the original PATH' >&2
    exit 127
fi
if test "$meter" = 1
then
    harnez_bin=$(command -v harnez 2>/dev/null || true)
    if test -n "$harnez_bin"
    then
        exec "$harnez_bin" agy-meter-run -- "$agy_path" "$@"
    fi
    printf '%s\n' 'harnez-agy: harnez not found; starting agy without metering' >&2
fi
if test "${old_path%%:*}" = "$shim_dir"
then
    export ANTIGRAVITY_AGENT=1
    exec "$agy_path" "$@"
fi
if test -n "$old_path"
then
    export PATH="$shim_dir:$old_path"
    export ANTIGRAVITY_AGENT=1
fi
if test -z "$old_path"
then
    export PATH="$shim_dir"
    export ANTIGRAVITY_AGENT=1
fi
exec "$agy_path" "$@"
`

// HarnezAgyLauncherPath returns the managed harnez-agy launcher path.
func HarnezAgyLauncherPath(home string) string {
	return filepath.Join(home, ".local", "bin", "harnez-agy")
}

// EnsureHarnezAgyLauncher installs the managed harnez-agy launcher.
func EnsureHarnezAgyLauncher(home string) (string, bool, error) {
	if home == "" {
		return "", false, errors.New("empty home directory")
	}
	path := HarnezAgyLauncherPath(home)
	changed, err := fsutil.WriteExecutableIfChanged(path, []byte(HarnezAgyLauncherContent))
	if err != nil {
		return path, false, fmt.Errorf("harnez-agy launcher %s: %w", path, err)
	}
	return path, changed, nil
}

const HarnezEnvContent = `# harnez:begin env
# Shell environment and helper functions for harnez-managed tools and agents.

# Antigravity (AGY) wrapper with guarded shims PATH and agent indicator
agy() {
    PATH="$HOME/.harnez/shims:$PATH" ANTIGRAVITY_AGENT=1 command agy "$@"
}
# harnez:end env
`

const HarnezShellRCSnippet = `if test -f "$HOME/.harnez/env.sh"
then source "$HOME/.harnez/env.sh"
fi
`

// HarnezEnvPath returns the standard location of the harnez environment script (~/.harnez/env.sh).
func HarnezEnvPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".harnez", "env.sh")
}

// ShellRCPaths returns potential shell rc targets (~/.bashrc, ~/.zshrc) that exist,
// or ~/.bashrc if neither exists.
func ShellRCPaths() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	candidates := []string{
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".zshrc"),
	}
	var existing []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			existing = append(existing, p)
		}
	}
	if len(existing) == 0 {
		return []string{candidates[0]}
	}
	return existing
}

// SwitchDocVariant re-installs a single already project-installed doc through
// the same installDoc/MergeManagedDoc path init.go uses, swapping which
// source variant supplies the managed content while preserving any
// harnez:stop-delimited local section. It touches nothing else — no other
// doc, no Makefile, no AGENTS.md section. name must be a valid entry in
// cfg.AgentsMD.Languages with Local set; variant must be "lite" or "full".
func SwitchDocVariant(dir string, cfg *Config, name, variant string) (changed bool, err error) {
	if variant != "lite" && variant != "full" {
		return false, fmt.Errorf("invalid variant %q: must be lite or full", variant)
	}
	lang, ok := cfg.AgentsMD.Languages[name]
	if !ok {
		return false, fmt.Errorf("unknown doc: %s", name)
	}
	if lang.Local == "" {
		return false, fmt.Errorf("doc %q has no project-local target (local: unset)", name)
	}
	resolveVariant := variant
	if variant == "full" {
		resolveVariant = ""
	}
	if variant == "lite" && lang.LiteSource == "" {
		return false, fmt.Errorf("doc %q has no lite_source configured; nothing to switch to", name)
	}
	dst := localPath(dir, lang.Local)
	r, err := installDoc(cfg.FS, lang.SourceFor(resolveVariant), dst, true)
	return r.changed, err
}

func localPath(projectDir, rel string) string {
	if filepath.IsAbs(rel) || projectDir == "" || projectDir == "." {
		return rel
	}
	return filepath.Join(projectDir, rel)
}

func printResult(action, path string, r applyResult) {
	if !r.changed {
		return
	}
	fmt.Printf("  %s %s\n", action, path)
	for _, n := range r.notes {
		fmt.Printf("    %s\n", n)
	}
}

// mergeDocs returns the union of config-declared docs and CLI --doc flags,
// preserving order (config first, then any extras from the flag).
func mergeDocs(fromConfig, fromFlag []string) []string {
	return jsonc.UnionStrings(fromConfig, fromFlag)
}

// ApplyAll applies configuration. installSystemd additionally installs the
// harnez-agent-collector systemd --user unit (issue 082) to
// ~/.config/systemd/user/. installShell (opt-in) additionally injects the
// harnez environment source block into shell rc files (~/.bashrc, ~/.zshrc).
// ApplyAll installs the managed configuration into target, always using the
// full doc source. See ApplyAllVariant to select a lite_source variant.
func ApplyAll(target string, cfg *Config, docs []string, forceDocs bool, installSystemd bool, installShell ...bool) error {
	return ApplyAllVariant(target, cfg, ApplyOptions{Docs: docs, ForceDocs: forceDocs, InstallSystemd: installSystemd, InstallShell: len(installShell) > 0 && installShell[0]})
}

// ApplyAllVariant installs the selected global components and doc variant.
// cfg remains unfiltered: opts.Selection controls both installation and active
// removal of hooks, adapters and skills with unmet requirements.
func ApplyAllVariant(target string, cfg *Config, opts ApplyOptions) error {
	selection, docs := opts.Selection, opts.Docs
	forceDocs, installSystemd, docVariant := opts.ForceDocs, opts.InstallSystemd, opts.DocVariant
	shellOpt := opts.InstallShell
	docs = expandDocNames(cfg, docs)
	if err := validateDocNames(cfg, docs); err != nil {
		return err
	}
	r := &applyReport{}
	removed, err := removeDecommissionedArtifacts(target, cfg)
	if err != nil {
		return err
	}
	r.changes += removed

	removed, err = removeDroppedSkills(cfg)
	if err != nil {
		return err
	}
	r.changes += removed

	globalChanged, err := manageGlobalSections(cfg, false)
	if err != nil {
		return err
	}
	if globalChanged {
		r.changes++
	}
	if err := applySettings(r, target, cfg, selection); err != nil {
		return err
	}
	jevResult, err := applyManagedLocalPlugins(target, cfg.JevCompactionEnabled, true, cfg.Env["CLAUDE_CODE_ENABLE_FUNCTION_HOOKS"] != "")
	if err != nil {
		return fmt.Errorf("managed local plugins: %w", err)
	}
	if jevResult.changed {
		r.changes++
	}
	printResult("managed", filepath.Join(target, "plugins", "marketplaces", jevMarketplaceName), jevResult)

	disableRateFeedback := RateFeedbackDisabled(cfg, nil)

	if err := applyCommands(r, cfg, selection, target); err != nil {
		return err
	}

	if err := applySkills(r, cfg, selection, disableRateFeedback); err != nil {
		return err
	}

	if err := applyPiBackgroundTasks(r, cfg, selection); err != nil {
		return err
	}

	if err := applyCodexHooks(r, cfg, selection); err != nil {
		return err
	}

	if err := applyAgyHooks(r, cfg, selection); err != nil {
		return err
	}

	if err := applyDistillAdapters(r, cfg, selection); err != nil {
		return err
	}

	if err := applyGlobalDocs(r, cfg, docVariant, docs, forceDocs, selection, target); err != nil {
		return err
	}

	if err := applyShellIntegration(r, target, cfg, shellOpt); err != nil {
		return err
	}

	if installSystemd {
		unitPath, ur, err := installSystemdUnit(cfg.FS)
		if err != nil {
			return fmt.Errorf("systemd unit: %w", err)
		}
		if ur.changed {
			r.changes++
		}
		printResult("wrote", unitPath, ur)
		if ur.changed {
			fmt.Println("  install and start with: harnez server install")
		}
	}

	if r.changes == 0 {
		fmt.Println("No changes.")
	} else if r.changes == 1 {
		fmt.Println("1 change.")
	} else {
		fmt.Printf("%d changes.\n", r.changes)
	}
	for _, s := range r.stats {
		fmt.Printf("  %-14s %s\n", s.label+":", s.detail)
	}
	return nil
}

// DiffAll diffs the config and reports whether changes/drift were detected.
func DiffAll(target string, cfg *Config, selection Set) (bool, error) {
	anyChanged := false
	report := func(changed bool, err error) error {
		if err != nil {
			return err
		}
		if changed {
			anyChanged = true
		}
		return nil
	}

	if err := report(diffSettingsJSON(filepath.Join(target, "settings.json"), buildSettingsDoc(cfg, selection))); err != nil {
		return false, fmt.Errorf("settings: %w", err)
	}
	if err := report(manageGlobalSections(cfg, true)); err != nil {
		return false, err
	}
	if err := report(diffSkills(cfg, selection)); err != nil {
		return false, err
	}
	if err := report(diffPiBackgroundTasks(cfg, selection)); err != nil {
		return false, fmt.Errorf("pi background tasks: %w", err)
	}
	// issue 491 M5: check Codex/AGY under the resolved selection directly
	// rather than skipping them because a filtered config blanked their
	// targets — without telemetry, drift means a stale hook still installed.
	if cfg.CodexHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
		installed, drifted := codex.Status(hooksPath)
		if selection.HasComponent("telemetry") {
			if !installed || drifted {
				anyChanged = true
			}
		} else if installed {
			anyChanged = true
		}
	}
	if cfg.AgyHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.AgyHooksTarget)
		agentHome := filepath.Dir(filepath.Dir(hooksPath))
		if _, err := os.Stat(agentHome); err == nil {
			installed, drifted := agy.Status(hooksPath)
			if selection.HasComponent("telemetry") {
				if !installed || drifted {
					anyChanged = true
				}
			} else if installed {
				anyChanged = true
			}
		}
	}

	if shimPath := BashShimPath(); shimPath != "" {
		if selection.HasComponent("telemetry") {
			if fi, err := os.Stat(shimPath); err != nil {
				anyChanged = true
			} else {
				data, readErr := os.ReadFile(shimPath)
				if readErr != nil || string(data) != BashShimContent || fi.Mode().Perm() != 0755 {
					anyChanged = true
				}
			}
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		launcherPath := HarnezAgyLauncherPath(home)
		if selection.HasComponent("telemetry") {
			if fi, err := os.Stat(launcherPath); err != nil {
				anyChanged = true
			} else {
				data, readErr := os.ReadFile(launcherPath)
				if readErr != nil || string(data) != HarnezAgyLauncherContent || fi.Mode().Perm() != 0755 {
					anyChanged = true
				}
			}
		}
	}

	if envPath := HarnezEnvPath(); envPath != "" {
		if _, err := os.Stat(envPath); err != nil {
			anyChanged = true
		} else {
			data, readErr := os.ReadFile(envPath)
			if readErr != nil || string(data) != HarnezEnvContent {
				anyChanged = true
			}
		}
	}
	for _, rcPath := range ShellRCPaths() {
		if markdown.ContainsSectionMK(rcPath, "env") {
			if changed, err := markdown.DiffMK(rcPath, "env", HarnezShellRCSnippet); err != nil || changed {
				anyChanged = true
			}
		}
	}

	gearExe := gearExecutable()
	for _, link := range GearSymlinkTargets(target, cfg) {
		if _, err := os.Lstat(link); err != nil {
			anyChanged = true
		} else if targetDst, err := os.Readlink(link); err != nil {
			anyChanged = true
		} else {
			expected := gearExe
			if rel, err := filepath.Rel(filepath.Dir(link), gearExe); err == nil {
				expected = rel
			}
			if targetDst != expected && targetDst != gearExe {
				anyChanged = true
			}
		}
	}

	docsForDiff := cfg.Docs
	if !selection.HasComponent("docs") {
		docsForDiff = nil
	}
	for _, name := range docsForDiff {
		lang, ok := cfg.AgentsMD.Languages[name]
		if !ok {
			continue
		}
		for _, dst := range docTargets(target, lang, cfg) {
			state := langDocState(cfg.FS, lang.Source, dst)
			if state != "bundled" {
				anyChanged = true
			}
		}
	}

	if !anyChanged {
		fmt.Println("No changes.")
	}
	return anyChanged, nil
}

// CleanAll cleans the config.
func CleanAll(target string, cfg *Config) error {
	withoutSections := *cfg
	withoutSections.GlobalSections = nil
	withoutSections.RetiredGlobalSections = append([]string(nil), cfg.RetiredGlobalSections...)
	for _, section := range cfg.GlobalSections {
		withoutSections.RetiredGlobalSections = append(withoutSections.RetiredGlobalSections, section.Name)
	}
	if _, err := manageGlobalSections(&withoutSections, false); err != nil {
		return err
	}
	if _, err := applyManagedLocalPlugins(target, false, false); err != nil {
		return fmt.Errorf("managed local plugins: %w", err)
	}
	if err := cleanSettingsJSON(filepath.Join(target, "settings.json")); err != nil {
		return fmt.Errorf("settings: %w", err)
	}
	for _, lang := range cfg.AgentsMD.Languages {
		for _, dst := range docTargets(target, lang, cfg) {
			if err := os.Remove(dst); err == nil {
				fmt.Printf("  removed %s\n", dst)
				_ = os.Remove(filepath.Dir(dst))
			}
		}
	}
	if l := cfg.AgentsMD.Local; l.Target != "" {
		sections := append([]MDSection(nil), l.Sections...)
		sections = append(sections, MDSection{Name: "Language Conventions"})
		for _, s := range sections {
			if err := cleanSectionMD(l.Target, s.Name); err != nil {
				return fmt.Errorf("agents_md.local [%s]: %w", s.Name, err)
			}
		}
	}
	for _, skillsRoot := range skillTargets(cfg) {
		entries, err := os.ReadDir(skillsRoot)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("skills dir %s: %w", skillsRoot, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(skillsRoot, entry.Name())
			if !isManagedSkill(skillDir) || isExternalSkill(skillDir) {
				continue
			}
			if err := os.RemoveAll(skillDir); err != nil {
				return fmt.Errorf("remove managed skill %s: %w", skillDir, err)
			}
			fmt.Printf("  removed %s\n", skillDir)
		}
	}
	if _, err := removeManagedPiBackgroundTasks(cfg); err != nil {
		return fmt.Errorf("pi background tasks: %w", err)
	}
	if cfg.CodexHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.CodexHooksTarget)
		if _, err := codex.Remove(hooksPath); err != nil {
			return fmt.Errorf("codex hooks [%s]: %w", hooksPath, err)
		}
	}
	if cfg.AgyHooksTarget != "" {
		hooksPath := fsutil.ExpandHome(cfg.AgyHooksTarget)
		if _, err := agy.Remove(hooksPath); err != nil {
			return fmt.Errorf("agy hooks [%s]: %w", hooksPath, err)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		launcherPath := HarnezAgyLauncherPath(home)
		if err := os.Remove(launcherPath); err == nil {
			fmt.Printf("  removed %s\n", launcherPath)
		}
	}
	if shimPath := BashShimPath(); shimPath != "" {
		if err := os.Remove(shimPath); err == nil {
			fmt.Printf("  removed %s\n", shimPath)
			_ = os.Remove(filepath.Dir(shimPath))
		}
	}
	if envPath := HarnezEnvPath(); envPath != "" {
		if err := os.Remove(envPath); err == nil {
			fmt.Printf("  removed %s\n", envPath)
			_ = os.Remove(filepath.Dir(envPath))
		}
	}
	for _, rcPath := range ShellRCPaths() {
		if removed, _, err := markdown.CleanMK(rcPath, "env"); err == nil && removed {
			fmt.Printf("  cleaned %s [env]\n", rcPath)
		}
	}
	for _, link := range GearSymlinkTargets(target, cfg) {
		if err := os.Remove(link); err == nil {
			fmt.Printf("  removed %s\n", link)
			_ = os.Remove(filepath.Dir(link)) // best-effort clean empty bin dir
		}
	}
	return nil
}

// ExpandTarget expands targets.
func ExpandTarget(flag, cfgTarget string) string {
	if flag != "" {
		return flag
	}
	if cfgTarget != "" {
		return fsutil.ExpandHome(cfgTarget)
	}
	return DefaultTarget()
}

// OpenConfig opens target configuration.
func OpenConfig(configPath string) (*Config, string, error) {
	if configPath == "" {
		cfg, err := LoadConfigEmbedded()
		return cfg, "(embedded)", err
	}
	cfg, err := LoadConfig(configPath)
	return cfg, configPath, err
}

// ManagedSkillMarker marks skills installed from config.yaml by apply.
const ManagedSkillMarker = ".harnez-managed"

func isManagedSkill(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ManagedSkillMarker))
	return err == nil
}

// externalSkillClash reports (and prints) whether dir holds an external skill
// installed by `harnez skill install`. apply never overwrites or removes
// one; the user renames it instead (issue 631).
func externalSkillClash(dir string) bool {
	if !isExternalSkill(dir) {
		return false
	}
	name := filepath.Base(dir)
	fmt.Printf("  clash   %s is an external skill; kept it. Rename it: harnez skill remove %s && harnez skill install <url> --as <prefix>-%s\n", fsutil.ContractHome(dir), name, name)
	return true
}

// isExternalSkill reports whether dir carries the external-skill marker.
// diff, status, and revert skip such dirs silently, matching apply.
func isExternalSkill(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, skillreg.MarkerFile))
	return err == nil
}

// AgentSkillTarget is one agent's skill directory.
type AgentSkillTarget struct {
	Agent string // "gemini", "codex", "claude", "prime"
	Dir   string
}

// SkillTargetsByAgent returns the same directories as skillTargets, labelled
// by agent, for the external skill registry (internal/skillreg), which
// installs differently per agent. When agents share a dir, the claude or
// codex label wins, so the shared dir still receives an explicit-only copy.
func SkillTargetsByAgent(cfg *Config) []AgentSkillTarget {
	home, _ := os.UserHomeDir()
	pick := func(configured, fallback string) string {
		if root := fsutil.ExpandHome(configured); root != "" {
			return root
		}
		if fallback != "" && home != "" {
			return filepath.Join(home, fallback)
		}
		return ""
	}
	candidates := []AgentSkillTarget{
		{"gemini", pick(cfg.SkillsTarget, ".gemini/skills")},
		{"codex", pick(cfg.CodexSkillsTarget, "")},
		{"claude", pick(cfg.ClaudeSkillsTarget, ".claude/skills")},
	}
	if root := primeAgentRoot(cfg); root != "" {
		candidates = append(candidates, AgentSkillTarget{"prime", filepath.Join(root, "skills")})
	}
	if root := piAgentRoot(cfg); root != "" {
		candidates = append(candidates, AgentSkillTarget{"pi", filepath.Join(root, "skills")})
	}
	var out []AgentSkillTarget
	index := map[string]int{}
	for _, c := range candidates {
		if c.Dir == "" {
			continue
		}
		if i, ok := index[c.Dir]; ok {
			if c.Agent == "claude" || c.Agent == "codex" {
				out[i].Agent = c.Agent
			}
			continue
		}
		index[c.Dir] = len(out)
		out = append(out, c)
	}
	return out
}
