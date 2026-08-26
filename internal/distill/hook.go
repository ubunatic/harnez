package distill

import "regexp"

// noisyCommandRE matches command lines worth auto-piping through distill:
// verbose test/build runners and git inspection commands. Deliberately an
// allowlist, not a denylist of "interactive" commands — missing an
// interactive case in a denylist silently mangles a session, whereas a
// missing case here just leaves a command unfiltered.
var noisyCommandRE = regexp.MustCompile(
	`(?:^|[;&|]\s*)(?:` +
		`go\s+(?:test|build|vet)\b` +
		`|cargo\s+(?:test|build)\b` +
		`|pytest\b` +
		`|npm\s+(?:test|run\s+test)\b` +
		`|make\s+(?:test|build|check)\b` +
		`|git\s+(?:status|diff|log)\b` +
		`)`,
)

// RewriteBashCommand decides whether a Bash tool command should be piped
// through `harnez distill` before it runs. It returns the rewritten command
// and true when a rewrite applies; otherwise the original command and false.
//
// The rewrite wraps the whole command in a subshell with `set -o pipefail`
// and merges stderr into stdout, so exit-code semantics of the original
// command survive the pipe (distill's own exit code would otherwise mask
// failures).
func RewriteBashCommand(command string) (string, bool) {
	if command == "" {
		return command, false
	}
	if !noisyCommandRE.MatchString(command) {
		return command, false
	}
	if distillInvocationRE.MatchString(command) {
		return command, false
	}
	return "set -o pipefail; ( " + command + " ) 2>&1 | harnez distill", true
}

var distillInvocationRE = regexp.MustCompile(`\bharnez\s+distill\b`)
