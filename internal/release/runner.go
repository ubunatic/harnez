package release

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options configures the release workflow.
type Options struct {
	Dir         string
	Bump        string
	Continue    bool
	DryRun      bool
	SignKey     string
	BuildCmd    string
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
		tagName = "v" + strings.TrimPrefix(targetVersion, "v")
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

		bumped, err := BumpVersion(currentVersion, opt.Bump)
		if err != nil {
			return fmt.Errorf("bump version (%s -> %s): %w", currentVersion, opt.Bump, err)
		}
		targetVersion = bumped.String()
		tagName = bumped.TagName()

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
		if err := runBuildStep(opt); err != nil {
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
		if !opt.DryRun {
			branch, err := getGitCurrentBranch(opt.Dir)
			if err != nil {
				return fmt.Errorf("get current branch: %w", err)
			}
			fmt.Fprintf(opt.Out, "  [git]       Pushing branch %s and tag %s to origin...\n", branch, tagName)
			if err := runCmd(opt.Dir, "git", "push", "origin", branch); err != nil {
				return fmt.Errorf("git push branch %s: %w", branch, err)
			}
			if err := runCmd(opt.Dir, "git", "push", "origin", tagName); err != nil {
				return fmt.Errorf("git push tag %s: %w", tagName, err)
			}
		} else {
			fmt.Fprintf(opt.Out, "  [dry-run]   Would push branch and tag %s to origin\n", tagName)
		}
	}

	// 9. Forge Publishing
	if !opt.SkipPublish {
		if err := runPublishStep(opt, projectName, tagName); err != nil {
			return err
		}
	}

	fmt.Fprintf(opt.Out, "==> Release %s completed successfully!\n", tagName)
	return nil
}

func runPreflight(opt Options) error {
	required := []string{"git"}
	if !opt.SkipSign {
		required = append(required, "minisign")
	}
	if !opt.SkipPublish {
		required = append(required, "fj")
	}
	if !opt.SkipBuild && opt.BuildCmd == "" {
		if fileExists(filepath.Join(opt.Dir, ".goreleaser.yaml")) || fileExists(filepath.Join(opt.Dir, ".goreleaser.yml")) {
			required = append(required, "goreleaser")
		}
	}

	for _, bin := range required {
		if _, err := exec.LookPath(bin); err != nil {
			return fmt.Errorf("required tool %q not found in PATH", bin)
		}
	}

	if !opt.SkipPush || !opt.SkipPublish {
		if _, err := DetectForgeInfo(opt.Dir); err != nil {
			return err
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

func runBuildStep(opt Options) error {
	if opt.BuildCmd != "" {
		fmt.Fprintf(opt.Out, "  [build]     Running custom build command: %s\n", opt.BuildCmd)
		if !opt.DryRun {
			parts := strings.Fields(opt.BuildCmd)
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

	fmt.Fprintf(opt.Out, "  [build]     No .goreleaser.yaml found and no --build-cmd specified; skipping artifact build\n")
	return nil
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

func runPublishStep(opt Options, projectName, tagName string) error {
	distDir := filepath.Join(opt.Dir, "dist")
	if !fileExists(distDir) {
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
		return fmt.Errorf("no release artifacts found in dist/")
	}

	title := fmt.Sprintf("%s %s", projectName, tagName)
	fmt.Fprintf(opt.Out, "  [publish]   Publishing %d asset(s) via fj release to %s\n", len(attachments), tagName)
	for _, a := range attachments {
		fmt.Fprintf(opt.Out, "              - %s\n", a)
	}

	if !opt.DryRun {
		if err := PublishForgejoRelease(opt.Dir, tagName, title, attachments, opt.DryRun, opt.Continue); err != nil {
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
