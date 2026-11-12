package claude

import "testing"

func TestPathEntryDirSkipsSystemBinDirs(t *testing.T) {
	if got := pathEntryDir("/usr/bin"); got != "" {
		t.Errorf("pathEntryDir(/usr/bin) = %q, want empty", got)
	}
	if got := pathEntryDir("/home/u/go/bin"); got != "/home/u/go/bin" {
		t.Errorf("pathEntryDir(go/bin) = %q", got)
	}
}
