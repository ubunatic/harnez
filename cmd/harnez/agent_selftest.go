package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"ubunatic.com/harnez/internal/resolve"
)

type selftestState struct {
	SessionID            string               `json:"session_id"`
	HelloAt              *time.Time           `json:"hello_at,omitempty"`
	BackgroundStartedAt  *time.Time           `json:"background_started_at,omitempty"`
	BackgroundFinishedAt *time.Time           `json:"background_finished_at,omitempty"`
	ConfirmAt            *time.Time           `json:"confirm_at,omitempty"`
	ConfirmRunningAt     *time.Time           `json:"confirm_running_at,omitempty"`
	ConfirmDuration      string               `json:"confirm_duration,omitempty"`
	VerifyAt             *time.Time           `json:"verify_at,omitempty"`
	Steps                []string             `json:"steps"`
	StepTimes            map[string]time.Time `json:"step_times,omitempty"`
}

func sanitizeSelftestID(id string) string {
	if id == "" {
		return "default"
	}
	var sb strings.Builder
	for _, r := range id {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	res := sb.String()
	if len(res) > 64 {
		res = res[:64]
	}
	return res
}

func selftestStateFilePath(stateDir, sessionID string) string {
	if stateDir == "" {
		stateDir = os.TempDir()
	}
	return filepath.Join(stateDir, fmt.Sprintf("harnez-selftest-%s.json", sanitizeSelftestID(sessionID)))
}

func loadSelftestState(path string) (*selftestState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st selftestState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, err
	}
	if st.StepTimes == nil {
		st.StepTimes = make(map[string]time.Time)
	}
	return &st, nil
}

func saveSelftestState(path string, st *selftestState) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(dir, "harnez-selftest-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0600); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

func isOutputTerminal(cmd *cobra.Command) bool {
	if out, ok := cmd.OutOrStdout().(*os.File); ok {
		return term.IsTerminal(int(out.Fd()))
	}
	return false
}

var isOutputTerminalFn = isOutputTerminal

func newAgentSelftestCmd() *cobra.Command {
	var step string
	var sessionOverride string
	var stateDir string
	var backgroundDuration time.Duration

	cmd := &cobra.Command{
		Use:          "selftest",
		Short:        "Run agent background task execution self-test",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			isTTY := isOutputTerminalFn(cmd)

			sessionID := sessionOverride
			if sessionID == "" {
				resolved, err := resolve.Session(resolve.SessionOptions{})
				if err == nil && resolved != "" {
					sessionID = resolved
				} else {
					sessionID = "default"
				}
			}

			stepRaw := strings.TrimSpace(step)
			if stepRaw == "" && len(args) > 0 {
				stepRaw = strings.Join(args, " ")
			}

			if stepRaw == "" {
				if isTTY {
					return cmd.Help()
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Run agent background task execution self-test. Run with --step hello to begin.")
				return nil
			}

			var stepName, stepExtra string
			parts := strings.Fields(stepRaw)
			stepName = parts[0]
			if len(parts) > 1 {
				stepExtra = strings.Join(parts[1:], " ")
			} else if len(args) > 0 && stepName == "confirm-running" {
				stepExtra = strings.Join(args, " ")
			}

			statePath := selftestStateFilePath(stateDir, sessionID)

			switch stepName {
			case "hello":
				now := time.Now()
				st := &selftestState{
					SessionID: sessionID,
					HelloAt:   &now,
					Steps:     []string{"hello"},
					StepTimes: map[string]time.Time{"hello": now},
				}
				if err := saveSelftestState(statePath, st); err != nil {
					return fmt.Errorf("failed to save selftest state: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Agent background self-test initialized (session: %s).\n", sessionID)
				fmt.Fprintln(cmd.OutOrStdout(), "Next actions:")
				fmt.Fprintln(cmd.OutOrStdout(), "1. FIRST: Run 'harnez agent selftest --step background' in your host's native background as a Task/Job.")
				fmt.Fprintln(cmd.OutOrStdout(), "2. SECOND: Run 'harnez agent selftest --step confirm' to confirm the task is running in the background.")
				return nil

			case "background":
				st, err := loadSelftestState(statePath)
				if err != nil || st.HelloAt == nil {
					return errors.New("run 'harnez agent selftest --step hello' first")
				}
				now := time.Now()
				st.BackgroundStartedAt = &now
				st.Steps = append(st.Steps, "background_started")
				st.StepTimes["background_started"] = now
				if err := saveSelftestState(statePath, st); err != nil {
					return fmt.Errorf("failed to save selftest state: %w", err)
				}

				dur := backgroundDuration
				if dur <= 0 {
					dur = 10 * time.Second
				}
				time.Sleep(dur)

				// Re-load in case confirm / confirm-running updated state during sleep
				if reloaded, err := loadSelftestState(statePath); err == nil {
					st = reloaded
				}
				finishNow := time.Now()
				st.BackgroundFinishedAt = &finishNow
				st.Steps = append(st.Steps, "background_finished")
				st.StepTimes["background_finished"] = finishNow
				if err := saveSelftestState(statePath, st); err != nil {
					return fmt.Errorf("failed to save selftest state: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Background task completed after %v.\n", dur)
				fmt.Fprintln(cmd.OutOrStdout(), "Next action: Run 'harnez agent selftest --step verify' to validate the execution sequence.")
				return nil

			case "confirm":
				st, err := loadSelftestState(statePath)
				if err != nil || st.HelloAt == nil {
					return errors.New("run 'harnez agent selftest --step hello' first")
				}
				if st.BackgroundStartedAt == nil {
					return errors.New("background task was not started; run 'harnez agent selftest --step background' in the background first")
				}
				if st.BackgroundFinishedAt != nil {
					return errors.New("background task has already finished; background task must run concurrently in the background")
				}
				now := time.Now()
				st.ConfirmAt = &now
				st.Steps = append(st.Steps, "confirm")
				st.StepTimes["confirm"] = now
				if err := saveSelftestState(statePath, st); err != nil {
					return fmt.Errorf("failed to save selftest state: %w", err)
				}

				fmt.Fprintln(cmd.OutOrStdout(), "Confirmed background task is currently active.")
				fmt.Fprintln(cmd.OutOrStdout(), "Next action: Inspect your native background job/task list, then run:")
				fmt.Fprintln(cmd.OutOrStdout(), "  harnez agent selftest --step confirm-running [<duration>]")
				return nil

			case "confirm-running":
				st, err := loadSelftestState(statePath)
				if err != nil || st.ConfirmAt == nil {
					return errors.New("run 'harnez agent selftest --step confirm' first")
				}
				now := time.Now()
				st.ConfirmRunningAt = &now
				st.ConfirmDuration = stepExtra
				st.Steps = append(st.Steps, "confirm-running")
				st.StepTimes["confirm-running"] = now
				if err := saveSelftestState(statePath, st); err != nil {
					return fmt.Errorf("failed to save selftest state: %w", err)
				}

				if stepExtra != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "Recorded background running confirmation (duration: %s).\n", stepExtra)
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "Recorded background running confirmation.")
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Next action: Await background task completion, then run 'harnez agent selftest --step verify'.")
				return nil

			case "verify":
				st, err := loadSelftestState(statePath)
				if err != nil {
					return fmt.Errorf("no selftest state found for session %s (run --step hello first)", sessionID)
				}

				var issues []string
				if st.HelloAt == nil {
					issues = append(issues, "--step hello was never executed")
				}
				if st.BackgroundStartedAt == nil {
					issues = append(issues, "--step background was never started")
				}
				if st.ConfirmAt == nil {
					issues = append(issues, "--step confirm was never executed")
				}
				if st.ConfirmRunningAt == nil {
					issues = append(issues, "--step confirm-running was never executed")
				}
				if st.BackgroundFinishedAt == nil {
					issues = append(issues, "--step background has not completed yet")
				}
				if st.BackgroundStartedAt != nil && st.ConfirmAt != nil {
					if st.ConfirmAt.Before(*st.BackgroundStartedAt) {
						issues = append(issues, "--step confirm ran before --step background was started")
					}
				}
				if st.ConfirmAt != nil && st.BackgroundFinishedAt != nil {
					if st.BackgroundFinishedAt.Before(*st.ConfirmAt) {
						issues = append(issues, "--step confirm ran after --step background had already completed")
					}
				}
				if st.ConfirmAt != nil && st.ConfirmRunningAt != nil {
					if st.ConfirmRunningAt.Before(*st.ConfirmAt) {
						issues = append(issues, "--step confirm-running ran before --step confirm")
					}
				}

				if len(issues) > 0 {
					return fmt.Errorf("selftest validation failed:\n- %s", strings.Join(issues, "\n- "))
				}

				now := time.Now()
				st.VerifyAt = &now
				st.Steps = append(st.Steps, "verify")
				st.StepTimes["verify"] = now
				_ = saveSelftestState(statePath, st)

				fmt.Fprintln(cmd.OutOrStdout(), "PASS: Background execution self-test passed successfully. All steps executed in the correct sequence.")
				return nil

			default:
				if isTTY {
					return fmt.Errorf("unknown step %q. Valid steps: hello, background, confirm, confirm-running, verify", stepName)
				}
				return fmt.Errorf("unknown step %q. Run 'harnez agent selftest --step hello' to begin", stepName)
			}
		},
	}

	cmd.SetHelpFunc(func(c *cobra.Command, args []string) {
		if isOutputTerminalFn(c) {
			fmt.Fprintln(c.OutOrStdout(), "Usage:")
			fmt.Fprintln(c.OutOrStdout(), "  harnez agent selftest [flags]")
			fmt.Fprintln(c.OutOrStdout(), "")
			fmt.Fprintln(c.OutOrStdout(), "Steps:")
			fmt.Fprintln(c.OutOrStdout(), "  hello                   Initialize self-test sequence")
			fmt.Fprintln(c.OutOrStdout(), "  background              Run background task (sleeps and notifies on completion)")
			fmt.Fprintln(c.OutOrStdout(), "  confirm                 Confirm background task is actively running")
			fmt.Fprintln(c.OutOrStdout(), "  confirm-running [<dur>] Record running task duration inspection")
			fmt.Fprintln(c.OutOrStdout(), "  verify                  Validate the entire execution order and state")
			fmt.Fprintln(c.OutOrStdout(), "")
			fmt.Fprintln(c.OutOrStdout(), "Flags:")
			fmt.Fprintln(c.OutOrStdout(), "      --step string       Step to execute")
			fmt.Fprintln(c.OutOrStdout(), "      --session string    Session ID override")
			fmt.Fprintln(c.OutOrStdout(), "      --duration duration Duration for background task (default 10s)")
			fmt.Fprintln(c.OutOrStdout(), "      --state-dir string  Directory for state tracking (default /tmp)")
		} else {
			fmt.Fprintln(c.OutOrStdout(), "Run agent background task execution self-test.")
			fmt.Fprintln(c.OutOrStdout(), "Run with --step hello to begin.")
		}
	})

	cmd.Flags().StringVar(&step, "step", "", "step to execute (run with --step hello to begin)")
	cmd.Flags().StringVar(&sessionOverride, "session", "", "session ID override")
	cmd.Flags().StringVar(&stateDir, "state-dir", "", "state directory for step tracking")
	cmd.Flags().DurationVar(&backgroundDuration, "duration", 10*time.Second, "background task duration")

	return cmd
}
