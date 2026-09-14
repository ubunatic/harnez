package telemetry

import "testing"

func TestCanonicalToolName(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"", ""},
		{"Bash", "Bash"},
		{"git", "git"},
		{"heartbeat", "heartbeat"},
		{"git status", "git"},
		{"git status --short", "git"},
		{"git status -s", "git"},
		{"harnez index -d .", "harnez"},
		{"harnez find issues next", "harnez"},
		{"make check && make install", "make"},
		{"which harnez || true", "which"},
		{"termaid 2>&1 || true", "termaid"},
		{"for", "Bash"},
		{"&&", "Bash"},
		{"*.lock", "Bash"},
		{"README.md", "Bash"},
		{"*.lock && git add -u && git commit -m \"chore\" || true", "Bash"},
		// issue 344 follow-up: mis-escaped/manual --tool values with a
		// stray trailing quote must not defeat data-file detection.
		{`crispasr-linux-x86_64.tar.gz"`, "Bash"},
		{`test-crispasr.tar.gz`, "Bash"},
		{`crispasr-linux-x86_64.tar.gz" 2>&1 | head -n 40`, "Bash"},
		{"crispasr --version", "crispasr"},
	}
	for _, tc := range cases {
		got := CanonicalToolName(tc.raw)
		if got != tc.want {
			t.Errorf("CanonicalToolName(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
