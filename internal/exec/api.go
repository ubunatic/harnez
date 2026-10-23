package exec

import (
	"io"
	"time"

	"ubunatic.com/harnez/internal/logging"
	"ubunatic.com/harnez/internal/server"
	"ubunatic.com/harnez/internal/telemetry"
)

// Options configures execution without coupling the implementation to Cobra.
type Options = execOptions

// Call describes the completed command for its telemetry row.
type Call = execCall

const (
	DefaultExecTimeout       = defaultExecTimeout
	DefaultExecInsertTimeout = defaultExecInsertTimeout
	TimeoutEnv               = execTimeoutEnv
	TimeoutShortEnv          = execTimeoutShortEnv
	TimeoutExplicitEnv       = execTimeoutExplicitEnv
	TimeoutEffectiveEnv      = execTimeoutEffectiveEnv
	ExpectFailureEnv         = expectFailureEnv
	Quota1Env                = quota1Env
	Quota1SandboxMarkerEnv   = quota1SandboxMarkerEnv
	Quota1SandboxMarkerValue = quota1SandboxMarkerValue
)

// RunWrapper executes args, streams their output, and records telemetry.
func RunWrapper(args []string, opts Options, in io.Reader, out, errOut io.Writer) (int, error) {
	return runExecWrapper(args, opts, in, out, errOut)
}

// RecordTelemetry makes a bounded best-effort write for a completed command.
func RecordTelemetry(opts Options, call Call) { recordExecTelemetry(opts, call) }

// RunHook processes the exec PreToolUse payload.
func RunHook(in io.Reader, out io.Writer) error { return runExecHook(in, out) }

// ResolveTimeout applies exec's timeout precedence rules.
func ResolveTimeout(opts Options, args []string) time.Duration { return resolveExecTimeout(opts, args) }

func ExitCodeFromError(err error) int { return exitCodeFromError(err) }
func InferToolFromArgs(args []string, fallback string) string {
	return inferToolFromArgs(args, fallback)
}
func ValidateToolName(tool string) error                   { return validateToolName(tool) }
func DetectExpectFailure(opts Options, args []string) bool { return detectExpectFailure(opts, args) }
func DetectQuota1(opts Options, args []string) bool        { return detectQuota1(opts, args) }
func HasExplicitTimeout(opts Options, args []string) bool  { return hasExplicitExecTimeout(opts, args) }
func IsQuota1SandboxMarker(value string) bool              { return isQuota1SandboxMarker(value) }
func Quota1SandboxArgs(args []string, cwd, goCache, goModCache, runtimeDir string) []string {
	return quota1SandboxArgs(args, cwd, goCache, goModCache, runtimeDir)
}
func AlreadyRoutedThroughExec(command string) bool { return alreadyRoutedThroughExec(command) }
func IsSimpleShellCommand(command string) bool     { return isSimpleShellCommand(command) }
func FormatGearRewrite(command, distillFlag string) string {
	return formatGearRewrite(command, distillFlag)
}
func ShellQuote(s string) string                           { return shellQuote(s) }
func DistillAutopipeRewrite(command string) (string, bool) { return distillAutopipeRewrite(command) }
func DebugLog(format string, args ...any)                  { logging.Debugf(format, args...) }
func IsGearInvocation(arg0 string) bool                    { return isGearInvocation(arg0) }

func InsertExecRow(dbPath string, call telemetry.ToolCall, send func(string, any) server.UpdateResult) error {
	return defaultInsertExecRow(dbPath, call, send)
}
