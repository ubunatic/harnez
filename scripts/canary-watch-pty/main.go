// Command canary-watch-pty is the Go replacement for the former
// canary-watch-pty.sh (used in issue 114 to confirm the streaming/batch
// flap). It drives `harnez usage --watch` under a real pseudo-terminal
// (via util-linux's `script`) so a non-interactive shell can capture the
// full redraw output for inspection.
//
// --watch reads /dev/tty directly for keypresses and runs stty against it,
// so a plain pipe or redirect leaves it unable to detect a terminal at all.
// `script` allocates a real pty, the same mechanism an interactive terminal
// emulator provides.
//
// Output:
//   - Panel titles seen (deduplicated)
//   - Remote Load box streaming vs batch label counts (flap detection)
//   - Full capture written to /tmp/harnez-watch-pty-<rand>.log
//
// Usage:
//
//	go run ./scripts/canary-watch-pty [SECONDS] [-- EXTRA_HARNEZ_ARGS...]
//
// Defaults to 20 seconds.
//
// Example:
//
//	go run ./scripts/canary-watch-pty 30 -- --proc
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	seconds := 20
	var extraArgs []string

	args := os.Args[1:]
	// Split on "--"
	for i, a := range args {
		if a == "--" {
			extraArgs = args[i+1:]
			args = args[:i]
			break
		}
	}
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: SECONDS must be an integer, got %q\n", args[0])
			os.Exit(1)
		}
		seconds = n
	}

	if err := run(seconds, extraArgs); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
}

func run(seconds int, extraArgs []string) error {
	// Check that `script` is available — it is part of util-linux.
	if _, err := exec.LookPath("script"); err != nil {
		return fmt.Errorf("`script` (util-linux) not found in PATH — required to allocate a real PTY: %w", err)
	}
	if _, err := exec.LookPath("harnez"); err != nil {
		return fmt.Errorf("`harnez` not found in PATH — run `make install` first: %w", err)
	}

	logFile, err := os.CreateTemp("", "harnez-watch-pty-*.log")
	if err != nil {
		return fmt.Errorf("create temp log: %w", err)
	}
	logPath := logFile.Name()
	logFile.Close()

	harnezCmd := strings.Join(append([]string{"harnez", "usage", "--watch"}, extraArgs...), " ")
	// COLUMNS/LINES override: give --watch enough width to render the [R] box label.
	scriptEnvCmd := fmt.Sprintf("COLUMNS=200 LINES=60 %s", harnezCmd)

	fmt.Printf("==> Recording %ds of '%s' under a pty\n", seconds, harnezCmd)

	cmd := exec.Command("script",
		"-q",               // quiet: no "Script started/done" banner
		"-e",               // propagate exit code of the wrapped command
		"-c", scriptEnvCmd, // command to run inside the pty
		logPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// timeout wraps the script run so it terminates after `seconds`.
	runErr := runWithTimeout(cmd, time.Duration(seconds)*time.Second)

	// Read back the capture — it contains ANSI escape codes from the TUI
	// redraw, which we strip for analysis.
	raw, readErr := os.ReadFile(logPath)
	if readErr != nil {
		return fmt.Errorf("read capture log %s: %w", logPath, readErr)
	}

	clean := stripANSI(string(raw))

	fmt.Println()
	fmt.Println("==> Panel titles seen (deduplicated, first-seen order):")
	panelRe := regexp.MustCompile(`\[[A-Za-z]\][^\x00-\x08\x0a-\x1f]*`)
	seen := make(map[string]bool)
	var panels []string
	for _, m := range panelRe.FindAllString(clean, -1) {
		t := strings.TrimSpace(m)
		if !seen[t] {
			seen[t] = true
			panels = append(panels, t)
		}
	}
	for _, p := range panels {
		fmt.Printf("    %s\n", p)
	}

	fmt.Println()
	fmt.Println("==> Remote Load box label occurrences (streaming vs batch, for flap detection):")
	// Matches patterns like: (@um760 · streaming)  or  (@um760 · batch)
	labelRe := regexp.MustCompile(`\(@[^ )]+\s*[·•]\s*(streaming|batch)\)`)
	counts := map[string]int{}
	for _, m := range labelRe.FindAllStringSubmatch(clean, -1) {
		counts[m[1]]++
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		fmt.Println("    (none found — [R] Remote Load box may not be present or load.watch_host not configured)")
	} else {
		for _, k := range keys {
			fmt.Printf("    %6d  %s\n", counts[k], k)
		}
		streaming := counts["streaming"]
		batch := counts["batch"]
		total := streaming + batch
		if total > 0 {
			fmt.Println()
			if streaming == 0 {
				fmt.Println("    RESULT: streaming never established — ControlMaster or load-stream not working")
			} else if float64(batch)/float64(total) > 0.4 {
				fmt.Printf("    RESULT: FLAPPING — %.0f%% of redraws were in batch mode (issue 114 likely present)\n",
					float64(batch)/float64(total)*100)
			} else {
				fmt.Printf("    RESULT: stable — %.0f%% of redraws were in streaming mode\n",
					float64(streaming)/float64(total)*100)
			}
		}
	}

	fmt.Println()
	fmt.Printf("==> Canary completed. Full capture kept at: %s\n", logPath)

	if runErr != nil {
		// timeout exit is expected and not a failure of the canary itself.
		if strings.Contains(runErr.Error(), "killed") || isExitCode(runErr, 124) {
			return nil
		}
		// harnez --watch exiting with non-zero inside script is also common
		// (Ctrl-C / SIGTERM from our timeout), so we don't propagate it.
		_ = runErr
	}
	return nil
}

// runWithTimeout runs cmd and kills it (along with its process group) after d.
func runWithTimeout(cmd *exec.Cmd, d time.Duration) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		cmd.Process.Kill() //nolint:errcheck
		<-done
		return nil
	}
}

// stripANSI removes ANSI/VT100 escape sequences from s.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\x1b[()][AB012]|\x1b[=>]|\r`)

func stripANSI(s string) string {
	clean := ansiRe.ReplaceAllString(s, "")
	// Collapse runs of NUL/control chars left by the pty recording.
	var buf bytes.Buffer
	sc := bufio.NewScanner(strings.NewReader(clean))
	for sc.Scan() {
		line := strings.Map(func(r rune) rune {
			if r < 0x20 && r != '\t' && r != '\n' {
				return -1
			}
			return r
		}, sc.Text())
		buf.WriteString(line)
		buf.WriteByte('\n')
	}
	return buf.String()
}

func isExitCode(err error, code int) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode() == code
	}
	return false
}
