package claude

import "strings"

// quotaTestCommand is the quota-1 make target that managed text names. Repos
// without quota-1 rules get the plain test target in its place.
const (
	quotaTestCommand = "make test-q1"
	plainTestCommand = "make test"
)

// managedTextFor renders managed rule or doc text for a repo. Without quota-1
// rules it names the plain test target instead of the quota-1 one.
func managedTextFor(text string, quota bool) string {
	if quota {
		return text
	}
	return strings.ReplaceAll(text, quotaTestCommand, plainTestCommand)
}
