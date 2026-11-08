package main

import (
	"io"

	"github.com/spf13/cobra"
	internalexec "ubunatic.com/harnez/internal/exec"
	"ubunatic.com/harnez/internal/telemetry"
)

type execOptions = internalexec.Options
type execCall = internalexec.Call

const (
	defaultExecTimeout       = internalexec.DefaultExecTimeout
	defaultExecInsertTimeout = internalexec.DefaultExecInsertTimeout
	execTimeoutEnv           = internalexec.TimeoutEnv
	execTimeoutShortEnv      = internalexec.TimeoutShortEnv
	execTimeoutExplicitEnv   = internalexec.TimeoutExplicitEnv
	execTimeoutEffectiveEnv  = internalexec.TimeoutEffectiveEnv
	expectFailureEnv         = internalexec.ExpectFailureEnv
	quota1Env                = internalexec.Quota1Env
	quota1SandboxMarkerEnv   = internalexec.Quota1SandboxMarkerEnv
	quota1SandboxMarkerValue = internalexec.Quota1SandboxMarkerValue
)

func execDependencies(opts execOptions) execOptions {
	if opts.AgentID == "" {
		opts.AgentID = detectAgent("", opts.Getenv)
	}
	if opts.SendTelemetryRecord == nil {
		opts.SendTelemetryRecord = sendTelemetryRecord
	}
	return opts
}

func newExecCmd() *cobra.Command {
	return newExecCommand()
}

func newExecHookCmd() *cobra.Command {
	return newExecHookCommand()
}

func runExecWrapper(args []string, opts execOptions, in io.Reader, out, errOut io.Writer) (int, error) {
	return internalexec.RunWrapper(args, execDependencies(opts), in, out, errOut)
}

func recordExecTelemetry(opts execOptions, call execCall) {
	internalexec.RecordTelemetry(execDependencies(opts), call)
}

func defaultInsertExecRow(dbPath string, call telemetry.ToolCall) error {
	return internalexec.InsertExecRow(dbPath, call, sendTelemetryRecord)
}

func runExecHook(in io.Reader, out io.Writer) error {
	return internalexec.RunHook(in, out)
}

func exitCodeFromError(err error) int { return internalexec.ExitCodeFromError(err) }

func inferToolFromArgs(args []string, fallback string) string {
	return internalexec.InferToolFromArgs(args, fallback)
}

func validateToolName(tool string) error { return internalexec.ValidateToolName(tool) }

func alreadyRoutedThroughExec(command string) bool {
	return internalexec.AlreadyRoutedThroughExec(command)
}

func formatGearRewrite(command, distillFlag string) string {
	return internalexec.FormatGearRewrite(command, distillFlag)
}

func shellQuote(s string) string { return internalexec.ShellQuote(s) }

func debugLog(format string, args ...any) { internalexec.DebugLog(format, args...) }

func isGearInvocation(arg0 string) bool { return internalexec.IsGearInvocation(arg0) }
