// Command canary-remote-load-stream is the Go replacement for the former
// canary-remote-load-stream.sh (issue 110) and the primary regression canary
// for issue 114 (stream drops to batch via stdin-EOF false-trigger).
//
// It validates two independent mechanisms in sequence:
//
//  1. SSH ControlMaster multiplexing works as a shared transport: a background
//     master connection allows concurrent "stream" and "batch" children to
//     share one TCP connection without interfering.
//
//  2. The stdin-EOF false-trigger bug (issue 114):
//     When exec.Command sets Cmd.Stdin = nil (the default), the child's stdin
//     is backed by /dev/null, which delivers EOF immediately. The remote
//     harnez load-stream process treats stdin-EOF as "parent tore down the
//     ControlMaster", so it exits within milliseconds — causing the UI to
//     immediately drop from "streaming" to "batch". This section:
//       (a) reproduces the bug: load-stream exits within 2s when stdin is nil
//       (b) validates the fix: load-stream stays alive when stdin is a live
//           io.Pipe() whose write end is held open for the stream's lifetime
//
// Usage:
//
//	go run ./scripts/canary-remote-load-stream um760
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: canary-remote-load-stream HOST")
		os.Exit(1)
	}
	host := os.Args[1]

	if err := run(host); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println("==> Canary completed successfully.")
}

func run(host string) error {
	if err := runControlMasterSection(host); err != nil {
		return fmt.Errorf("ControlMaster section: %w", err)
	}
	if err := runStdinEOFSection(host); err != nil {
		return fmt.Errorf("stdin-EOF section: %w", err)
	}
	return nil
}

// ── Section 1: ControlMaster multiplexing ────────────────────────────────────

func runControlMasterSection(host string) error {
	fmt.Printf("==> [1/2] ControlMaster multiplexing (%s)\n", host)

	dir, err := os.MkdirTemp("", "harnez-canary-cm-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	ctl := dir + "/cm"

	// Start master
	fmt.Println("    Starting background ControlMaster connection")
	if out, err := exec.Command("ssh",
		"-M", "-S", ctl,
		"-o", "ControlPersist=60",
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
		"-fN", host,
	).CombinedOutput(); err != nil {
		return fmt.Errorf("ssh -M: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	defer exec.Command("ssh", "-S", ctl, "-o", "BatchMode=yes", "-O", "exit", host).Run() //nolint:errcheck

	// Confirm alive
	fmt.Println("    Confirming control socket is live")
	if out, err := exec.Command("ssh", "-S", ctl, "-O", "check", host).CombinedOutput(); err != nil {
		return fmt.Errorf("ssh -O check: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	// Cold vs warm timing
	cold, err := timeCmd(exec.Command("ssh", "-o", "ControlMaster=no", "-o", "ConnectTimeout=5", host, "echo cold"))
	if err != nil {
		return fmt.Errorf("cold call: %w", err)
	}
	warm, err := timeCmd(exec.Command("ssh", "-S", ctl, "-o", "BatchMode=yes", host, "echo warm"))
	if err != nil {
		return fmt.Errorf("warm call: %w", err)
	}
	fmt.Printf("    Cold call (fresh handshake):    %v\n", cold.Round(time.Millisecond))
	fmt.Printf("    Warm call (multiplexed, no TLS): %v\n", warm.Round(time.Millisecond))
	if warm >= cold {
		// Not a hard failure — can flake on a loaded system — warn only.
		fmt.Println("    WARNING: warm call was not faster than cold (flaky timing?)")
	}

	// Long-lived stream child + concurrent batch call
	fmt.Println("    Starting long-lived stream child over master")
	streamCmd := exec.Command("ssh", "-S", ctl, "-o", "BatchMode=yes", host,
		"for i in 1 2 3; do echo sample-$i; sleep 0.2; done")
	streamCmd.Stdout = os.Stdout
	if err := streamCmd.Start(); err != nil {
		return fmt.Errorf("stream child start: %w", err)
	}
	fmt.Println("    Firing concurrent batch call over same master")
	batchOut, err := exec.Command("ssh", "-S", ctl, "-o", "BatchMode=yes", host, "echo concurrent-batch-call").CombinedOutput()
	if err != nil {
		streamCmd.Process.Kill() //nolint:errcheck
		return fmt.Errorf("batch call: %w", err)
	}
	fmt.Printf("    Batch response: %s", batchOut)
	if err := streamCmd.Wait(); err != nil {
		return fmt.Errorf("stream child wait: %w", err)
	}

	// Teardown + fallback
	fmt.Println("    Tearing down master")
	exec.Command("ssh", "-S", ctl, "-o", "BatchMode=yes", "-O", "exit", host).Run() //nolint:errcheck
	fmt.Println("    Confirming plain fallback still works after teardown")
	if out, err := exec.Command("ssh", "-o", "ConnectTimeout=5", host, "echo fallback").CombinedOutput(); err != nil {
		return fmt.Errorf("fallback call: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	fmt.Println("    OK")
	return nil
}

// ── Section 2: stdin-EOF false-trigger (issue 114) ───────────────────────────

// remoteLoadStreamCmd mirrors the constant in loadstream.go exactly so the
// canary exercises the same PATH search the production code relies on.
const remoteLoadStreamCmd = `PATH="$PATH:$HOME/go/bin:$HOME/bin:/usr/local/bin" harnez load-stream`

// streamSurvives starts harnez load-stream on host over a fresh ControlMaster
// and holds stdin open via the supplied stdinPipe (nil = no stdin / /dev/null).
// It waits probeWindow and returns whether the child was still running at that
// point, plus any NDJSON lines it received while alive.
// streamSurvives starts harnez load-stream on host over a fresh ControlMaster.
// stdinKeepOpen controls stdin wiring:
//   - false → cmd.Stdin is left nil (/dev/null) — reproduces the issue 114 bug.
//   - true  → cmd.Stdin is a live io.Pipe() whose write end is held open for
//     the probe window — validates the fix direction.
//
// It waits probeWindow and returns whether the child was still running at
// that point, plus any NDJSON lines received while it was alive.
func streamSurvives(host string, stdinKeepOpen bool, probeWindow time.Duration) (alive bool, lines []string, err error) {
	dir, err := os.MkdirTemp("", "harnez-canary-cm-*")
	if err != nil {
		return false, nil, err
	}
	defer os.RemoveAll(dir)
	ctl := dir + "/cm"

	if out, cmErr := exec.Command("ssh",
		"-M", "-S", ctl,
		"-o", "ControlPersist=60",
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
		"-fN", host,
	).CombinedOutput(); cmErr != nil {
		return false, nil, fmt.Errorf("start ControlMaster: %w\n%s", cmErr, strings.TrimSpace(string(out)))
	}
	defer exec.Command("ssh", "-S", ctl, "-o", "BatchMode=yes", "-O", "exit", host).Run() //nolint:errcheck

	cmd := exec.Command("ssh", "-S", ctl, "-o", "BatchMode=yes", host, remoteLoadStreamCmd)

	// Stdin wiring — this is the mechanism under test.
	var stdinPW *io.PipeWriter
	if stdinKeepOpen {
		var pr *io.PipeReader
		pr, stdinPW = io.Pipe()
		cmd.Stdin = pr
	}
	// else: cmd.Stdin == nil → /dev/null (the bug path).

	var stderrBuf strings.Builder
	cmd.Stderr = &stderrBuf

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if startErr := cmd.Start(); startErr != nil {
		stdout.Close()
		if stdinPW != nil {
			stdinPW.Close()
		}
		return false, nil, fmt.Errorf("start ssh child: %w", startErr)
	}

	// Collect lines; goroutine exits when stdout closes (process done).
	lineCh := make(chan string, 64)
	go func() {
		defer close(lineCh)
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			if line := strings.TrimSpace(sc.Text()); line != "" {
				lineCh <- line
			}
		}
	}()

	// Drain lines until probeWindow elapses or process dies.
	deadline := time.After(probeWindow)
	for {
		select {
		case line, ok := <-lineCh:
			if ok {
				lines = append(lines, line)
			} else {
				// lineCh closed → process already died.
				cmd.Wait() //nolint:errcheck
				if len(lines) == 0 && stderrBuf.Len() > 0 {
					return false, lines, fmt.Errorf("load-stream produced no output; stderr: %s", strings.TrimSpace(stderrBuf.String()))
				}
				return false, lines, nil
			}
		case <-deadline:
			// probeWindow elapsed — process is still running. Tear down cleanly:
			// kill first so exec's internal stdin-copy goroutine unblocks (it
			// blocks on pr.Read otherwise), then close the write end, then Wait.
			cmd.Process.Kill() //nolint:errcheck
			if stdinPW != nil {
				stdinPW.CloseWithError(io.ErrClosedPipe)
			}
			cmd.Wait() //nolint:errcheck
			for l := range lineCh {
				lines = append(lines, l)
			}
			if len(lines) == 0 && stderrBuf.Len() > 0 {
				return true, lines, fmt.Errorf("load-stream produced no output; stderr: %s", strings.TrimSpace(stderrBuf.String()))
			}
			return true, lines, nil
		}
	}
}

func runStdinEOFSection(host string) error {
	fmt.Printf("==> [2/2] stdin-EOF false-trigger regression (issue 114, host=%s)\n", host)

	// probeWindow must be long enough that at least one NDJSON sample can
	// arrive: load-stream emits one line per second, but SSH connection setup
	// + process start adds ~0.2s, and the first sample isn't flushed until
	// the first full tick fires after startup (~1–2s). 4s gives comfortable
	// headroom for at least two samples before we declare "alive".
	probeWindow := 4 * time.Second

	// (a) Reproduce the bug: Cmd.Stdin == nil → /dev/null → remote EOF → early exit.
	fmt.Printf("    (a) BUG reproduction: load-stream with nil stdin (Cmd.Stdin = nil)\n")
	fmt.Printf("        Waiting %v to see if the remote process exits prematurely...\n", probeWindow)
	alive, lines, err := streamSurvives(host, false, probeWindow)
	if err != nil {
		return fmt.Errorf("bug-reproduction probe: %w", err)
	}
	fmt.Printf("        Lines received: %d\n", len(lines))
	if alive {
		fmt.Printf("        UNEXPECTED: process was still alive after %v — bug not reproduced.\n", probeWindow)
		fmt.Printf("        (SSH on this system may propagate stdin differently than exec.Command does)\n")
	} else {
		fmt.Printf("        CONFIRMED: process exited within %v — stdin-EOF false-trigger bug present.\n", probeWindow)
	}

	// (b) Validate the fix direction: hold stdin open via io.Pipe().
	fmt.Printf("    (b) FIX validation: load-stream with live io.Pipe() stdin\n")
	fmt.Printf("        Waiting %v to see if the remote process stays alive...\n", probeWindow)
	alive, lines, err = streamSurvives(host, true, probeWindow)
	if err != nil {
		return fmt.Errorf("fix-validation probe: %w", err)
	}
	fmt.Printf("        Lines received: %d\n", len(lines))
	if !alive {
		return fmt.Errorf("fix validation FAILED: process exited within %v even with live stdin pipe — unexpected", probeWindow)
	}
	fmt.Printf("        CONFIRMED: process stayed alive for full %v with live stdin pipe — fix direction valid.\n", probeWindow)

	if len(lines) == 0 {
		return fmt.Errorf("no NDJSON lines received from load-stream — check PATH/binary on remote host")
	}
	fmt.Printf("        First sample (truncated): %.120s\n", lines[0])

	fmt.Println("    OK")
	return nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func timeCmd(cmd *exec.Cmd) (time.Duration, error) {
	start := time.Now()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("%w\n%s", err, strings.TrimSpace(string(out)))
	}
	return time.Since(start), nil
}
