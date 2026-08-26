package claude

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ubunatic.com/harnez/internal/markdown"
)

type docsDriftFile struct {
	name   string
	path   string
	status string
	diff   string
}

type docsScanRepo struct {
	name  string
	files []docsDriftFile
}

// ScanDocs compares managed docs in one eligible repository or its immediate
// child repositories and returns a read-only report.
func ScanDocs(target string, cfg *Config) (string, error) {
	if target == "" {
		return "", errors.New("scan directory is empty")
	}
	target, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve scan target: %w", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		return "", fmt.Errorf("stat scan target: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("scan target %s is not a directory", target)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return "", fmt.Errorf("read scan directory: %w", err)
	}
	var repos []docsScanRepo
	skipped := 0
	for _, entry := range entries {
		child := filepath.Join(target, entry.Name())
		childInfo, statErr := os.Stat(child)
		if statErr != nil || !childInfo.IsDir() || !hasAgentDoc(child) {
			skipped++
			continue
		}
		files, compareErr := compareConfiguredDocs(child, cfg)
		if compareErr != nil {
			return "", fmt.Errorf("scan %s: %w", child, compareErr)
		}
		repos = append(repos, docsScanRepo{name: entry.Name(), files: files})
	}
	if len(repos) > 0 {
		sort.Slice(repos, func(i, j int) bool { return repos[i].name < repos[j].name })
		return renderDocsScanSummary(repos, skipped), nil
	}
	if hasAgentDoc(target) {
		files, err := compareConfiguredDocs(target, cfg)
		if err != nil {
			return "", err
		}
		return renderDocsScanRepo(target, files, cfg), nil
	}
	return renderDocsScanSummary(nil, skipped), nil
}

func hasAgentDoc(dir string) bool {
	for _, name := range []string{"AGENTS.md", "CLAUDE.md"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func docsDriftReportFiles(files []docsDriftFile) []docsDriftFile {
	filtered := make([]docsDriftFile, 0, len(files))
	for _, file := range files {
		if file.status != "missing" {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func docsDriftChanged(files []docsDriftFile) bool {
	for _, file := range docsDriftReportFiles(files) {
		if file.status != "identical" {
			return true
		}
	}
	return false
}

func renderDocsScanRepo(repoDir string, files []docsDriftFile, cfg *Config) string {
	return renderDocsDriftReportOptions(docsDriftSourceRepo(repoDir, cfg), repoDir, docsDriftReportFiles(files), false)
}

func renderDocsScanSummary(repos []docsScanRepo, skipped int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Scanned: %d eligible, %d skipped\n", len(repos), skipped)
	var clean []string
	for _, repo := range repos {
		if !docsDriftChanged(repo.files) {
			clean = append(clean, repo.name)
		}
	}
	if len(clean) > 0 {
		fmt.Fprintf(&b, "Clean: %s\n", strings.Join(clean, ", "))
	}
	var drift []docsScanRepo
	for _, repo := range repos {
		if docsDriftChanged(repo.files) {
			drift = append(drift, repo)
		}
	}
	if len(drift) > 0 {
		b.WriteString("Drift:\n")
		for _, repo := range drift {
			var names []string
			for _, file := range docsDriftReportFiles(repo.files) {
				if file.status != "identical" {
					names = append(names, fmt.Sprintf("%s (%s)", file.name, file.status))
				}
			}
			fmt.Fprintf(&b, "  %s  %s\n", repo.name, strings.Join(names, ", "))
		}
	}
	return b.String()
}

// CaptureDocsDrift compares configured source docs with their project-local copies
// and writes a lightweight Markdown report. It does not modify either source or
// project docs.
func CaptureDocsDrift(repoDir, outputPath string, cfg *Config) (string, bool, error) {
	if repoDir == "" {
		return "", false, errors.New("current repository path is empty")
	}
	repoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return "", false, fmt.Errorf("resolve repository path: %w", err)
	}
	sourceRepo := docsDriftSourceRepo(repoDir, cfg)
	if outputPath == "" {
		outputPath, err = defaultDocsDriftPath(repoDir, sourceRepo)
		if err != nil {
			return "", false, err
		}
	} else if !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(repoDir, outputPath)
	}

	files, err := compareConfiguredDocs(repoDir, cfg)
	if err != nil {
		return "", false, err
	}
	changed := false
	for _, file := range files {
		if file.status != "identical" {
			changed = true
		}
	}

	report := renderDocsDriftReport(sourceRepo, repoDir, files)
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return "", false, fmt.Errorf("create report directory: %w", err)
	}
	if err := os.WriteFile(outputPath, []byte(report), 0o644); err != nil {
		return "", false, fmt.Errorf("write docs drift report: %w", err)
	}
	return outputPath, changed, nil
}

func defaultDocsDriftPath(repoDir, sourceRepo string) (string, error) {
	candidates := []string{repoDir}
	if sourceRepo != "" && sourceRepo != "embedded harnez config" && sourceRepo != repoDir {
		candidates = append([]string{sourceRepo}, candidates...)
	}
	var inbox string
	for _, candidate := range candidates {
		candidateInbox := filepath.Join(candidate, "issues", "inbox")
		if info, err := os.Stat(filepath.Dir(candidateInbox)); err == nil && info.IsDir() {
			inbox = candidateInbox
			break
		}
	}
	if inbox == "" {
		return "", fmt.Errorf("cannot find harnez inbox in %s; use --out to choose a report path", strings.Join(candidates, ", "))
	}
	base := filepath.Join(inbox, "managed-docs-drift-"+time.Now().Format("20060102-150405")+".md")
	path := base
	for n := 2; ; n++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		}
		path = filepath.Join(inbox, fmt.Sprintf("managed-docs-drift-%s-%d.md", time.Now().Format("20060102-150405"), n))
	}
}

func docsDriftSourceRepo(repoDir string, cfg *Config) string {
	if cfg.Dir != "" && cfg.Dir != "." {
		if sourceRepo, err := filepath.Abs(cfg.Dir); err == nil {
			return sourceRepo
		}
	}
	if sourceRepo := findHarnezRepo(repoDir); sourceRepo != "" {
		return sourceRepo
	}
	return "embedded harnez config"
}

func findHarnezRepo(repoDir string) string {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	candidates = append(candidates, filepath.Join(filepath.Dir(repoDir), "harnez"), repoDir)
	seen := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		dir, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		for {
			if !seen[dir] {
				seen[dir] = true
				if isHarnezRepo(dir) {
					return dir
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return ""
}

func isHarnezRepo(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	if !strings.Contains(string(data), "module ubunatic.com/harnez\n") {
		return false
	}
	if info, err := os.Stat(filepath.Join(dir, "config.yaml")); err != nil || info.IsDir() {
		return false
	}
	if info, err := os.Stat(filepath.Join(dir, "issues")); err != nil || !info.IsDir() {
		return false
	}
	return true
}

func compareConfiguredDocs(repoDir string, cfg *Config) ([]docsDriftFile, error) {
	var files []docsDriftFile
	known := make(map[string]bool)
	for _, name := range docNamesInOrder(cfg) {
		lang := cfg.AgentsMD.Languages[name]
		if lang.Source == "" || lang.Local == "" {
			continue
		}
		localPath := localPath(repoDir, lang.Local)
		known[filepath.Clean(localPath)] = true
		sourcePath := filepath.Join(cfg.Dir, filepath.FromSlash(lang.Source))
		if absSource, absErr := filepath.Abs(sourcePath); absErr == nil {
			known[filepath.Clean(absSource)] = true
		}
		source, err := fs.ReadFile(cfg.FS, filepath.ToSlash(lang.Source))
		if err != nil {
			return nil, fmt.Errorf("read configured source %s: %w", sourcePath, err)
		}
		current, readErr := os.ReadFile(localPath)
		if readErr != nil && !os.IsNotExist(readErr) {
			return nil, fmt.Errorf("read project doc %s: %w", localPath, readErr)
		}
		status := "identical"
		if readErr != nil {
			status = "missing"
		} else if !bytes.Equal(source, current) {
			status = "changed"
		}
		diff, err := unifiedDocDiff(sourcePath, localPath, source, current)
		if err != nil {
			return nil, err
		}
		files = append(files, docsDriftFile{name: filepath.ToSlash(filepath.Clean(lang.Local)), path: filepath.Clean(localPath), status: status, diff: diff})
	}
	files, err := compareLocalAgentsSections(repoDir, cfg, files)
	if err != nil {
		return nil, err
	}

	// A marker-tagged local doc that is not in the configured set is useful drift
	// evidence; ordinary handwritten docs are intentionally left alone.
	var extras []string
	err = filepath.WalkDir(filepath.Join(repoDir, "docs"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || known[filepath.Clean(path)] {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(data), "<!-- harnez:bundled -->") {
			extras = append(extras, path)
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("scan project docs: %w", err)
	}
	sort.Strings(extras)
	for _, path := range extras {
		current, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		diff, err := unifiedDocDiff("configured source (missing)", path, nil, current)
		if err != nil {
			return nil, err
		}
		files = append(files, docsDriftFile{name: filepath.ToSlash(filepath.Join("docs", strings.TrimPrefix(path, filepath.Join(repoDir, "docs")+string(filepath.Separator)))), path: filepath.Clean(path), status: "extra", diff: diff})
	}
	return files, nil
}

func compareLocalAgentsSections(repoDir string, cfg *Config, files []docsDriftFile) ([]docsDriftFile, error) {
	local := cfg.AgentsMD.Local
	if local.Target == "" {
		return files, nil
	}
	sections := append([]MDSection(nil), local.Sections...)
	if len(cfg.Docs) > 0 {
		sections = append(sections, MDSection{Name: "Language Conventions", Content: buildLangConventions(docNamesInOrder(cfg), cfg)})
	}
	if len(sections) == 0 {
		return files, nil
	}
	agentsPath := localPath(repoDir, local.Target)
	current, err := os.ReadFile(agentsPath)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read project AGENTS.md %s: %w", agentsPath, err)
	}
	for _, section := range sections {
		expected := strings.TrimRight(markdown.MDMarkers.Begin(section.Name)+"\n"+strings.TrimRight(section.Content, "\n")+"\n"+markdown.MDMarkers.End(section.Name)+"\n", "\n") + "\n"
		actual := ""
		if start, end, found := markdown.SectionBounds(string(current), markdown.MDMarkers.Begin(section.Name), markdown.MDMarkers.End(section.Name)); found {
			actual = string(current[start:end])
		}
		status := "identical"
		if actual == "" {
			status = "missing"
		} else if expected != actual {
			status = "changed"
		}
		diff, err := unifiedDocDiff("configured AGENTS.md#"+section.Name, agentsPath, []byte(expected), []byte(actual))
		if err != nil {
			return nil, err
		}
		files = append(files, docsDriftFile{name: "AGENTS.md#" + section.Name, path: agentsPath, status: status, diff: diff})
	}
	return files, nil
}

func unifiedDocDiff(sourcePath, localPath string, source, current []byte) (string, error) {
	writeTemp := func(data []byte) (string, error) {
		file, err := os.CreateTemp("", "harnez-doc-drift-*")
		if err != nil {
			return "", err
		}
		if _, err := file.Write(data); err != nil {
			file.Close()
			os.Remove(file.Name())
			return "", err
		}
		if err := file.Close(); err != nil {
			os.Remove(file.Name())
			return "", err
		}
		return file.Name(), nil
	}
	oldFile, err := writeTemp(source)
	if err != nil {
		return "", err
	}
	defer os.Remove(oldFile)
	newFile, err := writeTemp(current)
	if err != nil {
		return "", err
	}
	defer os.Remove(newFile)
	cmd := exec.Command("diff", "-u", "--label", sourcePath, "--label", localPath, oldFile, newFile)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return output.String(), nil
		}
		return "", fmt.Errorf("diff %s and %s: %w", sourcePath, localPath, err)
	}
	return "", nil
}

func renderDocsDriftReport(sourceRepo, repoDir string, files []docsDriftFile) string {
	return renderDocsDriftReportOptions(sourceRepo, repoDir, files, true)
}

func renderDocsDriftReportOptions(sourceRepo, repoDir string, files []docsDriftFile, includeMissing bool) string {
	var b strings.Builder
	b.WriteString("---\nsource_repo: ")
	b.WriteString(yamlScalar(sourceRepo))
	b.WriteString("\nfiles:\n")
	for _, file := range files {
		b.WriteString("  - ")
		b.WriteString(yamlScalar(file.name))
		b.WriteByte('\n')
	}
	b.WriteString("---\n\n# Managed Docs Drift\n\n")
	counts := map[string]int{}
	for _, file := range files {
		counts[file.status]++
	}
	if includeMissing {
		fmt.Fprintf(&b, "Compared %d configured docs from `%s` against `%s`. Summary: %d changed, %d missing, %d extra, %d identical.\n\n", len(files), sourceRepo, repoDir, counts["changed"], counts["missing"], counts["extra"], counts["identical"])
	} else {
		fmt.Fprintf(&b, "Compared %d present configured docs from `%s` against `%s`. Summary: %d changed, %d extra, %d identical.\n\n", len(files), sourceRepo, repoDir, counts["changed"], counts["extra"], counts["identical"])
	}
	for _, file := range files {
		fmt.Fprintf(&b, "## `%s` (%s)\n\n", file.name, file.status)
		if file.diff == "" {
			b.WriteString("No differences.\n\n")
			continue
		}
		b.WriteString("```diff\n")
		b.WriteString(file.diff)
		if !strings.HasSuffix(file.diff, "\n") {
			b.WriteByte('\n')
		}
		b.WriteString("```\n\n")
	}
	return b.String()
}

func yamlScalar(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
