package exec

import "path/filepath"

func isGearInvocation(arg0 string) bool {
	base := filepath.Base(arg0)
	return base == "⚙" || base == "⚙️" || base == "\xe2\x9a\x99" || base == "\xe2\x9a\x99\xef\xb8\x8f"
}
