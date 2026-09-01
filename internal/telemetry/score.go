package telemetry

import (
	"fmt"
	"regexp"
)

var (
	// Score 1: Critical failures, runtime panics, crashes, missing tools
	reGoPanic     = regexp.MustCompile(`(?m)(?:panic:\s+runtime\s+error|panic:\s+|fatal\s+error:|goroutine\s+\d+\s+\[)`)
	reSegfault    = regexp.MustCompile(`(?i)(?:SIGSEGV|SIGBUS|segmentation\s+fault|core\s+dumped)`)
	rePyCrash     = regexp.MustCompile(`(?m)(?:Traceback\s+\(most\s+recent\s+call\s+last\):|Fatal\s+Python\s+error:|Uncaught\s+exception)`)
	reNodeCrash   = regexp.MustCompile(`(?i)(?:UnhandledPromiseRejection|Unhandled\s+promise\s+rejection|FATAL\s+ERROR:)`)
	reCmdNotFound = regexp.MustCompile(`(?i)(?:command\s+not\s+found|Permission\s+denied)`)

	// Score 2: Compiler, syntax, type, and import errors
	reGoBuild    = regexp.MustCompile(`(?m)(?:undefined:\s+|cannot\s+use\s+.*as\s+type|syntax\s+error:|imported\s+and\s+not\s+used|cannot\s+find\s+package|declared\s+and\s+not\s+used|assignment\s+mismatch|type\s+.*has\s+no\s+field\s+or\s+method|not\s+enough\s+arguments|too\s+many\s+arguments|missing\s+return)`)
	rePySyntax   = regexp.MustCompile(`(?m)(?:IndentationError:|NameError:|AttributeError:|ModuleNotFoundError:|ImportError:)`)
	reNodeSyntax = regexp.MustCompile(`(?m)(?:ReferenceError:|Cannot\s+find\s+module)`)
	reTypeSyntax = regexp.MustCompile(`(?m)(?:SyntaxError:|TypeError:)`)
	reRustBuild  = regexp.MustCompile(`(?m)(?:error\[E\d+\]:|mismatched\s+types)`)
	reCBuild     = regexp.MustCompile(`(?m)(?:\b(?:gcc|clang|g\+\+):\s+error:|\berror:\s+)`)

	// Score 3: Test failures and linter violations
	reGoTestFail = regexp.MustCompile(`(?m)(?:---\s+FAIL:|FAIL\t)`)
	reTestFail   = regexp.MustCompile(`(?m)(?:FAILED\s+\(failures=|FAILURES!|=== FAILED|\[FAIL\]|\bFAILED\b)`)
	reLintFail   = regexp.MustCompile(`(?m)(?:\.go:\d+:\d+:\s+.*\(.*\)|\[ERROR\]\s+\[lint\]|eslint.*error|pylint|flake8)`)
)

// ScoreShell inspects command output and exit code to derive a synthetic
// execution quality score (1-5) and a descriptive note.
//
// Scoring scale:
//   - 1: Critical failure, crash, runtime panic, segfault, unhandled rejection, command not found
//   - 2: Build / syntax / type failure, missing package, or unclassified non-zero exit
//   - 3: Test failure, assertion error, linter violation
//   - 5: Clean success (exit code 0 with no error signatures)
func ScoreShell(output string, exitCode int) (int, string) {
	// Signal termination (128+sig)
	if exitCode > 128 {
		sig := exitCode - 128
		return 1, fmt.Sprintf("signal termination (signal %d)", sig)
	}

	// 1. Critical crashes / panics
	if reGoPanic.MatchString(output) {
		return 1, "go runtime panic / fatal error"
	}
	if reSegfault.MatchString(output) {
		return 1, "segmentation fault / fatal signal"
	}
	if rePyCrash.MatchString(output) {
		return 1, "python traceback / uncaught exception"
	}
	if reNodeCrash.MatchString(output) {
		return 1, "unhandled promise rejection / fatal error"
	}
	if reCmdNotFound.MatchString(output) && exitCode != 0 {
		return 1, "command not found / permission denied"
	}

	// 2. Build & syntax errors
	if reGoBuild.MatchString(output) {
		return 2, "go build / syntax error"
	}
	if rePySyntax.MatchString(output) {
		return 2, "python syntax / type error"
	}
	if reNodeSyntax.MatchString(output) {
		return 2, "javascript syntax / type error"
	}
	if reTypeSyntax.MatchString(output) {
		return 2, "syntax / type error"
	}
	if reRustBuild.MatchString(output) {
		return 2, "rust compile error"
	}
	if reCBuild.MatchString(output) && exitCode != 0 {
		return 2, "c/c++ build error"
	}

	// 3. Test & lint failures
	if reGoTestFail.MatchString(output) {
		return 3, "go test failure"
	}
	if reTestFail.MatchString(output) && exitCode != 0 {
		return 3, "test failure"
	}
	if reLintFail.MatchString(output) && exitCode != 0 {
		return 3, "linter violation"
	}

	// Exit code evaluation
	if exitCode != 0 {
		return 2, fmt.Sprintf("command failed with exit code %d", exitCode)
	}

	return 5, "clean success"
}
