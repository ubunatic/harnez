package claude

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type goWorkAction uint8

const (
	goWorkSkip goWorkAction = iota
	goWorkCreate
)

type goWorkPlan struct {
	action     goWorkAction
	targetDir  string
	workspace  string
	moduleDirs []string
	moduleArgs []string
	reason     string
}

type goCommandRunner func(dir string, args ...string) (string, error)

func runGoCommand(dir string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	return combinedCommandOutput(cmd)
}

func runGoCommandWithoutWorkspace(dir string, args ...string) (string, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GOWORK=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GOWORK=off")
	return combinedCommandOutput(cmd)
}

func combinedCommandOutput(cmd *exec.Cmd) (string, error) {
	out, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(out)), err
	}
	return strings.TrimSpace(string(out)), nil
}

func planGoWorkspace(dir string, run goCommandRunner) (goWorkPlan, error) {
	targetDir, err := filepath.Abs(dir)
	if err != nil {
		return goWorkPlan{}, fmt.Errorf("resolve project directory: %w", err)
	}
	targetDir = cleanExistingPath(targetDir)
	localWork := filepath.Join(targetDir, "go.work")
	if _, err := os.Lstat(localWork); err == nil {
		return goWorkPlan{targetDir: targetDir, workspace: localWork, reason: "local go.work already exists"}, nil
	} else if !os.IsNotExist(err) {
		return goWorkPlan{}, fmt.Errorf("stat %s: %w", localWork, err)
	}

	moduleDirs, moduleArgs, err := findGoModules(targetDir)
	if err != nil {
		return goWorkPlan{}, err
	}
	if len(moduleDirs) == 0 {
		return goWorkPlan{targetDir: targetDir, reason: "no Go modules found"}, nil
	}

	activeWork, probeErr := run(targetDir, "env", "GOWORK")
	if probeErr != nil {
		return goWorkPlan{targetDir: targetDir, moduleDirs: moduleDirs, moduleArgs: moduleArgs,
			reason: fmt.Sprintf("Go workspace probe failed: %v%s", probeErr, commandOutputSuffix(activeWork))}, nil
	}
	activeWork = strings.TrimSpace(activeWork)
	if activeWork == "off" {
		return goWorkPlan{targetDir: targetDir, moduleDirs: moduleDirs, moduleArgs: moduleArgs,
			reason: "Go workspace discovery is disabled by GOWORK=off"}, nil
	}
	if activeWork == "" {
		return goWorkPlan{targetDir: targetDir, moduleDirs: moduleDirs, moduleArgs: moduleArgs,
			reason: "Go reports no active workspace"}, nil
	}

	activeWork = cleanExistingPath(activeWork)
	if configured := strings.TrimSpace(os.Getenv("GOWORK")); configured != "" {
		return goWorkPlan{targetDir: targetDir, workspace: activeWork, moduleDirs: moduleDirs, moduleArgs: moduleArgs,
			reason: "GOWORK explicitly selects a workspace, so a local file would not override it"}, nil
	}
	if !containsPath(filepath.Dir(activeWork), targetDir) || filepath.Dir(activeWork) == targetDir {
		return goWorkPlan{targetDir: targetDir, workspace: activeWork, moduleDirs: moduleDirs, moduleArgs: moduleArgs,
			reason: "active go.work is not an enclosing parent workspace"}, nil
	}

	workspaceJSON, probeErr := run(targetDir, "work", "edit", "-json")
	if probeErr != nil {
		return goWorkPlan{targetDir: targetDir, workspace: activeWork, moduleDirs: moduleDirs, moduleArgs: moduleArgs,
			reason: fmt.Sprintf("parent workspace membership probe failed: %v%s", probeErr, commandOutputSuffix(workspaceJSON))}, nil
	}
	included, err := parseWorkspaceModules(activeWork, workspaceJSON)
	if err != nil {
		return goWorkPlan{targetDir: targetDir, workspace: activeWork, moduleDirs: moduleDirs, moduleArgs: moduleArgs,
			reason: fmt.Sprintf("parent workspace membership probe returned invalid JSON: %v", err)}, nil
	}
	for _, moduleDir := range moduleDirs {
		if !included[cleanExistingPath(moduleDir)] {
			return goWorkPlan{
				action: goWorkCreate, targetDir: targetDir, workspace: activeWork,
				moduleDirs: moduleDirs, moduleArgs: moduleArgs,
				reason: "an enclosing workspace omits one or more project Go modules",
			}, nil
		}
	}
	return goWorkPlan{targetDir: targetDir, workspace: activeWork, moduleDirs: moduleDirs, moduleArgs: moduleArgs,
		reason: "all project Go modules are already included in the enclosing workspace"}, nil
}

func reconcileGoWorkspace(dir string) (bool, error) {
	plan, err := planGoWorkspace(dir, runGoCommand)
	if err != nil {
		return false, err
	}
	if plan.action != goWorkCreate {
		fmt.Printf("  go.work skipped: %s\n", plan.reason)
		return false, nil
	}
	if output, err := runGoCommandWithoutWorkspace(plan.targetDir, append([]string{"work", "init"}, plan.moduleArgs...)...); err != nil {
		return false, fmt.Errorf("create %s: go work init: %w%s", filepath.Join(plan.targetDir, "go.work"), err, commandOutputSuffix(output))
	}
	fmt.Printf("  created %s (%d module(s); isolates from %s)\n",
		filepath.Join(plan.targetDir, "go.work"), len(plan.moduleDirs), plan.workspace)
	return true, nil
}

func findGoModules(root string) ([]string, []string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
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
	switch name {
	case ".git", ".hg", ".svn", "node_modules", "vendor":
		return true
	default:
		return false
	}
}

func parseWorkspaceModules(workFile, data string) (map[string]bool, error) {
	var parsed struct {
		Use []struct {
			DiskPath string
		}
	}
	if err := json.Unmarshal([]byte(data), &parsed); err != nil {
		return nil, err
	}
	included := make(map[string]bool, len(parsed.Use))
	base := filepath.Dir(workFile)
	for _, use := range parsed.Use {
		path := use.DiskPath
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		included[cleanExistingPath(path)] = true
	}
	return included, nil
}

func cleanExistingPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	return path
}

func containsPath(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func commandOutputSuffix(output string) string {
	if output == "" {
		return ""
	}
	return ": " + output
}
