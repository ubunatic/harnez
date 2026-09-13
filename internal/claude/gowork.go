package claude

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const managedWorkspaceMarker = "// harnez:managed"

func isManagedWorkspace(dir string) (isManaged bool, hasExample bool, hasWork bool, isSymlink bool, isTracked bool) {
	examplePath := filepath.Join(dir, "go.work.example")
	workPath := filepath.Join(dir, "go.work")

	if data, err := os.ReadFile(examplePath); err == nil {
		hasExample = true
		if strings.Contains(string(data), managedWorkspaceMarker) {
			isManaged = true
		}
	}

	if fi, err := os.Lstat(workPath); err == nil {
		hasWork = true
		if fi.Mode()&os.ModeSymlink != 0 {
			isSymlink = true
			if target, err := os.Readlink(workPath); err == nil && (target == "go.work.example" || strings.HasSuffix(target, "go.work.example")) {
				isManaged = true
			}
		} else if data, err := os.ReadFile(workPath); err == nil {
			if strings.Contains(string(data), managedWorkspaceMarker) {
				isManaged = true
			}
		}
	}

	if hasWork && !isSymlink {
		cmd := exec.Command("git", "ls-files", "--error-unmatch", "go.work")
		cmd.Dir = dir
		if err := cmd.Run(); err == nil {
			isTracked = true
		}
	}

	return
}

func ensureGitignoreEntries(dir string, entries ...string) (bool, error) {
	gitignorePath := filepath.Join(dir, ".gitignore")
	var existing []byte
	if data, err := os.ReadFile(gitignorePath); err == nil {
		existing = data
	} else if !os.IsNotExist(err) {
		return false, err
	}

	content := string(existing)
	var toAdd []string
	for _, entry := range entries {
		pattern := strings.TrimPrefix(entry, "/")
		if !strings.Contains(content, entry) && !strings.Contains(content, pattern) {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return false, nil
	}

	var b bytes.Buffer
	b.Write(existing)
	if len(existing) > 0 && !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n# local Go workspace (untracked; see go.work.example)\n")
	for _, entry := range toAdd {
		b.WriteString(entry + "\n")
	}

	if err := os.WriteFile(gitignorePath, b.Bytes(), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func detectGoVersion(dir string) string {
	goModPath := filepath.Join(dir, "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		re := regexp.MustCompile(`(?m)^go\s+([0-9.]+)`)
		if m := re.FindStringSubmatch(string(data)); len(m) > 1 {
			return m[1]
		}
	}
	return "1.26.5"
}

func generateGoWorkExample(dir string, moduleArgs []string) string {
	version := detectGoVersion(dir)
	var b strings.Builder
	b.WriteString(managedWorkspaceMarker + "\n")
	b.WriteString("// Example Go workspace for local co-development.\n")
	b.WriteString("// Symlinked or copied to go.work locally (untracked).\n")
	b.WriteString(fmt.Sprintf("go %s\n\n", version))
	b.WriteString("use (\n")
	for _, arg := range moduleArgs {
		b.WriteString(fmt.Sprintf("\t%s\n", arg))
	}
	b.WriteString(")\n")
	return b.String()
}

func reconcileGoWorkspace(dir string, optIn bool) (bool, error) {
	moduleDirs, moduleArgs, err := findGoModules(dir)
	if err != nil {
		return false, err
	}
	if len(moduleDirs) == 0 {
		return false, nil
	}

	examplePath := filepath.Join(dir, "go.work.example")
	workPath := filepath.Join(dir, "go.work")

	isManaged, hasExample, hasWork, isSymlink, isTracked := isManagedWorkspace(dir)

	// If not managed and opt-in was NOT requested
	if !isManaged && !optIn {
		if hasWork && isTracked {
			fmt.Printf("  go.work skipped: go.work is tracked in git (use --gowork or add '%s' to opt in)\n", managedWorkspaceMarker)
			return false, nil
		}
		if hasWork {
			fmt.Printf("  go.work skipped: local untracked go.work exists (use --gowork or add '%s' to opt in)\n", managedWorkspaceMarker)
			return false, nil
		}
		// No go.work and no go.work.example: default is skip
		return false, nil
	}

	var changed bool

	// Case 1: Tracked go.work being migrated with opt-in
	if hasWork && isTracked && optIn && !isManaged {
		workData, err := os.ReadFile(workPath)
		if err != nil {
			return false, fmt.Errorf("read %s: %w", workPath, err)
		}
		exampleContent := managedWorkspaceMarker + "\n" + string(workData)
		if err := os.WriteFile(examplePath, []byte(exampleContent), 0o644); err != nil {
			return false, fmt.Errorf("write %s: %w", examplePath, err)
		}
		gitRm := exec.Command("git", "rm", "--cached", "go.work")
		gitRm.Dir = dir
		_ = gitRm.Run()
		_ = os.Remove(workPath)
		if err := os.Symlink("go.work.example", workPath); err != nil {
			return false, fmt.Errorf("symlink %s: %w", workPath, err)
		}
		if gitignoreChanged, err := ensureGitignoreEntries(dir, "/go.work", "/go.work.sum"); err == nil && gitignoreChanged {
			changed = true
		}
		fmt.Printf("  migrated tracked go.work to go.work.example and created symlink\n")
		return true, nil
	}

	// Case 2: Untracked go.work being adopted
	if hasWork && !isSymlink && (!hasExample || optIn) {
		workData, err := os.ReadFile(workPath)
		if err != nil {
			return false, fmt.Errorf("read %s: %w", workPath, err)
		}
		exampleContent := string(workData)
		if !strings.Contains(exampleContent, managedWorkspaceMarker) {
			exampleContent = managedWorkspaceMarker + "\n" + exampleContent
		}
		if err := os.WriteFile(examplePath, []byte(exampleContent), 0o644); err != nil {
			return false, fmt.Errorf("write %s: %w", examplePath, err)
		}
		_ = os.Remove(workPath)
		if err := os.Symlink("go.work.example", workPath); err != nil {
			return false, fmt.Errorf("symlink %s: %w", workPath, err)
		}
		if gitignoreChanged, err := ensureGitignoreEntries(dir, "/go.work", "/go.work.sum"); err == nil && gitignoreChanged {
			changed = true
		}
		fmt.Printf("  migrated go.work to go.work.example and created symlink\n")
		return true, nil
	}

	// Case 3: Need to create go.work.example from scratch
	if !hasExample {
		exampleContent := generateGoWorkExample(dir, moduleArgs)
		if err := os.WriteFile(examplePath, []byte(exampleContent), 0o644); err != nil {
			return false, fmt.Errorf("write %s: %w", examplePath, err)
		}
		changed = true
	}

	// Case 4: Ensure symlink
	if !hasWork {
		if err := os.Symlink("go.work.example", workPath); err != nil {
			return false, fmt.Errorf("symlink %s: %w", workPath, err)
		}
		changed = true
		fmt.Printf("  created symlink %s -> go.work.example\n", workPath)
	} else if isSymlink {
		target, err := os.Readlink(workPath)
		if err != nil || (target != "go.work.example" && target != examplePath) {
			_ = os.Remove(workPath)
			if err := os.Symlink("go.work.example", workPath); err != nil {
				return false, fmt.Errorf("recreate symlink %s: %w", workPath, err)
			}
			changed = true
		}
	}

	// Ensure .gitignore
	if gitignoreChanged, err := ensureGitignoreEntries(dir, "/go.work", "/go.work.sum"); err != nil {
		return false, err
	} else if gitignoreChanged {
		changed = true
	}

	return changed, nil
}

func findGoModules(root string) ([]string, []string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path != root {
				return filepath.SkipDir
			}
			return walkErr
		}
		if entry.IsDir() && path != root && ignoredModuleScanDir(entry.Name()) {
			return filepath.SkipDir
		}
		if !entry.IsDir() && entry.Name() == "go.mod" {
			dirs = append(dirs, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("scan Go modules in %s: %w", root, err)
	}
	sort.Strings(dirs)
	args := make([]string, 0, len(dirs))
	for _, moduleDir := range dirs {
		rel, err := filepath.Rel(root, moduleDir)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve Go module %s: %w", moduleDir, err)
		}
		if rel == "." {
			args = append(args, ".")
		} else {
			args = append(args, "./"+filepath.ToSlash(rel))
		}
	}
	return dirs, args, nil
}

func ignoredModuleScanDir(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	switch name {
	case "node_modules", "vendor", "testdata":
		return true
	default:
		return false
	}
}

