package quota1

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// RunRecord is the persisted identity and outcome of the latest quota-1 run.
// A nil Exit means the run did not complete with a normal process exit.
type RunRecord struct {
	Started      time.Time  `json:"started"`
	PGID         int        `json:"pgid"`
	PIDStarttime uint64     `json:"pid_starttime"`
	Finished     *time.Time `json:"finished"`
	Exit         *int       `json:"exit"`
}

// CheckOptions configures the Quota-1 check and state recording.
type CheckOptions struct {
	// Dir is the working directory or repository root (defaults to ".").
	Dir string

	// StateFile explicitly overrides the state file path.
	StateFile string

	// Getenv retrieves environment variables (nil defaults to os.Getenv).
	Getenv func(string) string

	// Now provides the current time (nil defaults to time.Now).
	Now func() time.Time
}

// Result describes the outcome of a Quota-1 check.
type Result struct {
	Allowed      bool
	Message      string
	StateFile    string
	ModifiedFile string
}

// ResolveStateFile returns the path to the Quota-1 state file and root directory.
// If .git is found as a repository directory in dir or any ancestor, it returns .git/harnez/quota_1.state.
// Otherwise, it returns .harnez/quota_1.state in dir.
func ResolveStateFile(dir string) (stateFile string, root string, err error) {
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("resolve dir: %w", err)
	}

	cur := absDir
	for {
		gitPath := filepath.Join(cur, ".git")
		if fi, err := os.Stat(gitPath); err == nil {
			if fi.IsDir() {
				if _, err := os.Stat(filepath.Join(gitPath, "HEAD")); err == nil {
					return filepath.Join(gitPath, "harnez", "quota_1.state"), cur, nil
				}
				// Ignore directories that only happen to be named .git.
			} else {
				// In git worktrees or submodules, .git is a file. Use .harnez in the worktree root.
				return filepath.Join(cur, ".harnez", "quota_1.state"), cur, nil
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}

	return filepath.Join(absDir, ".harnez", "quota_1.state"), absDir, nil
}

// isBypass returns true if QUOTA_BYPASS=1 or HARNEZ_QUOTA_BYPASS=1 (or "true").
func isBypass(getenv func(string) string) bool {
	if getenv == nil {
		getenv = os.Getenv
	}
	for _, key := range []string{"QUOTA_BYPASS", "HARNEZ_QUOTA_BYPASS"} {
		v := strings.TrimSpace(getenv(key))
		if v == "1" || strings.EqualFold(v, "true") {
			return true
		}
	}
	return false
}

// ReadRunRecord reads either a current JSON run record or a legacy timestamp.
// Legacy timestamps represent completed runs and continue to consume quota.
func ReadRunRecord(path string) (RunRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RunRecord{}, err
	}
	str := strings.TrimSpace(string(data))
	if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
		exit := 0
		return RunRecord{Started: t, Finished: &t, Exit: &exit}, nil
	}
	var record RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return RunRecord{}, err
	}
	if record.Started.IsZero() {
		return RunRecord{}, fmt.Errorf("quota-1 state has no started time")
	}
	return record, nil
}

func readState(path string) (time.Time, error) {
	record, err := ReadRunRecord(path)
	if err != nil {
		return time.Time{}, err
	}
	return record.Started, nil
}

// ChangesSinceLastRun returns eligible source files modified after the last
// Quota-1 run, along with that run's recorded time. Missing state is not an
// error and returns no changes.
func ChangesSinceLastRun(dir string) ([]string, time.Time, error) {
	return ChangesSinceLastRunAfter(dir, time.Time{})
}

// ChangesSinceLastRunAfter returns code files changed since the last quota run
// and modified after turnStarted. It excludes ignored, markdown and issue files.
func ChangesSinceLastRunAfter(dir string, turnStarted time.Time) ([]string, time.Time, error) {
	stateFile, root, err := ResolveStateFile(dir)
	if err != nil {
		return nil, time.Time{}, err
	}
	since, err := readState(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, time.Time{}, nil
		}
		return nil, time.Time{}, fmt.Errorf("read quota-1 state: %w", err)
	}
	files, err := findModifiedSourceFiles(root, stateFile, since)
	if err != nil {
		return nil, time.Time{}, err
	}
	filtered := files[:0]
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || (!turnStarted.IsZero() && !info.ModTime().After(turnStarted)) {
			continue
		}
		rel, err := filepath.Rel(root, path)
		if err != nil || strings.EqualFold(filepath.Ext(path), ".md") || strings.HasPrefix(filepath.ToSlash(rel), "issues/") {
			continue
		}
		tracked := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", "--", rel)
		if err := tracked.Run(); err == nil {
			filtered = append(filtered, path)
			continue
		}
		unignored := exec.Command("git", "-C", root, "check-ignore", "-q", "--", rel)
		if err := unignored.Run(); err != nil {
			filtered = append(filtered, path)
		}
	}
	return filtered, since, nil
}

// writeState writes the timestamp to the state file.
func writeState(path string, t time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(t.UTC().Format(time.RFC3339Nano)+"\n"), 0644)
}

func writeRunRecord(path string, record RunRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// UpdateProcess records the process group and Linux /proc start time for a run.
// A zero start time means the platform does not expose a readable /proc stat.
func UpdateProcess(path string, pgid, pid int) error {
	record, err := ReadRunRecord(path)
	if err != nil {
		return err
	}
	record.PGID = pgid
	record.PIDStarttime = ProcStarttime(pid)
	return writeRunRecord(path, record)
}

// FinishRun records completion. A nil exit indicates a signal or other
// abnormal termination and leaves the quota available for a retry.
func FinishRun(path string, finished time.Time, exit *int) error {
	record, err := ReadRunRecord(path)
	if err != nil {
		return err
	}
	finished = finished.UTC()
	record.Finished = &finished
	if exit == nil {
		record.Exit = nil
	} else {
		value := *exit
		record.Exit = &value
	}
	return writeRunRecord(path, record)
}

// ProcStarttime reads field 22 from /proc/<pid>/stat. It parses after the last
// ')' because the process comm field may itself contain spaces or parentheses.
func ProcStarttime(pid int) uint64 {
	file, err := os.Open(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0
	}
	return parseProcStarttime(scanner.Text())
}

func parseProcStarttime(line string) uint64 {
	end := strings.LastIndex(line, ")")
	if end < 0 || end+1 >= len(line) {
		return 0
	}
	fields := strings.Fields(line[end+1:])
	if len(fields) <= 19 {
		return 0
	}
	starttime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0
	}
	return starttime
}

// isExcludedDir returns true for directories that should not be scanned for code changes.
func isExcludedDir(name string) bool {
	switch name {
	case ".git", ".harnez", "vendor", "node_modules", ".claude", ".gemini", ".idea", ".vscode",
		".cache", "tmp", "temp", "dist", "build", "target", "bin", "__pycache__", ".pytest_cache":
		return true
	}
	return strings.HasPrefix(name, ".")
}

// isExcludedFile returns true for files that do not count as source code modifications.
func isExcludedFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	for _, ext := range []string{
		"~", ".swp", ".swo", ".tmp", ".temp", ".bak", ".log", ".lock", ".test",
	} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// findModifiedSourceFile checks if any repository source file in root has ModTime > since.
func findModifiedSourceFile(root, stateFilePath string, since time.Time) (string, error) {
	files, err := findModifiedSourceFiles(root, stateFilePath, since)
	if err != nil || len(files) == 0 {
		return "", err
	}
	return files[0], nil
}

func findModifiedSourceFiles(root, stateFilePath string, since time.Time) ([]string, error) {
	absStateFile, _ := filepath.Abs(stateFilePath)
	var found []string

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if isExcludedDir(name) {
				return filepath.SkipDir
			}
			return nil
		}

		absPath, _ := filepath.Abs(path)
		if absPath == absStateFile {
			return nil
		}
		if isExcludedFile(name) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(since) {
			found = append(found, path)
		}
		return nil
	})

	if err != nil && !errors.Is(err, fs.SkipAll) {
		return nil, err
	}
	return found, nil
}

// CheckAndRecord evaluates whether a test run is allowed under Quota-1 rules.
// If allowed, it records the run's start in the state file.
// If blocked, it returns Allowed=false with a descriptive error message without updating the state file.
func CheckAndRecord(opts CheckOptions) (Result, error) {
	now := time.Now()
	if opts.Now != nil {
		now = opts.Now()
	}

	stateFile := opts.StateFile
	root := opts.Dir
	if stateFile == "" {
		sf, r, err := ResolveStateFile(opts.Dir)
		if err != nil {
			return Result{}, fmt.Errorf("resolve state file: %w", err)
		}
		stateFile = sf
		root = r
	} else if root == "" {
		root = "."
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Result{}, fmt.Errorf("resolve root dir: %w", err)
	}

	// 1. Check bypass
	if isBypass(opts.Getenv) {
		if err := writeRunRecord(stateFile, RunRecord{Started: now.UTC()}); err != nil {
			return Result{}, fmt.Errorf("write quota-1 state: %w", err)
		}
		return Result{
			Allowed:   true,
			Message:   "Quota-1: bypass active via environment variable; test execution allowed",
			StateFile: stateFile,
		}, nil
	}

	// 2. Check state file existence and allow an incomplete run one retry.
	previous, err := ReadRunRecord(stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			if err := writeRunRecord(stateFile, RunRecord{Started: now.UTC()}); err != nil {
				return Result{}, fmt.Errorf("write quota-1 state: %w", err)
			}
			return Result{
				Allowed:   true,
				Message:   "Quota-1: initial test execution allowed; quota recorded",
				StateFile: stateFile,
			}, nil
		}
		// If unreadable or corrupted, overwrite and allow run
		if err := writeRunRecord(stateFile, RunRecord{Started: now.UTC()}); err != nil {
			return Result{}, fmt.Errorf("rewrite corrupt quota-1 state: %w", err)
		}
		return Result{
			Allowed:   true,
			Message:   "Quota-1: reset corrupted state; test execution allowed",
			StateFile: stateFile,
		}, nil
	}

	if previous.Exit == nil {
		if err := writeRunRecord(stateFile, RunRecord{Started: now.UTC()}); err != nil {
			return Result{}, fmt.Errorf("update incomplete quota-1 state: %w", err)
		}
		return Result{Allowed: true, Message: "Quota-1: previous run was incomplete; one retry allowed", StateFile: stateFile}, nil
	}

	// 3. State file exists: check if any source files modified since last run.
	modifiedFile, err := findModifiedSourceFile(absRoot, stateFile, previous.Started)
	if err != nil {
		return Result{}, fmt.Errorf("check modified source files: %w", err)
	}

	if modifiedFile != "" {
		if err := writeRunRecord(stateFile, RunRecord{Started: now.UTC()}); err != nil {
			return Result{}, fmt.Errorf("update quota-1 state: %w", err)
		}
		return Result{
			Allowed:      true,
			Message:      fmt.Sprintf("Quota-1: source modification detected in %s; test execution allowed", modifiedFile),
			StateFile:    stateFile,
			ModifiedFile: modifiedFile,
		}, nil
	}

	// 4. No files modified: block!
	return Result{
		Allowed:   false,
		Message:   fmt.Sprintf("Quota-1: test execution blocked because no repository source files have been modified since the last test run (%s).\nUnder Quota-1 rules, code must be modified before running tests again.\n(Bypass available via QUOTA_BYPASS=1 or HARNEZ_QUOTA_BYPASS=1 for emergency/manual overrides)", previous.Started.Format(time.RFC3339)),
		StateFile: stateFile,
	}, nil
}
