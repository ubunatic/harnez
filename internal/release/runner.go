package release

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Options configures the release workflow.
type Options struct {
	Dir         string
	Bump        string
	Continue    bool
	DryRun      bool
	Force       bool
	SignKey     string
	BuildCmd    string
	TagPrefix   string
	SkipBuild   bool
	SkipSign    bool
	SkipPublish bool
	SkipPush    bool
	Out         io.Writer
}

// Run executes the end-to-end release lifecycle.
func Run(opt Options) error {
	if opt.Dir == "" {
		opt.Dir = "."
	}
	absDir, err := filepath.Abs(opt.Dir)
	if err != nil {
		return fmt.Errorf("resolve dir %s: %w", opt.Dir, err)
	}
	opt.Dir = absDir

	if opt.Out == nil {
		opt.Out = os.Stdout
	}

	projectName := filepath.Base(opt.Dir)
	fmt.Fprintf(opt.Out, "==> Preparing release for %s (dry-run=%t, continue=%t)\n", projectName, opt.DryRun, opt.Continue)

	// 1. Preflight toolchain check
	if err := runPreflight(opt); err != nil {
		return err
	}

	// 2. Resolve & verify Minisign key
	var keyPath string
	if !opt.SkipSign {
		k, err := ResolveMinisignKey(opt.Dir, opt.SignKey)
		if err != nil {
			return err
		}
		keyPath = k
		fmt.Fprintf(opt.Out, "  [key]       Using minisign key: %s\n", keyPath)
		if !opt.DryRun {
			if err := TestMinisignKey(keyPath); err != nil {
				return fmt.Errorf("minisign key test failed: %w", err)
			}
		}
	}

	// 3. Clean working tree check (unless continue or dry-run)
	if !opt.Continue && !opt.DryRun {
		clean, err := isGitClean(opt.Dir)
		if err != nil {
			return fmt.Errorf("check git status: %w", err)
		}
		if !clean {
			return fmt.Errorf("git working tree is dirty; please commit or stash changes before releasing")
		}
	}

	// 4. Version determination & spec handling
	var targetVersion string
	var tagName string
	spec, specPath, _ := LoadVersionSpec(opt.Dir)

	tagPrefix := "v"
	if opt.TagPrefix != "" {
		tagPrefix = opt.TagPrefix
	} else if spec != nil && spec.TagPrefix != nil {
		tagPrefix = *spec.TagPrefix
	}

	if opt.Continue {
		if spec != nil && spec.Version != "" {
			targetVersion = spec.Version
		} else {
			detected, err := AutoDetectCurrentVersion(opt.Dir)
			if err != nil {
				return fmt.Errorf("detect version for --continue: %w", err)
			}
			targetVersion = detected
		}
		tagName = FormatTag(tagPrefix, targetVersion)
		fmt.Fprintf(opt.Out, "  [version]   Continuing release for version: %s (tag: %s)\n", targetVersion, tagName)
	} else {
		var currentVersion string
		if spec != nil && spec.Version != "" {
			currentVersion = spec.Version
		} else {
			detected, err := AutoDetectCurrentVersion(opt.Dir)
			if err != nil {
				return fmt.Errorf("detect current version: %w", err)
			}
			currentVersion = detected
		}

		cleanCurrent := strings.TrimPrefix(currentVersion, "v")
		if tagPrefix != "" && strings.HasPrefix(currentVersion, tagPrefix) {
			cleanCurrent = strings.TrimPrefix(currentVersion, tagPrefix)
		}

		if !opt.Force {
			prevTag := FormatTag(tagPrefix, cleanCurrent)
			exists, err := tagExists(opt.Dir, prevTag)
			if err != nil {
				return fmt.Errorf("check previous tag %s: %w", prevTag, err)
			}
			if exists {
				hasDiff, err := hasDiffSinceTag(opt.Dir, prevTag)
				if err != nil {
					return fmt.Errorf("check diff since %s: %w", prevTag, err)
				}
				if !hasDiff {
					fmt.Fprintf(opt.Out, "  [diff]      No changes since %s — nothing to release. Use --force to re-release the current code as a new version.\n", prevTag)
					return nil
				}
			}
		}

		bumped, err := BumpVersion(currentVersion, opt.Bump)
		if err != nil {
			return fmt.Errorf("bump version (%s -> %s): %w", currentVersion, opt.Bump, err)
		}
		targetVersion = bumped.String()
		tagName = FormatTag(tagPrefix, targetVersion)

		fmt.Fprintf(opt.Out, "  [version]   Bumping version: %s -> %s (tag: %s)\n", currentVersion, targetVersion, tagName)

		// Sync version spec & language files
		if spec == nil {
			spec = &VersionSpec{Version: targetVersion}
			if specPath == "" {
				specPath = filepath.Join(opt.Dir, "version.yaml")
			}
		} else {
			spec.Version = targetVersion
		}

		var changedFiles []string
		if !opt.DryRun {
			if err := SaveVersionSpec(specPath, spec); err != nil {
				return fmt.Errorf("save %s: %w", specPath, err)
			}
			changedFiles = append(changedFiles, specPath)

			syncRes, err := SyncLanguageFiles(opt.Dir, targetVersion, spec.Files)
			if err != nil {
				return fmt.Errorf("sync language files: %w", err)
			}
			changedFiles = append(changedFiles, syncRes.UpdatedFiles...)
			changedFiles = append(changedFiles, syncRes.CreatedFiles...)
			for _, uf := range syncRes.UpdatedFiles {
				fmt.Fprintf(opt.Out, "  [sync]      Updated %s\n", uf)
			}
			for _, cf := range syncRes.CreatedFiles {
				fmt.Fprintf(opt.Out, "  [sync]      Created %s\n", cf)
			}
		} else {
			fmt.Fprintf(opt.Out, "  [dry-run]   Would update %s to %s\n", specPath, targetVersion)
		}

		// Git commit & tag
		if !opt.DryRun {
			commitMsg := fmt.Sprintf("chore(release): bump version to %s", tagName)
			if err := gitAddAndCommit(opt.Dir, commitMsg, changedFiles); err != nil {
				return fmt.Errorf("git commit version bump: %w", err)
			}
			fmt.Fprintf(opt.Out, "  [git]       Committed version bump (%s)\n", commitMsg)

			tagMsg := fmt.Sprintf("%s %s", projectName, tagName)
			if err := gitCreateTag(opt.Dir, tagName, tagMsg); err != nil {
				return fmt.Errorf("git create tag %s: %w", tagName, err)
			}
			fmt.Fprintf(opt.Out, "  [git]       Created annotated tag: %s\n", tagName)
		} else {
			fmt.Fprintf(opt.Out, "  [dry-run]   Would commit version bump and create tag %s\n", tagName)
		}
	}

	// 5. Artifact Build & Packaging
	if !opt.SkipBuild {
		if err := runBuildStep(opt, spec); err != nil {
			return err
		}
	}

	// 6. Minisign Cryptographic Signing
	if !opt.SkipSign {
		if err := runSigningStep(opt, projectName, targetVersion, keyPath); err != nil {
			return err
		}
	}

	// 7. Check Forgejo / Codeberg Releases unit
	forge, err := DetectForgeInfo(opt.Dir)
	if err == nil && forge != nil {
		token := GetForgeToken(forge.Host)
		if token != "" {
			enabled, err := EnsureHasReleases(forge, token, opt.DryRun)
			if err != nil {
				fmt.Fprintf(opt.Out, "  [forge]     Warning: failed to check 'has_releases' unit: %v\n", err)
			} else if enabled {
				fmt.Fprintf(opt.Out, "  [forge]     Verified/Enabled 'has_releases' on %s/%s\n", forge.Owner, forge.Repo)
			}
		}
	}

	// 8. Remote Git Push
	if !opt.SkipPush {
		remoteName := "origin"
		if forge != nil && forge.RemoteName != "" {
			remoteName = forge.RemoteName
		}
		if !opt.DryRun {
			branch, err := getGitCurrentBranch(opt.Dir)
			if err != nil {
				return fmt.Errorf("get current branch: %w", err)
			}
			fmt.Fprintf(opt.Out, "  [git]       Pushing branch %s and tag %s to %s...\n", branch, tagName, remoteName)
			if err := runCmd(opt.Dir, "git", "push", remoteName, branch); err != nil {
				return fmt.Errorf("git push branch %s to %s: %w", branch, remoteName, err)
			}
			if err := runCmd(opt.Dir, "git", "push", remoteName, tagName); err != nil {
				return fmt.Errorf("git push tag %s to %s: %w", tagName, remoteName, err)
			}
		} else {
			fmt.Fprintf(opt.Out, "  [dry-run]   Would push branch and tag %s to %s\n", tagName, remoteName)
		}
	}

	// 9. Forge Publishing
	if !opt.SkipPublish {
		if err := runPublishStep(opt, projectName, tagName, forge); err != nil {
			return err
		}
	}

	fmt.Fprintf(opt.Out, "==> Release %s completed successfully!\n", tagName)
	return nil
}

func runPreflight(opt Options) error {
	if !opt.SkipPush || !opt.SkipPublish {
		if _, err := DetectForgeInfo(opt.Dir); err != nil {
			return err
		}
	}

	required := []string{"git"}
	if !opt.SkipSign {
		required = append(required, "minisign")
	}
	if !opt.SkipPublish {
		required = append(required, "fj")
	}
	buildCmd := opt.BuildCmd
	spec, _, _ := LoadVersionSpec(opt.Dir)
	if buildCmd == "" && spec != nil {
		buildCmd = spec.BuildCmd
	}
	if !opt.SkipBuild && buildCmd == "" {
		if fileExists(filepath.Join(opt.Dir, ".goreleaser.yaml")) || fileExists(filepath.Join(opt.Dir, ".goreleaser.yml")) {
			required = append(required, "goreleaser")
		} else if detectMakefileBuildTarget(opt.Dir) != "" {
			required = append(required, "make")
		}
	}

	for _, bin := range required {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("required tool %q not found in PATH", bin)
		}
	}

	return nil
}

func isGitClean(dir string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}

// tagExists reports whether tag exists in the repo at dir. Any failure to
// resolve it (missing tag, or dir not being a git repo at all — e.g. a
// brand-new project with no prior release) is treated as "does not exist"
// rather than a hard error, so first-ever releases proceed unimpeded.
func tagExists(dir, tag string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", "refs/tags/"+tag)
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// hasDiffSinceTag reports whether HEAD differs from tag's tree.
func hasDiffSinceTag(dir, tag string) (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet", tag, "HEAD")
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, err
}

func getGitCurrentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitAddAndCommit(dir, message string, files []string) error {
	args := append([]string{"add"}, files...)
	if err := runCmd(dir, "git", args...); err != nil {
		return err
	}
	return runCmd(dir, "git", "commit", "-m", message)
}

func gitCreateTag(dir, tagName, tagMsg string) error {
	return runCmd(dir, "git", "tag", "-a", tagName, "-m", tagMsg)
}

func runBuildStep(opt Options, spec *VersionSpec) error {
	buildCmd := opt.BuildCmd
	if buildCmd == "" && spec != nil {
		buildCmd = spec.BuildCmd
	}

	if buildCmd != "" {
		fmt.Fprintf(opt.Out, "  [build]     Running custom build command: %s\n", buildCmd)
		if !opt.DryRun {
			parts := strings.Fields(buildCmd)
			if err := runCmd(opt.Dir, parts[0], parts[1:]...); err != nil {
				return fmt.Errorf("build failed: %w", err)
			}
		}
		return nil
	}

	goreleaserYaml := filepath.Join(opt.Dir, ".goreleaser.yaml")
	if !fileExists(goreleaserYaml) {
		goreleaserYaml = filepath.Join(opt.Dir, ".goreleaser.yml")
	}

	if fileExists(goreleaserYaml) {
		args := []string{"release", "--clean", "--skip=publish", "--skip=sign"}
		if opt.Continue {
			args = append(args, "--skip=validate")
		}
		fmt.Fprintf(opt.Out, "  [build]     Running goreleaser %s\n", strings.Join(args, " "))
		if !opt.DryRun {
			if err := runCmd(opt.Dir, "goreleaser", args...); err != nil {
				return fmt.Errorf("goreleaser build failed: %w", err)
			}
		}
		return nil
	}

	if target := detectMakefileBuildTarget(opt.Dir); target != "" {
		cmdStr := "make " + target
		fmt.Fprintf(opt.Out, "  [build]     Running make target: %s\n", target)
		if !opt.DryRun {
			if err := runCmd(opt.Dir, "make", target); err != nil {
				return fmt.Errorf("build (%s) failed: %w", cmdStr, err)
			}
		}
		return nil
	}

	fmt.Fprintf(opt.Out, "  [build]     No .goreleaser.yaml found and no --build-cmd specified; skipping artifact build\n")
	return nil
}

func detectMakefileBuildTarget(dir string) string {
	makefilePath := filepath.Join(dir, "Makefile")
	if !fileExists(makefilePath) {
		makefilePath = filepath.Join(dir, "makefile")
		if !fileExists(makefilePath) {
			makefilePath = filepath.Join(dir, "GNUmakefile")
			if !fileExists(makefilePath) {
				return ""
			}
		}
	}

	for _, target := range []string{"pack", "dist"} {
		if hasMakefileTarget(dir, target) {
			return target
		}
	}
	return ""
}

func hasMakefileTarget(dir, target string) bool {
	cmd := exec.Command("make", "-n", target)
	cmd.Dir = dir
	if err := cmd.Run(); err == nil {
		return true
	}

	for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
		p := filepath.Join(dir, name)
		data, err := os.ReadFile(p)
		if err == nil {
			targetRegex := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(target) + `\s*:`)
			if targetRegex.Match(data) {
				return true
			}
		}
	}
	return false
}

func runSigningStep(opt Options, projectName, version, keyPath string) error {
	checksumPath := filepath.Join(opt.Dir, "dist", "SHA256SUMS")
	if !fileExists(checksumPath) {
		fmt.Fprintf(opt.Out, "  [sign]      No dist/SHA256SUMS found to sign\n")
		return nil
	}

	fmt.Fprintf(opt.Out, "  [sign]      Signing %s with minisign\n", checksumPath)
	if !opt.DryRun {
		comment := fmt.Sprintf("%s %s", projectName, version)
		sigFile, err := SignFile(keyPath, checksumPath, comment)
		if err != nil {
			return fmt.Errorf("signing %s: %w", checksumPath, err)
		}
		fmt.Fprintf(opt.Out, "  [sign]      Created signature %s\n", sigFile)
	}
	return nil
}

func runPublishStep(opt Options, projectName, tagName string, forge *ForgeInfo) error {
	distDir := filepath.Join(opt.Dir, "dist")
	if !fileExists(distDir) {
		if opt.DryRun {
			fmt.Fprintf(opt.Out, "  [dry-run]   Would publish release %s via fj release\n", tagName)
			return nil
		}
		return fmt.Errorf("dist directory %s does not exist; cannot publish", distDir)
	}

	entries, err := os.ReadDir(distDir)
	if err != nil {
		return fmt.Errorf("read dist directory: %w", err)
	}

	var attachments []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".tar.gz") ||
			strings.HasSuffix(name, ".zip") ||
			strings.HasSuffix(name, ".minisig") ||
			name == "SHA256SUMS" ||
			strings.HasPrefix(name, projectName+"-") {
			attachments = append(attachments, filepath.Join("dist", name))
		}
	}

	if len(attachments) == 0 {
		if opt.DryRun {
			fmt.Fprintf(opt.Out, "  [dry-run]   Would publish release %s via fj release\n", tagName)
			return nil
		}
		return fmt.Errorf("no release artifacts found in dist/")
	}

	title := fmt.Sprintf("%s %s", projectName, tagName)
	fmt.Fprintf(opt.Out, "  [publish]   Publishing %d asset(s) via fj release to %s\n", len(attachments), tagName)
	for _, a := range attachments {
		fmt.Fprintf(opt.Out, "              - %s\n", a)
	}

	if !opt.DryRun {
		remoteName := ""
		if forge != nil {
			remoteName = forge.RemoteName
		}
		if err := PublishForgejoRelease(opt.Dir, tagName, title, attachments, remoteName, opt.DryRun, opt.Continue); err != nil {
			return err
		}
	}
	return nil
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
