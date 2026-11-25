// Package selfinstall deploys the harnez binary via ubunatic.com/goself and
// provisions local shell PATH, completion scripts, man pages, and symlinks.
package selfinstall

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ubunatic.com/goself"
	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/server"
)

const (
	// DefaultDir is the default binary install directory passed to goself.
	DefaultDir = "~/.local/bin"

	releaseBaseURL = "https://codeberg.org/ubunatic/harnez/releases/download/v{version}"
)

// Options configures one harnez install or update run.
type Options struct {
	Host           string
	Strategy       string
	Dir            string
	Version        string
	AllowDirty     bool
	Direct         bool
	DryRun         bool
	NoShell        bool
	NoMan          bool
	Home           string
	GoBinDir       string // optional override for goself.InstallDir() (tests)
	InstallMan     func(io.Writer) error
	RefreshService func(context.Context, string, io.Writer) error
}

// Run installs or updates the harnez binary locally or on Options.Host and, for
// local installs, updates shell PATH, completions, gear symlink, man pages, and
// any active collector service, and warns when PATH finds another harnez first.
func Run(ctx context.Context, opts Options, out io.Writer) error {
	strategy, err := goself.ParseStrategy(opts.Strategy)
	if err != nil {
		return err
	}
	dir := opts.Dir
	if dir == "" {
		dir = DefaultDir
	}
	if opts.DryRun {
		printDryRunPlan(out, opts, strategy, dir)
		return nil
	}

	target := goself.Target{Dir: dir, Name: "harnez"}
	if opts.Host != "" {
		target.Runner = goself.SSH(opts.Host)
	}
	deployOpts := goself.Options{
		Strategy:   strategy,
		Version:    opts.Version,
		AllowDirty: opts.AllowDirty,
		Direct:     opts.Direct,
		Release: goself.ReleaseSource{
			BaseURL:   releaseBaseURL,
			Archive:   ReleaseArchive,
			Checksums: "SHA256SUMS",
		},
	}

	res, err := goself.New(deployOpts).Deploy(ctx, target)
	if err != nil {
		if errors.Is(err, goself.ErrUncommittedChanges) {
			return fmt.Errorf("%w (pass --allow-dirty to deploy anyway)", err)
		}
		return err
	}
	fmt.Fprintf(out, "installed %s (%s, %s)\n", fsutil.ContractHome(res.Path), res.Platform, res.Strategy)
	for _, w := range res.Warnings {
		fmt.Fprintf(out, "  warning: %s\n", w)
	}

	if opts.Host != "" {
		return nil
	}
	return setupLocalUX(ctx, opts, res.Path, out)
}

// ReleaseArchive maps goself platform names to harnez GoReleaser archive names.
func ReleaseArchive(name, version string, p goself.Platform) string {
	arch := p.Arch
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		if p.OS == "linux" {
			arch = "aarch64"
		}
	}
	return fmt.Sprintf("%s-%s-%s-%s.tar.gz", name, version, arch, p.OS)
}

func printDryRunPlan(out io.Writer, opts Options, strategy goself.Strategy, dir string) {
	where := "locally"
	if opts.Host != "" {
		where = "on " + opts.Host
	}
	fmt.Fprintf(out, "would install harnez %s into %s (strategy=%s)\n", where, dir, strategy)
	if opts.Host != "" {
		return
	}
	fmt.Fprintf(out, "would ensure gear symlink %s\n", filepath.Join(dir, "⚙"))
	if !opts.NoShell {
		fmt.Fprintf(out, "would configure shell PATH (%s) and bash/zsh/fish completions\n", dir)
	}
	if !opts.NoMan && opts.InstallMan != nil {
		fmt.Fprintln(out, "would install roff man pages")
	}
	fmt.Fprintln(out, "would restart active harnez-agent-collector.service if installed")
}

func setupLocalUX(ctx context.Context, opts Options, binPath string, out io.Writer) error {
	binDir := filepath.Dir(binPath)
	link, changed, err := ensureGearSymlink(binPath)
	if err != nil {
		return err
	}
	if changed {
		fmt.Fprintf(out, "  symlink %s -> %s\n", fsutil.ContractHome(link), filepath.Base(binPath))
	}

	goBinDir := opts.GoBinDir
	if goBinDir == "" {
		goBinDir = goself.InstallDir()
	}
	if goLink, goChanged, err := ensureGoBinSymlink(binPath, goBinDir); err != nil {
		return err
	} else if goChanged {
		fmt.Fprintf(out, "  symlink %s -> %s\n", fsutil.ContractHome(goLink), fsutil.ContractHome(binPath))
	}

	if !opts.NoShell {
		if err := applyShellSetup(opts.Home, binDir, out); err != nil {
			return err
		}
	}

	if !opts.NoMan && opts.InstallMan != nil {
		if err := opts.InstallMan(out); err != nil {
			return err
		}
	}

	refresh := opts.RefreshService
	if refresh == nil {
		refresh = refreshSystemdCollector
	}
	if err := refresh(ctx, binPath, out); err != nil {
		return err
	}
	if w := shadowWarning(binPath); w != "" {
		fmt.Fprintln(out, w)
	}
	return nil
}

// shadowWarning reports when the harnez that PATH finds first is not binPath,
// so a stale copy elsewhere (such as GOPATH/bin) would keep running (issue 1245).
func shadowWarning(binPath string) string {
	found, err := exec.LookPath(filepath.Base(binPath))
	if err != nil {
		return ""
	}
	if sameFile(found, binPath) {
		return ""
	}
	return fmt.Sprintf("  warning: harnez on PATH is %s, not %s; remove it or put %s first on PATH",
		fsutil.ContractHome(found), fsutil.ContractHome(binPath), fsutil.ContractHome(filepath.Dir(binPath)))
}

func sameFile(a, b string) bool {
	ai, err := os.Stat(a)
	if err != nil {
		return false
	}
	bi, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(ai, bi)
}

func ensureGearSymlink(binPath string) (string, bool, error) {
	binDir := filepath.Dir(binPath)
	target := filepath.Base(binPath)
	link := filepath.Join(binDir, "⚙")
	if current, err := os.Readlink(link); err == nil && current == target {
		return link, false, nil
	}
	tmp := filepath.Join(binDir, fmt.Sprintf(".harnez-gear-%d.tmp", os.Getpid()))
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		return "", false, fmt.Errorf("create gear symlink: %w", err)
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return "", false, fmt.Errorf("install gear symlink %s: %w", link, err)
	}
	return link, true, nil
}

// ensureGoBinSymlink points <goBinDir>/<name> at binPath when goBinDir exists
// and differs from filepath.Dir(binPath), replacing any stale go-install copy
// atomically so PATH entries with ~/go/bin ahead of ~/.local/bin stay current.
func ensureGoBinSymlink(binPath, goBinDir string) (string, bool, error) {
	if goBinDir == "" || sameFile(goBinDir, filepath.Dir(binPath)) {
		return "", false, nil
	}
	info, err := os.Stat(goBinDir)
	if err != nil || !info.IsDir() {
		return "", false, nil
	}
	link := filepath.Join(goBinDir, filepath.Base(binPath))
	if current, err := os.Readlink(link); err == nil && current == binPath {
		return link, false, nil
	}
	if linfo, err := os.Lstat(link); err == nil && linfo.IsDir() {
		return "", false, fmt.Errorf("cannot replace directory %s with symlink to %s", link, binPath)
	}
	tmp := filepath.Join(goBinDir, fmt.Sprintf(".%s-link-%d.tmp", filepath.Base(binPath), os.Getpid()))
	_ = os.Remove(tmp)
	if err := os.Symlink(binPath, tmp); err != nil {
		return "", false, fmt.Errorf("create gobin symlink: %w", err)
	}
	if err := os.Rename(tmp, link); err != nil {
		_ = os.Remove(tmp)
		return "", false, fmt.Errorf("install gobin symlink %s: %w", link, err)
	}
	return link, true, nil
}

func applyShellSetup(home, binDir string, out io.Writer) error {
	inst, err := claude.NewShellSetup(home, binDir)
	if err != nil {
		return fmt.Errorf("shell setup: %w", err)
	}
	res, err := inst.Apply()
	if err != nil {
		return err
	}
	for _, c := range res.Changes {
		fmt.Fprintf(out, "  %s\n", claude.DescribeShellChange(c))
	}
	if len(res.Changes) == 0 && len(res.Unchanged) > 0 {
		fmt.Fprintf(out, "  shell setup ok (%d target(s) up to date)\n", len(res.Unchanged))
	}
	for _, c := range inst.Skipped() {
		if strings.HasSuffix(c.Reason, "does not exist") {
			fmt.Fprintf(out, "  skipped %s (%s)\n", c.Label, c.Reason)
		}
	}
	if res.PathMissing && claude.PathEntryDir(binDir) != "" {
		fmt.Fprintf(out, "  note: %s is not on this shell's PATH; open a new terminal or source your shell rc so harnez is reachable\n", fsutil.ContractHome(binDir))
	}
	return nil
}

func refreshSystemdCollector(ctx context.Context, binPath string, out io.Writer) error {
	cfg, err := server.LoadConfig()
	if err != nil {
		return err
	}
	systemd := server.NewSystemdService(cfg)
	status, err := systemd.Status(ctx, 0)
	if err != nil || !status.Available || !status.Installed || !status.Active {
		return nil
	}
	systemd.Executable = binPath
	if _, err := systemd.Install(ctx); err != nil {
		return err
	}
	if _, err := systemd.Restart(ctx); err != nil {
		return err
	}
	fmt.Fprintf(out, "  restarted %s (%s)\n", cfg.SystemdUnitName, fsutil.ContractHome(binPath))
	return nil
}
