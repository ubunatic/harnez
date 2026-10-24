package usage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type killFailureRecord struct {
	Time      time.Time         `json:"time"`
	Provider  string            `json:"provider"`
	Command   string            `json:"command"`
	PID       *int              `json:"pid"`
	ElapsedMS int64             `json:"elapsed_ms"`
	Signal    string            `json:"signal"`
	Cause     string            `json:"cause"`
	Findings  map[string]string `json:"findings"`
}

func isSignalFailure(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "signal:")
}

func signalName(err error) string {
	if err == nil {
		return "unknown"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "signal: killed"):
		return "SIGKILL"
	case strings.Contains(message, "signal: terminated"):
		return "SIGTERM"
	default:
		return "unknown"
	}
}

func collectKnownKillFindings(ctx context.Context, _ Collector, _ error, ctxErr error) map[string]string {
	findings := map[string]string{
		"pid":    "unknown",
		"sender": "unknown",
		"oom":    "unknown",
		"stderr": "unknown",
	}
	switch {
	case errors.Is(ctxErr, context.DeadlineExceeded):
		findings["context"] = "deadline exceeded"
	case errors.Is(ctxErr, context.Canceled):
		findings["context"] = "canceled"
	case ctx.Err() != nil:
		findings["context"] = "check budget exceeded"
	default:
		findings["context"] = "not canceled"
	}
	return findings
}

func (r *Registry) recordKillFailure(c Collector, elapsed time.Duration, err, ctxErr error) {
	if r == nil {
		return
	}
	settings, budget, settingsErr := usageKillLogSettings()
	if settingsErr != nil {
		fmt.Fprintf(os.Stderr, "harnez server: cannot configure kill log: %v\n", settingsErr)
		return
	}
	r.killWG.Add(1)
	go func() {
		defer r.killWG.Done()
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		defer cancel()
		findings := r.killCheck(ctx, c, err, ctxErr)
		cause := fetchFailureCause(c, elapsed, err, ctxErr)
		command := c.Command
		if command == "" {
			command = c.ID
		}
		record := killFailureRecord{
			Time:      time.Now().UTC(),
			Provider:  c.Provider,
			Command:   command,
			ElapsedMS: elapsed.Milliseconds(),
			Signal:    signalName(err),
			Cause:     cause,
			Findings:  findings,
		}
		stateHome := os.Getenv("XDG_STATE_HOME")
		if !filepath.IsAbs(stateHome) {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return
			}
			stateHome = filepath.Join(home, ".local", "state")
		}
		logPath := filepath.Join(stateHome, filepath.FromSlash(settings.Path))
		r.killLogMu.Lock()
		writeErr := appendRotatingKillRecord(logPath, settings, record)
		r.killLogMu.Unlock()
		if writeErr != nil {
			fmt.Fprintf(os.Stderr, "harnez server: cannot write kill log: %v\n", writeErr)
		}
	}()
}

func appendRotatingKillRecord(path string, settings usageKillLogSpec, record killFailureRecord) error {
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode kill record: %w", err)
	}
	line = append(line, '\n')
	if int64(len(line)) > settings.MaxBytes {
		return fmt.Errorf("kill record exceeds configured maximum size")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create kill log directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat kill log: %w", err)
	}
	if info != nil && info.Size()+int64(len(line)) > settings.MaxBytes {
		if err := rotateKillLog(path, settings.Backups); err != nil {
			return err
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open kill log: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(line); err != nil {
		return fmt.Errorf("append kill log: %w", err)
	}
	return nil
}

func rotateKillLog(path string, backups int) error {
	if backups == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove full kill log: %w", err)
		}
		return nil
	}
	oldest := fmt.Sprintf("%s.%d", path, backups)
	if err := os.Remove(oldest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove oldest kill log: %w", err)
	}
	for i := backups - 1; i >= 1; i-- {
		from := fmt.Sprintf("%s.%d", path, i)
		to := fmt.Sprintf("%s.%d", path, i+1)
		if err := os.Rename(from, to); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("rotate kill log %s: %w", from, err)
		}
	}
	if err := os.Rename(path, path+".1"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("rotate kill log: %w", err)
	}
	return nil
}

func (r *Registry) waitKillDiagnostics() { r.killWG.Wait() }
