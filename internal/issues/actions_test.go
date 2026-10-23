package issues

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComposeStatusTable(t *testing.T) {
	cases := []struct {
		verb, reason, want string
		wantErr            bool
	}{
		{"open", "", "Open", false},
		{"open", "deferred", "Open — deferred", false},
		{"start", "", "In Progress", false},
		{"start", "picking this up to unblock 043", "In Progress — picking this up to unblock 043", false},
		{"block", "waiting on upstream", "Blocked — waiting on upstream", false},
		{"block", "", "", true},
		{"close", "", "Closed", false},
		{"close", "resolved", "Closed — resolved", false},
		{"done", "", "Closed", false},
		{"done", "resolved", "Closed — resolved", false},
		{"draft", "", "Draft", false},
		{"draft", "x", "Draft — x", false},
		{"backlog", "", "Backlog", false},
		{"backlog", "later", "Backlog — later", false},
		{"promote", "", "Open", false},
		{"review", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.verb+"/"+tc.reason, func(t *testing.T) {
			got, err := ComposeStatus(tc.verb, tc.reason)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("got %q, want error", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("ComposeStatus() = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestApplyStatusBatchInjectsAndRestoresIndexOperations(t *testing.T) {
	dir := t.TempDir()
	issuesDir := filepath.Join(dir, "issues")
	if err := os.MkdirAll(issuesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	ticket := filepath.Join(issuesDir, "042-example.md")
	original := []byte("# 042 — Example\n\n**Status**: Open\n")
	if err := os.WriteFile(ticket, original, 0o644); err != nil {
		t.Fatal(err)
	}
	readme := filepath.Join(issuesDir, "README.md")
	readmeOriginal := []byte("original index\n")
	if err := os.WriteFile(readme, readmeOriginal, 0o644); err != nil {
		t.Fatal(err)
	}
	updates, snapshots, restores := 0, 0, 0
	result, drift, err := ApplyStatusBatch("close", []string{"42"}, nil, StatusBatchOptions{
		Dir: dir, Check: true,
		SnapshotIndexes: func(string) (any, error) { snapshots++; return os.ReadFile(readme) },
		RestoreIndexes:  func(_ string, value any) error { restores++; return os.WriteFile(readme, value.([]byte), 0o644) },
		UpdateIndexes: func(string) (bool, error) {
			updates++
			return true, os.WriteFile(readme, []byte("updated index\n"), 0o644)
		},
	})
	if err != nil || !drift || len(result) != 1 || result[0].NewStatus != "Closed" {
		t.Fatalf("ApplyStatusBatch = %+v, drift=%v, err=%v", result, drift, err)
	}
	if updates != 1 || snapshots != 1 || restores != 1 {
		t.Fatalf("injected calls update/snapshot/restore = %d/%d/%d", updates, snapshots, restores)
	}
	gotTicket, err := os.ReadFile(ticket)
	if err != nil {
		t.Fatal(err)
	}
	gotReadme, err := os.ReadFile(readme)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotTicket) != string(original) || string(gotReadme) != string(readmeOriginal) {
		t.Fatalf("check left side effects: ticket=%q index=%q", gotTicket, gotReadme)
	}
}

func TestLifecycleActionsAndTicketSubject(t *testing.T) {
	if err := ValidateLifecycleTransition("backlog", "Closed"); err == nil {
		t.Fatal("closed ticket moved to backlog")
	}
	if err := ValidateLifecycleTransition("promote", "Open"); err == nil {
		t.Fatal("open ticket promoted")
	}
	if got, err := CloseStatus("close", "Closed — old", "Closed", false); err != nil || got != "Closed — old" {
		t.Fatalf("CloseStatus() = %q, %v", got, err)
	}
	if _, err := CloseStatus("start", "Open", "In Progress", true); err == nil {
		t.Fatal("invalid amend accepted")
	}
	if !MessageReferencesTicket("feat: close #0042", "42") || MessageReferencesTicket("feat: close 142", "42") {
		t.Fatal("ticket subject matching is incorrect")
	}
}

func TestDuplicateErrorUsesInjectedMatchesAndLifecycle(t *testing.T) {
	err := DuplicateError("Widget", []DuplicateMatch{
		{Number: "12", RawStatus: "Backlog", PlainTitle: "widget cleanup", Score: .93, Strong: true},
		{Number: "13", RawStatus: "Open", PlainTitle: "unrelated", Score: .2},
	}, 87)
	if err == nil || !strings.Contains(err.Error(), "87%") || !strings.Contains(err.Error(), "harnez issues promote 12") || strings.Contains(err.Error(), "13") {
		t.Fatalf("DuplicateError = %v", err)
	}
}

func TestRewriteIssueReferencesExactFilenames(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"issues/096-b.md.", "issues/097-b.md."},
		{"096-b.md, 096-b.md!", "097-b.md, 097-b.md!"},
		{"../issues/096-b.md#section", "../issues/097-b.md#section"},
		{"1096-b.md 096-b.md.bak 096 #096", "1096-b.md 096-b.md.bak 096 #096"},
		{"issues/096-b.md-extra", "issues/096-b.md-extra"},
	} {
		if got := RewriteReferences(tc.in, "issues/096-b.md", "issues/097-b.md"); got != tc.want {
			t.Errorf("%q => %q, want %q", tc.in, got, tc.want)
		}
	}
	if strings.Contains(RewriteReferences("issues/096-b.md", "issues/096-b.md", "issues/097-b.md"), "096-b") {
		t.Fatal("reference was not rewritten")
	}
}
