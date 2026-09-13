// exitcode gives a RunE function a way to request a specific process exit
// code without calling os.Exit itself.
//
// Calling os.Exit inside RunE is a real anti-pattern, not just a style
// preference: os.Exit terminates the process immediately, so nothing after
// it in the call stack ever runs -- including main()'s executeAndRecord
// (cmd/harnez/clilog.go, issue 326), which is the one place cli_invocations
// writes its row. Every RunE that called os.Exit directly was therefore
// invisible to `harnez log`: not "recorded with the wrong exit code," but
// not recorded at all. Modern Cobra guidance (spf13/cobra#2124, #837, and
// the square/exit convention of picking the exit code at the application
// boundary) agrees: RunE should return an error, and exactly one place --
// main(), after root.Execute() returns -- should translate that into a
// process exit code.
//
// The wrinkle Cobra doesn't solve for you: a plain error always maps to
// exit 1, but several commands here need a specific code that isn't 1 --
// `--check`/`--exit-code` drift signals (git-diff-style: "found a
// difference", not "something went wrong") and exec/distill's subprocess
// passthrough (report the wrapped command's own exit code, which can be
// anything). exitCodeError carries that code through the normal RunE
// return path instead.
package main

import (
	"errors"

	"github.com/spf13/cobra"
)

// exitCodeError is a RunE return value meaning "exit with Code, and don't
// print anything extra" -- the command has already written whatever output
// is worth showing (a drift report, a wrapped subprocess's own stderr), so
// Cobra's default "Error: <message>" line would only be noise. Error()
// deliberately returns "" for exactly that reason.
type exitCodeError struct{ Code int }

func (e *exitCodeError) Error() string { return "" }

// exitCodeFromRunError maps a root.Execute return value to the process exit
// code the boundary in main() should use -- also the value
// cli_invocations.exit_code records for the row (cmd/harnez/clilog.go).
// Distinct from exec.go's exitCodeFromError, which extracts a wrapped
// subprocess's shell-convention exit code from an *exec.ExitError -- a
// different mapping over a different kind of error.
func exitCodeFromRunError(err error) int {
	if err == nil {
		return 0
	}
	var ec *exitCodeError
	if errors.As(err, &ec) {
		return ec.Code
	}
	return 1
}

// silenceIfExitCode marks cmd's error *and* usage output silenced when err
// is an exitCodeError, so Cobra prints nothing for a signal the command has
// already reported itself -- neither the "Error: " line nor (distill and
// exec don't otherwise set SilenceUsage) a full usage dump on every wrapped
// command that happens to exit non-zero. Cobra reads both SilenceErrors and
// SilenceUsage at print time, after RunE returns, so setting them here --
// only on this return path -- leaves the same command's other, genuine
// error returns printing exactly as before; it is not a command-wide
// SilenceErrors/SilenceUsage: true at construction time, which would also
// swallow those.
func silenceIfExitCode(cmd *cobra.Command, err error) error {
	var ec *exitCodeError
	if errors.As(err, &ec) {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
	}
	return err
}
