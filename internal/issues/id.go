package issues

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var remoteIDRegex = regexp.MustCompile(`^GH[1-9]\d*$`)
var localIDRegex = regexp.MustCompile(`^\d+$`)

// NormalizeID accepts local numbers and case-insensitive GH IDs without mixing namespaces.
func NormalizeID(s string) (string, error) {
	s = strings.ToUpper(strings.TrimSpace(s))
	if remoteIDRegex.MatchString(s) {
		return s, nil
	}
	if localIDRegex.MatchString(s) {
		n, err := strconv.Atoi(s)
		if err == nil {
			return fmt.Sprintf("%03d", n), nil
		}
	}
	return "", fmt.Errorf("invalid ticket id %q (expected a local number or GH<number>)", s)
}

// LessID orders local tickets first, then GitHub IDs, numerically within each group.
// Compare digit lengths to avoid integer overflow on remote IDs.
func LessID(a, b string) bool {
	ar, br := strings.HasPrefix(a, "GH"), strings.HasPrefix(b, "GH")
	if ar != br {
		return !ar
	}
	an := strings.TrimLeft(strings.TrimPrefix(a, "GH"), "0")
	bn := strings.TrimLeft(strings.TrimPrefix(b, "GH"), "0")
	if len(an) != len(bn) {
		return len(an) < len(bn)
	}
	if an != bn {
		return an < bn
	}
	return a < b
}
