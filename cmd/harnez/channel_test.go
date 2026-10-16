package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"ubunatic.com/harnez/internal/channel"
	"ubunatic.com/harnez/internal/subagent"
)

func TestChannelSetupPrintsManualConfiguration(t *testing.T) {
	cfg, err := channel.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	executable := "/a path with spaces/harnez"
	if err := printChannelSetup(&output, cfg, executable); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	start := strings.Index(text, "{\n")
	end := strings.Index(text, "\n\nOptional (for unattended receipts")
	if start < 0 || end < start {
		t.Fatalf("missing MCP snippet: %s", text)
	}
	var config struct {
		Servers map[string]struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(text[start:end]), &config); err != nil {
		t.Fatal(err)
	}
	server := config.Servers[cfg.ServerName]
	if server.Type != "stdio" || server.Command != executable || len(server.Args) != 1 || server.Args[0] != "channel" {
		t.Fatalf("incorrect subprocess configuration: %#v", server)
	}

	settingsStart := strings.Index(text, "{\n  \"permissions\":")
	settingsEnd := strings.Index(text, "\n\nIn that directory")
	if settingsStart < 0 || settingsEnd < settingsStart {
		t.Fatalf("missing settings snippet: %s", text)
	}
	var settings struct {
		Permissions struct {
			Allow []string `json:"allow"`
		} `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(text[settingsStart:settingsEnd]), &settings); err != nil {
		t.Fatal(err)
	}
	expectedPermission := "mcp__" + cfg.ServerName + "__" + cfg.ReplyToolName
	if len(settings.Permissions.Allow) != 1 || settings.Permissions.Allow[0] != expectedPermission {
		t.Fatalf("wrong permissions allow entry: %#v (expected %q)", settings.Permissions.Allow, expectedPermission)
	}

	expectedLaunch := `claude --model ` + cfg.CanaryModel + ` --dangerously-load-development-channels server:` + cfg.ServerName + ` --append-system-prompt "`
	if !strings.Contains(text, expectedLaunch) || !strings.Contains(text, cfg.RegistryDir) {
		t.Fatalf("missing manual launch with append-system-prompt or registry path: %s", text)
	}
}

func TestChannelServeRequiresHostSessionID(t *testing.T) {
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	cmd := newChannelCmd()
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "CLAUDE_CODE_SESSION_ID") {
		t.Fatalf("server accepted missing host identity: %v", err)
	}
}

func TestChannelHelpExplainsProjectApprovalAndRestart(t *testing.T) {
	cmd := newChannelCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"New MCP server found in this project", "Use this MCP server", "restart", "reply tool"} {
		if !strings.Contains(out.String(), text) {
			t.Fatalf("missing help %q: %s", text, out.String())
		}
	}
}

func TestMergeChannelSessionsAndCodexFailureFallback(t *testing.T) {
	endpoints := []channel.Endpoint{
		{SessionID: "claude-2", PID: 22, URL: "http://127.0.0.1:2", WorkingDir: "/work/claude"},
		{SessionID: "claude-1", PID: 11, URL: "http://127.0.0.1:1", WorkingDir: "/work/claude"},
	}
	listed := mergeChannelSessions(endpoints, func() ([]channel.CodexThread, error) {
		return []channel.CodexThread{
			{ID: "thread-2", Name: "cati1", CWD: "/work/codex"},
			{ID: "thread-1", Name: "", CWD: "/work/other"},
		}, nil
	}, nil, "thread-2")
	if len(listed) != 4 || listed[0].Provider != "claude" || listed[0].SessionID != "claude-1" || listed[1].SessionID != "claude-2" || listed[2].Provider != "codex" || listed[2].SessionID != "thread-1" || listed[3].Name != "cati1" {
		t.Fatalf("merged sessions = %#v", listed)
	}
	if !listed[3].Live || listed[2].Live {
		t.Fatalf("current Codex thread liveness = %#v", listed[2:])
	}
	listed = mergeChannelSessions(endpoints[:1], func() ([]channel.CodexThread, error) {
		return nil, errors.New("codex app-server unavailable")
	}, nil, "thread-2")
	if len(listed) != 1 || listed[0].Provider != "claude" || listed[0].SessionID != "claude-2" {
		t.Fatalf("Codex failure lost Claude sessions: %#v", listed)
	}
}

func TestChannelListOverviewAndLiveFiltering(t *testing.T) {
	listed := []listedChannel{
		{Provider: "claude", SessionID: "claude-live", PID: 11, URL: "http://127.0.0.1:1", WorkingDir: "/work/claude"},
		{Provider: "codex", SessionID: "codex-old", Name: "", WorkingDir: "/work/codex", Live: true},
		{Provider: "codex", SessionID: "codex-named", Name: "worker", WorkingDir: "/work/other"},
	}

	var overview bytes.Buffer
	if err := printChannelOverview(&overview, listed); err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"PROVIDER\tNAME\tID\tSTATUS", "claude\t-\tclaude-live\tlive", "codex\t-\tcodex-old\tlive", "codex\tworker\tcodex-named\tinactive", `Use "harnez channel list --json" or "harnez channel inspect <id>" for full details`} {
		if !strings.Contains(overview.String(), expected) {
			t.Fatalf("overview missing %q: %s", expected, overview.String())
		}
	}

	var names bytes.Buffer
	if err := printChannelProjection(&names, listed, true); err != nil {
		t.Fatal(err)
	}
	if names.String() != "claude-live\ncodex-old\nworker\n" {
		t.Fatalf("names projection = %q", names.String())
	}
	var ids bytes.Buffer
	if err := printChannelProjection(&ids, listed, false); err != nil {
		t.Fatal(err)
	}
	if ids.String() != "claude-live\ncodex-old\ncodex-named\n" {
		t.Fatalf("IDs projection = %q", ids.String())
	}

	live := filterLiveChannelSessions(listed)
	if len(live) != 2 || live[0].SessionID != "claude-live" || live[1].SessionID != "codex-old" {
		t.Fatalf("live sessions = %#v", live)
	}

	var raw bytes.Buffer
	if err := json.NewEncoder(&raw).Encode(listed); err != nil {
		t.Fatal(err)
	}
	var decoded []listedChannel
	if err := json.Unmarshal(raw.Bytes(), &decoded); err != nil || len(decoded) != len(listed) || decoded[1].SessionID != "codex-old" {
		t.Fatalf("JSON output did not preserve full sessions: %#v, err=%v", decoded, err)
	}
}

func TestChannelListCommandFlags(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"overview_default_active_only", []string{"list"}, "codex\tcurrent-worker\tcodex-current\tlive"},
		{"names_default_active_only", []string{"list", "--names"}, "current-worker\n"},
		{"ids_default_active_only", []string{"list", "--ids"}, "codex-current\n"},
		{"json_default_active_only", []string{"list", "--json"}, `"session_id":"codex-current"`},
		{"all_overview", []string{"list", "--all"}, "codex\tworker\tcodex-named\tinactive"},
		{"all_names", []string{"list", "--all", "--names"}, "current-worker\n"},
		{"all_ids", []string{"list", "--all", "--ids"}, "codex-named\n"},
		{"all_json", []string{"list", "--all", "--json"}, `"session_id":"codex-old"`},
		{"session_id_fallback", []string{"list"}, "codex\tcurrent-worker\tcodex-current\tlive"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("CODEX_THREAD_ID", "")
			t.Setenv("CODEX_SESSION_ID", "")
			if tc.name == "session_id_fallback" {
				t.Setenv("CODEX_SESSION_ID", "codex-current")
			} else {
				t.Setenv("CODEX_THREAD_ID", "codex-current")
			}
			cmd := newChannelCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tc.args)
			listCmd, _, err := cmd.Find([]string{"list"})
			if err != nil {
				t.Fatal(err)
			}
			listCmd.RunE = func(cmd *cobra.Command, args []string) error {
				return runChannelList(cmd, args,
					func(context.Context, channel.Config) ([]channel.Endpoint, error) {
						return []channel.Endpoint{{SessionID: "claude-live", PID: 11, URL: "http://127.0.0.1:1", WorkingDir: "/work/claude"}}, nil
					},
					func(context.Context, channel.Config) ([]channel.CodexThread, error) {
						return []channel.CodexThread{{ID: "codex-current", Name: "current-worker", CWD: "/work/codex"}, {ID: "codex-old", CWD: "/work/codex"}, {ID: "codex-named", Name: "worker", CWD: "/work/other"}}, nil
					}, noActiveCodexProcesses)
			}
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output missing %q: %s", tc.want, out.String())
			}
			if !strings.Contains(strings.Join(tc.args, " "), "--all") {
				if strings.Contains(out.String(), "codex-old") || strings.Contains(out.String(), "codex-named") {
					t.Fatalf("default output included inactive session: %s", out.String())
				}
			} else if !strings.Contains(out.String(), "codex-old") {
				t.Fatalf("--all output omitted inactive session: %s", out.String())
			}
		})
	}
}

func noActiveCodexProcesses() ([]*subagent.Session, error) { return nil, nil }

func TestMergeChannelSessionsMarksActiveProcessSessionsLive(t *testing.T) {
	threads := func() ([]channel.CodexThread, error) {
		return []channel.CodexThread{
			{ID: "thread-listed", Name: "listed", CWD: "/work/a"},
			{ID: "thread-idle", Name: "idle", CWD: "/work/b"},
		}, nil
	}
	active := []*subagent.Session{
		{ProviderSessionID: "thread-listed", Provider: "codex", Name: "process-name", WorkingDir: "/work/a"},
		{ProviderSessionID: "thread-unlisted", Provider: "codex", Name: "proc-worker", WorkingDir: "/work/c"},
	}
	listed := mergeChannelSessions(nil, threads, active, "")
	wantLive := map[string]bool{"thread-listed": true, "thread-idle": false, "thread-unlisted": true}
	if len(listed) != len(wantLive) {
		t.Fatalf("merged sessions (duplicates or missing?) = %#v", listed)
	}
	for _, session := range listed {
		want, ok := wantLive[session.SessionID]
		if !ok || session.Live != want {
			t.Fatalf("session %q live = %v, want %v (known=%v)", session.SessionID, session.Live, want, ok)
		}
		if session.SessionID == "thread-listed" && session.Name != "listed" {
			t.Fatalf("app-server name lost for matched thread: %#v", session)
		}
		if session.SessionID == "thread-unlisted" && (session.Name != "proc-worker" || session.WorkingDir != "/work/c") {
			t.Fatalf("process-only session lost its details: %#v", session)
		}
	}
}

func TestMergeChannelSessionsEnvOverrideWithProcessDiscovery(t *testing.T) {
	threads := func() ([]channel.CodexThread, error) {
		return []channel.CodexThread{{ID: "thread-env"}, {ID: "thread-proc"}, {ID: "thread-none"}}, nil
	}
	active := []*subagent.Session{
		{ProviderSessionID: "thread-proc", Provider: "codex"},
		{Provider: "codex"}, // no ID: must be skipped, not listed as an empty session
	}
	listed := mergeChannelSessions(nil, threads, active, "thread-env")
	if len(listed) != 3 {
		t.Fatalf("merged sessions = %#v", listed)
	}
	wantLive := map[string]bool{"thread-env": true, "thread-proc": true, "thread-none": false}
	for _, session := range listed {
		if session.Live != wantLive[session.SessionID] {
			t.Fatalf("session %q live = %v, want %v", session.SessionID, session.Live, wantLive[session.SessionID])
		}
	}
}

func TestMergeChannelSessionsKeepsProcessSessionsWhenAppServerFails(t *testing.T) {
	active := []*subagent.Session{{ProviderSessionID: "thread-proc", Provider: "codex", Name: "proc"}}
	listed := mergeChannelSessions(nil, func() ([]channel.CodexThread, error) {
		return nil, errors.New("codex app-server unavailable")
	}, active, "")
	if len(listed) != 1 || listed[0].SessionID != "thread-proc" || !listed[0].Live {
		t.Fatalf("app-server failure dropped process-matched session: %#v", listed)
	}
}

func runChannelListForTest(t *testing.T, args []string, currentThreadID string, threads func(context.Context, channel.Config) ([]channel.CodexThread, error), active func() ([]*subagent.Session, error)) (string, error) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_THREAD_ID", currentThreadID)
	t.Setenv("CODEX_SESSION_ID", "")
	cmd := newChannelCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	listCmd, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatal(err)
	}
	listCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return runChannelList(cmd, args,
			func(context.Context, channel.Config) ([]channel.Endpoint, error) { return nil, nil },
			threads, active)
	}
	err = cmd.Execute()
	return out.String(), err
}

func TestChannelListShowsActiveProcessSessionsByDefault(t *testing.T) {
	threads := func(context.Context, channel.Config) ([]channel.CodexThread, error) {
		return []channel.CodexThread{{ID: "codex-listed", Name: "listed", CWD: "/work/a"}, {ID: "codex-old", CWD: "/work/a"}}, nil
	}
	active := func() ([]*subagent.Session, error) {
		return []*subagent.Session{{ProviderSessionID: "codex-proc", Provider: "codex", Name: "proc-worker", WorkingDir: "/work/proc"}}, nil
	}
	out, err := runChannelListForTest(t, []string{"list"}, "", threads, active)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "codex\tproc-worker\tcodex-proc\tlive") {
		t.Fatalf("active process session missing from default list: %s", out)
	}
	if strings.Contains(out, "codex-listed") || strings.Contains(out, "codex-old") {
		t.Fatalf("default list included inactive threads: %s", out)
	}
}

func TestChannelListShowsSupervisedAgyInAllViews(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "agy.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	agy := &subagent.Session{ID: "agy-live-id", ProviderSessionID: "agy-conversation", Name: "agy-worker", Provider: "agy", ProcessPID: os.Getpid(), ControlSocket: socket, WorkingDir: "/work/agy"}
	listed := mergeChannelSessions(nil, func() ([]channel.CodexThread, error) { return nil, nil }, []*subagent.Session{agy}, "")
	if len(listed) != 1 || listed[0].Provider != "agy" || listed[0].SessionID != "agy-live-id" || !isLiveChannelSession(listed[0]) {
		t.Fatalf("AGY session not live: %#v", listed)
	}
	listed = filterLiveChannelSessions(listed)
	if len(listed) != 1 {
		t.Fatalf("default live filter dropped AGY session: %#v", listed)
	}
	var overview bytes.Buffer
	if err := printChannelOverview(&overview, listed); err != nil || !strings.Contains(overview.String(), "agy\tagy-worker\tagy-live-id\tlive") {
		t.Fatalf("overview = %q, err=%v", overview.String(), err)
	}
	for _, tc := range []struct{ flag, want string }{{"--names", "agy-worker\n"}, {"--ids", "agy-live-id\n"}} {
		var projection bytes.Buffer
		if err := printChannelProjection(&projection, listed, tc.flag == "--names"); err != nil || projection.String() != tc.want {
			t.Fatalf("%s projection = %q, err=%v", tc.flag, projection.String(), err)
		}
	}
	var raw bytes.Buffer
	if err := json.NewEncoder(&raw).Encode(listed); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw.String(), "agy-live-id") || !strings.Contains(raw.String(), "agy-worker") || !strings.Contains(raw.String(), "agy") {
		t.Fatalf("JSON omitted AGY session: %s", raw.String())
	}
}

func TestChannelListProcessErrorKeepsAppServerEnvOverride(t *testing.T) {
	threads := func(context.Context, channel.Config) ([]channel.CodexThread, error) {
		return []channel.CodexThread{{ID: "codex-env", Name: "env-worker"}, {ID: "codex-other"}}, nil
	}
	failing := func() ([]*subagent.Session, error) { return nil, errors.New("read process table") }
	out, err := runChannelListForTest(t, []string{"list"}, "codex-env", threads, failing)
	if err != nil {
		t.Fatalf("process discovery error must not fail channel list: %v", err)
	}
	if !strings.Contains(out, "codex\tenv-worker\tcodex-env\tlive") || strings.Contains(out, "codex-other") {
		t.Fatalf("env override lost when process discovery failed: %s", out)
	}
}

func TestChannelListAppServerErrorKeepsActiveProcessSessions(t *testing.T) {
	threads := func(context.Context, channel.Config) ([]channel.CodexThread, error) {
		return nil, errors.New("codex app-server unavailable")
	}
	active := func() ([]*subagent.Session, error) {
		return []*subagent.Session{{ProviderSessionID: "codex-proc", Provider: "codex", Name: "proc-worker"}}, nil
	}
	out, err := runChannelListForTest(t, []string{"list"}, "", threads, active)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "codex\tproc-worker\tcodex-proc\tlive") {
		t.Fatalf("process session dropped when app-server failed: %s", out)
	}
}

func TestChannelListRejectsConflictingProjectionFlags(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"list", "--names", "--ids"}, "--names and --ids are mutually exclusive"},
		{[]string{"list", "--json", "--names"}, "--json cannot be combined with --names or --ids"},
		{[]string{"list", "--json", "--ids"}, "--json cannot be combined with --names or --ids"},
	} {
		cmd := newChannelCmd()
		cmd.SetOut(io.Discard)
		cmd.SetErr(io.Discard)
		cmd.SetArgs(tc.args)
		listCmd, _, err := cmd.Find([]string{"list"})
		if err != nil {
			t.Fatal(err)
		}
		listCmd.RunE = func(cmd *cobra.Command, args []string) error {
			return runChannelList(cmd, args,
				func(context.Context, channel.Config) ([]channel.Endpoint, error) { return nil, nil },
				func(context.Context, channel.Config) ([]channel.CodexThread, error) { return nil, nil },
				noActiveCodexProcesses)
		}
		err = cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("args %v error = %v, want %q", tc.args, err, tc.want)
		}
	}
}
