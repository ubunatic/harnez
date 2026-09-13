// bareflag implements issue 318's git-log-style bare "-N" limit shorthand
// (e.g. `harnez find issues -2`, `harnez issues list -2`), shared by both
// `find` and `issues list` so they never drift on this behavior. Cobra/pflag
// has no notion of a bare numeric shorthand -- git only supports `-N` because
// it does its own argv pre-processing before any flag parser runs -- so this
// rewrites a bare "-N" token into "--limit N" before root.Execute() sees it.
package main

import (
	"regexp"
	"strings"
)

// bareNumericFlagRe matches a token that is exactly a dash followed by one
// or more digits (e.g. "-2", "-15"). The anchors mean it only ever matches
// a whole token, never a substring of a longer one ("engine-2") and never a
// multi-character flag-shaped token ("-2x").
var bareNumericFlagRe = regexp.MustCompile(`^-[0-9]+$`)

// rewriteBareNumericLimit rewrites a single "-N" token in args into
// "--limit N". It is deliberately conservative:
//   - does nothing if an explicit -n/--limit flag is already present, so an
//     explicit flag always wins and is never silently overridden
//   - does nothing if more than one bare "-N" token is present, since which
//     one is the intended limit is ambiguous -- left for Cobra's own error
//     to surface rather than guessed
func rewriteBareNumericLimit(args []string) []string {
	bareIdx := -1
	for i, a := range args {
		switch {
		case a == "-n" || a == "--limit" || strings.HasPrefix(a, "--limit="):
			return args
		case bareNumericFlagRe.MatchString(a):
			if bareIdx != -1 {
				return args
			}
			bareIdx = i
		}
	}
	if bareIdx == -1 {
		return args
	}
	out := make([]string, 0, len(args)+1)
	out = append(out, args[:bareIdx]...)
	out = append(out, "--limit", strings.TrimPrefix(args[bareIdx], "-"))
	out = append(out, args[bareIdx+1:]...)
	return out
}

// rewriteArgsForBareLimit scans a full harnez argv (excluding the program
// name) and applies rewriteBareNumericLimit only to the two invocation
// shapes issue 318 scopes the shorthand to: `harnez find ...` and `harnez
// issues list ...`. Every other command -- including every other `issues`
// verb, where a bare number is a ticket-number positional argument, not a
// limit -- is left untouched.
func rewriteArgsForBareLimit(args []string) []string {
	if len(args) == 0 {
		return args
	}
	switch args[0] {
	case "find":
		return append(args[:1:1], rewriteBareNumericLimit(args[1:])...)
	case "issues":
		if len(args) > 1 && args[1] == "list" {
			return append(args[:2:2], rewriteBareNumericLimit(args[2:])...)
		}
	}
	return args
}
