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
		// further reported garbage: argument-value-shaped single tokens
		// (volume specs, ssh host:port, package@version pins, dotfiles,
		// bare punctuation) with no metacharacter or whitespace to catch
		// them, so they need the character-class check instead.
		{"models.json:ro,Z", "Bash"},
		{".minisign", "Bash"},
		{"cm-git@codeberg.org:22", "Bash"},
		{`entry.sh"`, "entry.sh"},
		{"voxi-modifierd@latest", "Bash"},
		{`macos-hello"`, "macos-hello"},
		{",", "Bash"},
		// shell builtins/keywords with no independent binary group under
		// Bash even as a single bare token, consistent with issue 344.
		{"cd", "Bash"},
		{"scripts/lint.sh", "scripts/lint.sh"},
	}
	for _, tc := range cases {
		got := CanonicalToolName(tc.raw)
		if got != tc.want {
			t.Errorf("CanonicalToolName(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}
