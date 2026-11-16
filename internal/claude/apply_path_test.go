package claude

import "testing"

func TestPathEntryDirSkipsSystemBinDirs(t *testing.T) {
	if got := PathEntryDir("/usr/bin"); got != "" {
		t.Errorf("PathEntryDir(/usr/bin) = %q, want empty", got)
	}
	if got := PathEntryDir("/home/u/go/bin"); got != "/home/u/go/bin" {
		t.Errorf("PathEntryDir(go/bin) = %q", got)
	}
}
