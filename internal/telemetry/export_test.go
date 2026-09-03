package telemetry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestBuildExport_ScrubsWorkingDir constructs a fixture row containing a
// real-looking absolute path in WorkingDir (issue 204's core requirement)
// and asserts, by string search over the serialized JSON output, that the
// raw absolute path never appears anywhere in the export — not just that
// some normalization function was invoked.
func TestBuildExport_ScrubsWorkingDir(t *testing.T) {
	rawPath := "/home/testuser/projects/foo"
	rows := []ToolCall{
		{
			CreatedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			SessionID:   "sess-abc-123",
			TicketID:    "204",
			ProjectName: "foo",
			WorkingDir:  rawPath,
			AgentID:     "claude",
			ToolName:    "Read",
			CallType:    "internal",
			RawBytes:    100,
		},
	}

	exp := BuildExport(rows, time.Now())
	if len(exp.ToolCalls) != 1 {
		t.Fatalf("expected 1 exported row, got %d", len(exp.ToolCalls))
	}
	if got := exp.ToolCalls[0].ProjectDir; got != "foo" {
		t.Errorf("ProjectDir = %q, want %q", got, "foo")
	}

	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)

	if strings.Contains(out, rawPath) {
		t.Fatalf("raw absolute path %q leaked into export JSON: %s", rawPath, out)
	}
	if strings.Contains(out, "testuser") {
		t.Fatalf("raw username fragment leaked into export JSON: %s", out)
	}
	if strings.Contains(out, "/home/") {
		t.Fatalf("absolute path prefix leaked into export JSON: %s", out)
	}
}

// TestBuildExport_ScrubsAbsolutePathTicketAndProject verifies that even if
// TicketID or ProjectName were ever populated with an absolute path
// (rather than the expected short identifier), the export still strips it
// down to a base name and never leaks the raw path.
func TestBuildExport_ScrubsAbsolutePathTicketAndProject(t *testing.T) {
	rawPath := "/home/testuser/projects/bar"
	rows := []ToolCall{
		{
			CreatedAt:   time.Now(),
			SessionID:   "sess-1",
			TicketID:    rawPath,
			ProjectName: rawPath,
			WorkingDir:  rawPath,
			AgentID:     "codex",
			ToolName:    "Edit",
			CallType:    "shell",
		},
	}

	exp := BuildExport(rows, time.Now())
	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)

	if strings.Contains(out, rawPath) {
		t.Fatalf("raw absolute path leaked into export JSON via ticket_id/project_name: %s", out)
	}
	if exp.ToolCalls[0].TicketID != "bar" || exp.ToolCalls[0].ProjectName != "bar" {
		t.Errorf("expected TicketID/ProjectName normalized to %q, got %q/%q", "bar", exp.ToolCalls[0].TicketID, exp.ToolCalls[0].ProjectName)
	}
}

// TestBuildExport_DropsNote asserts free-text Note content never appears
// in the export, since there is no safe automatic way to scrub arbitrary
// free text (it could contain anything, including emails or paths typed
// by a human).
func TestBuildExport_DropsNote(t *testing.T) {
	secret := "contact someone@example.com at /home/testuser/secret"
	rows := []ToolCall{
		{
			CreatedAt: time.Now(),
			SessionID: "sess-1",
			AgentID:   "claude",
			ToolName:  "Read",
			CallType:  "internal",
			Note:      secret,
		},
	}
	exp := BuildExport(rows, time.Now())
	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)
	if strings.Contains(out, "someone@example.com") || strings.Contains(out, "/home/testuser") {
		t.Fatalf("Note free text leaked into export JSON: %s", out)
	}
}

func TestExportAll(t *testing.T) {
	dir := t.TempDir()
	db, err := Open(dir + "/tool_catalog.sqlite")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	if err := db.Insert(ToolCall{
		SessionID:  "sess-1",
		WorkingDir: "/home/testuser/projects/foo",
		AgentID:    "claude",
		ToolName:   "Read",
		CallType:   "internal",
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	exp, err := ExportAll(db, time.Now())
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	if len(exp.ToolCalls) != 1 {
		t.Fatalf("expected 1 row, got %d", len(exp.ToolCalls))
	}
	if exp.ToolCalls[0].ProjectDir != "foo" {
		t.Errorf("ProjectDir = %q, want %q", exp.ToolCalls[0].ProjectDir, "foo")
	}
}

func TestBuildExport_IncludesActivityCategory(t *testing.T) {
	rows := []ToolCall{
		{
			CreatedAt: time.Now(),
			SessionID: "sess-1",
			AgentID:   "claude",
			ToolName:  "Edit",
			CallType:  "shell",
			Note:      "fixed bug in parser",
		},
		{
			CreatedAt: time.Now(),
			SessionID: "sess-1",
			AgentID:   "claude",
			ToolName:  "Bash",
			CallType:  "shell",
			Note:      "go test ./...",
		},
	}

	exp := BuildExport(rows, time.Now())
	if len(exp.ToolCalls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(exp.ToolCalls))
	}
	if exp.ToolCalls[0].ActivityCategory != CategoryEdit {
		t.Errorf("call 0 category = %q, want %q", exp.ToolCalls[0].ActivityCategory, CategoryEdit)
	}
	if exp.ToolCalls[1].ActivityCategory != CategoryTest {
		t.Errorf("call 1 category = %q, want %q", exp.ToolCalls[1].ActivityCategory, CategoryTest)
	}

	data, err := json.Marshal(exp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `"activity_category":"edit"`) {
		t.Errorf("expected activity_category edit in JSON output: %s", out)
	}
	if !strings.Contains(out, `"activity_category":"test"`) {
		t.Errorf("expected activity_category test in JSON output: %s", out)
	}
}
