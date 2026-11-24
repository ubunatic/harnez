package exec

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ubunatic.com/harnez/internal/claude"
	"ubunatic.com/harnez/internal/quota1"
)

// quota1ActiveEnv is set to "1" in the environment of every Quota-1 child that
// harnez exec starts. A go test run nested inside an active Quota-1 run (for
// example a subprocess test under make test-q1) must not be counted a second
// time, so detection skips go test commands while it is set.
const quota1ActiveEnv = "HARNEZ_QUOTA1_ACTIVE"

// shellAssignRE matches a leading NAME=value word, such as GOWORK=off.
var shellAssignRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)

// shellPrefixWords may stand before the command word without being the
// command themselves: reserved words, and wrappers that only re-dispatch
// their arguments. `env`, `timeout`, `nice` and `time` are handled separately
// in skipCommandPrefix because they accept options (and timeout's duration).
var shellPrefixWords = map[string]bool{
	"!": true, "{": true, "}": true, "if": true, "then": true, "else": true,
	"elif": true, "do": true, "while": true, "until": true,
	"exec": true, "command": true,
}

var (
	envValueFlags = map[string]bool{
		"-u": true, "--unset": true,
		"-C": true, "--chdir": true,
		"-S": true, "--split-string": true,
	}
	timeoutValueFlags = map[string]bool{
		"-s": true, "--signal": true,
		"-k": true, "--kill-after": true,
	}
	niceValueFlags = map[string]bool{
		"-n": true, "--adjustment": true,
	}
	timeValueFlags = map[string]bool{
		"-f": true, "--format": true,
		"-o": true, "--output": true,
	}
)

// runsGoTest reports whether args run `go test` or `gotestsum` in command
// position. args is either a direct argv or a shell -c script; a script is
// split into its simple commands so that `cd sub && gotestsum ./...` matches
// while `echo "go test"` and `rg "go test" .` do not.
func runsGoTest(args []string) bool {
	commands := [][]string{args}
	// unwrapShellCommand returns exactly one script word for shell -c forms,
	// which is the only case where len(args) > 1 and the result has length 1.
	if script := unwrapShellCommand(args); len(args) > 1 && len(script) == 1 {
		commands = shellSimpleCommands(script[0])
	}
	for _, words := range commands {
		if isGoTestCommand(words) {
			return true
		}
	}
	return false
}

// isGoTestCommand reports whether one simple command (its words, quotes
// removed) is `go test ...` or `gotestsum ...`. Leading assignments and
// prefix words are skipped first, and the executable is compared by base name
// so /usr/local/go/bin/go matches too.
func isGoTestCommand(words []string) bool {
	words = skipCommandPrefix(words)
	if len(words) == 0 {
		return false
	}
	switch filepath.Base(words[0]) {
	case "gotestsum":
		return true
	case "go":
		return goSubcommand(words[1:]) == "test"
	}
	return false
}

// skipCommandPrefix drops leading assignments, prefix words, and command
// wrappers (`env`, `timeout`, `nice`, `time`) together with their options so
// the first remaining word is the command position.
func skipCommandPrefix(words []string) []string {
	for len(words) > 0 {
		w := words[0]
		switch filepath.Base(w) {
		case "env":
			words = skipFlagOptions(words[1:], envValueFlags)
			continue
		case "timeout":
			words = skipFlagOptions(words[1:], timeoutValueFlags)
			if len(words) > 0 {
				words = words[1:] // duration
			}
			continue
		case "nice":
			words = skipFlagOptions(words[1:], niceValueFlags)
			continue
		case "time":
			words = skipFlagOptions(words[1:], timeValueFlags)
			continue
		}
		if shellAssignRE.MatchString(w) || shellPrefixWords[w] {
			words = words[1:]
			continue
		}
		return words
	}
	return words
}

// skipFlagOptions consumes leading flag tokens (stopping at `--` or the first
// non-flag word). Flags in valueFlags consume the following word as their value.
func skipFlagOptions(words []string, valueFlags map[string]bool) []string {
	for len(words) > 0 {
		arg := words[0]
		if arg == "--" {
			return words[1:]
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return words
		}
		words = words[1:]
		if valueFlags[arg] && len(words) > 0 {
			words = words[1:]
		}
	}
	return words
}

// goSubcommand returns the first word after `go` that is not a flag. `go -C
// dir` takes a value, so that value is skipped as well.
func goSubcommand(args []string) string {
	sub, _ := goSubcommandAndArgs(args)
	return sub
}

// shellSimpleCommands splits a shell script into its simple commands and
// returns each command's words with quotes and backslash escapes removed.
// It is a small scanner, not a shell parser: it understands single and double
// quotes, backslash escapes and # comments, and treats ; & | ( ) newline and
// backtick as command boundaries. && and || yield empty segments, which are
// dropped. Operators inside quotes never split a command.
func shellSimpleCommands(script string) [][]string {
	var (
		commands [][]string
		words    []string
		word     strings.Builder
		inWord   bool
		single   bool
		double   bool
		escaped  bool
	)
	endWord := func() {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
	}
	endCommand := func() {
		endWord()
		if len(words) > 0 {
			commands = append(commands, words)
		}
		words = nil
	}

	runes := []rune(script)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case escaped:
			word.WriteRune(r)
			inWord = true
			escaped = false
		case single:
			if r == '\'' {
				single = false
			} else {
				word.WriteRune(r)
			}
		case double:
			switch r {
			case '"':
				double = false
			case '\\':
				escaped = true
			default:
				word.WriteRune(r)
			}
		case r == '\\':
			escaped, inWord = true, true
		case r == '\'':
			single, inWord = true, true
		case r == '"':
			double, inWord = true, true
		case r == ' ' || r == '\t':
			endWord()
		case r == '#' && !inWord:
			// A comment runs to the end of the line; stop on the newline so
			// the loop ends the command there.
			for i+1 < len(runes) && runes[i+1] != '\n' {
				i++
			}
		case strings.ContainsRune(";&|()\n`", r):
			endCommand()
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	endCommand()
	return commands
}

// quota1Configured reports whether the repository at opts.Quota1Dir uses
// Quota-1: .harnez/repo.yaml sets rules.quota, the Quota.md rule file exists,
// or a Quota-1 state file from an earlier run is already present. A repo with
// none of these keeps plain go test unblocked.
func quota1Configured(opts execOptions) bool {
	stateFile, root, err := quota1.ResolveStateFile(opts.Quota1Dir)
	if err != nil {
		return false
	}
	if _, err := os.Stat(stateFile); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(root, ".harnez", "rules", "Quota.md")); err == nil {
		return true
	}
	cfg, err := claude.LoadRepoConfig(root)
	return err == nil && cfg.Rules.Quota
}

// quota1DetectedGoTest reports whether a plain or coverage go test (or
// gotestsum) run should count toward the Quota-1 budget. It is false inside
// an active Quota-1 run, so nested test commands are never double-counted.
func quota1DetectedGoTest(opts execOptions, getenv func(string) string, args []string) bool {
	if getenv(quota1ActiveEnv) == "1" {
		return false
	}
	return runsGoTest(args) && quota1Configured(opts)
}

var goTestValueFlags = map[string]bool{
	"C": true, "p": true, "covermode": true, "coverpkg": true,
	"asmflags": true, "buildmode": true, "compiler": true,
	"gccgoflags": true, "gcflags": true, "installsuffix": true, "ldflags": true,
	"mod": true, "modfile": true, "overlay": true, "pgo": true,
	"pkgdir": true, "tags": true, "toolexec": true, "exec": true,
	"bench": true, "test.bench": true, "benchtime": true, "blockprofile": true, "blockprofilerate": true,
	"count": true, "coverprofile": true, "cpu": true, "cpuprofile": true,
	"fuzz": true, "test.fuzz": true, "fuzzminimizetime": true, "fuzztime": true, "list": true,
	"memprofile": true, "memprofilerate": true, "mutexprofile": true, "mutexprofilefraction": true,
	"outputdir": true, "parallel": true, "run": true, "test.run": true, "skip": true,
	"timeout": true, "trace": true, "vet": true, "o": true, "shuffle": true,
}

var makeValueFlags = map[string]bool{
	"-C": true, "--directory": true,
	"-f": true, "--file": true, "--makefile": true,
	"-I": true, "--include-dir": true,
	"-o": true, "--old-file": true, "--assume-old": true,
	"-W": true, "--what-if": true, "--new-file": true, "--assume-new": true,
	"-E": true, "--eval": true,
}

// classifyQuota1Tier determines the Quota-1 tier for a command (direct argv or
// shell -c script) by taking the maximum tier across test-executing segments.
// Unrecognised commands (e.g. explicit --quota-1 custom scripts) default to TierFull.
func classifyQuota1Tier(args []string) int {
	commands := [][]string{args}
	if script := unwrapShellCommand(args); len(args) > 1 && len(script) == 1 {
		commands = shellSimpleCommands(script[0])
	}
	maxTier := 0
	for _, words := range commands {
		if tier, ok := classifySimpleCommandTier(words); ok && tier > maxTier {
			maxTier = tier
		}
	}
	if maxTier == 0 {
		return quota1.TierFull
	}
	return maxTier
}

func classifySimpleCommandTier(words []string) (int, bool) {
	words = skipCommandPrefix(words)
	if len(words) == 0 {
		return 0, false
	}
	switch filepath.Base(words[0]) {
	case "gotestsum":
		return classifyGoTestArgs(gotestsumGoTestArgs(words[1:])), true
	case "go":
		sub, rest := goSubcommandAndArgs(words[1:])
		if sub == "test" {
			return classifyGoTestArgs(rest), true
		}
	case "make":
		return classifyMakeArgs(words[1:]), true
	}
	return 0, false
}

func gotestsumGoTestArgs(args []string) []string {
	for i, arg := range args {
		if arg == "--" {
			return args[i+1:]
		}
	}
	return args
}

func goSubcommandAndArgs(args []string) (string, []string) {
	for i := 0; i < len(args); i++ {
		switch arg := args[i]; {
		case arg == "-C":
			i++
		case strings.HasPrefix(arg, "-"):
		default:
			return arg, args[i+1:]
		}
	}
	return "", nil
}

func classifyGoTestArgs(args []string) int {
	hasFilter := false
	isShort := false
	var targets []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-args" {
			break
		}
		if arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			name := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
			flagName, flagVal, hasEq := strings.Cut(name, "=")
			switch flagName {
			case "run", "test.run", "bench", "test.bench", "fuzz", "test.fuzz":
				hasFilter = true
			case "short", "test.short":
				isShort = !hasEq || flagVal == "true" || flagVal == "1"
			}
			if !hasEq && goTestValueFlags[flagName] && i+1 < len(args) {
				i++
			}
			continue
		}
		targets = append(targets, arg)
	}

	if hasFilter {
		return quota1.TierPackage
	}
	repoWide := false
	for _, t := range targets {
		if t == "./..." || t == "..." {
			repoWide = true
			break
		}
	}
	if !repoWide {
		return quota1.TierPackage
	}
	if isShort {
		return quota1.TierShort
	}
	return quota1.TierFull
}

func classifyMakeArgs(args []string) int {
	maxTier := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			continue
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			if strings.Contains(arg, "=") {
				continue
			}
			if makeValueFlags[arg] && i+1 < len(args) {
				i++
				continue
			}
			if (arg == "-j" || arg == "--jobs") &&
				i+1 < len(args) && isAllDigits(args[i+1]) {
				i++
			} else if (arg == "-l" || arg == "--load-average") &&
				i+1 < len(args) && isDecimalNumber(args[i+1]) {
				i++
			}
			continue
		}
		if strings.Contains(arg, "=") {
			continue
		}
		var tier int
		switch arg {
		case "test-pkg":
			tier = quota1.TierPackage
		case "test-short":
			tier = quota1.TierShort
		default:
			tier = quota1.TierFull
		}
		if tier > maxTier {
			maxTier = tier
		}
	}
	if maxTier == 0 {
		return quota1.TierFull
	}
	return maxTier
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func isDecimalNumber(s string) bool {
	if s == "" {
		return false
	}
	seenDot := false
	seenDigit := false
	for _, r := range s {
		if r == '.' {
			if seenDot {
				return false
			}
			seenDot = true
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
		seenDigit = true
	}
	return seenDigit
}
