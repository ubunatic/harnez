package server

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSystemdInstallUsesTempUnitDirAndStartsConfiguredUnit(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	unitDir := t.TempDir()
	var calls []string
	service := SystemdService{
		Config: cfg, UnitDir: unitDir, Executable: "/tmp/harnez", GOOS: "linux",
		LookPath: func(string) (string, error) { return "/fake/systemctl", nil },
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, strings.Join(args, " "))
			return nil, nil
		},
	}
	available, err := service.Install(context.Background())
	if err != nil || !available {
		t.Fatalf("Install() = available %t, error %v", available, err)
	}
	unit, err := os.ReadFile(filepath.Join(unitDir, cfg.SystemdUnitName))
	if err != nil {
		t.Fatal(err)
	}
	content := string(unit)
	for _, want := range []string{
		"ExecStart=/tmp/harnez server",
		"Restart=on-failure",
		"RestartSec=5s",
		"RuntimeDirectory=harnez",
		"RuntimeDirectoryPreserve=yes",
		"ReadWritePaths=%t/harnez",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("unit missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "{{") {
		t.Errorf("unit has unresolved placeholders:\n%s", content)
	}
	wantCalls := []string{"--user show-environment", "--user daemon-reload", "--user enable --now " + cfg.SystemdUnitName}
	if strings.Join(calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("systemctl calls = %q, want %q", calls, wantCalls)
	}
}

func TestSystemdUninstallAndStatusUseFakeSystemctl(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	unitDir := t.TempDir()
	unitPath := filepath.Join(unitDir, cfg.SystemdUnitName)
	if err := os.WriteFile(unitPath, []byte("unit"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls []string
	service := SystemdService{
		Config: cfg, UnitDir: unitDir, GOOS: "linux",
		LookPath: func(string) (string, error) { return "/fake/systemctl", nil },
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, strings.Join(args, " "))
			switch {
			case strings.Contains(strings.Join(args, " "), "MainPID"):
				return []byte("1234\n"), nil
			case strings.Contains(strings.Join(args, " "), "is-active"):
				return []byte("active\n"), nil
			default:
				return nil, nil
			}
		},
	}
	status, err := service.Status(context.Background(), 5678)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Available || !status.Installed || !status.Active || status.MainPID != 1234 || status.SocketPID != 5678 {
		t.Fatalf("Status() = %+v", status)
	}
	available, err := service.Uninstall(context.Background())
	if err != nil || !available {
		t.Fatalf("Uninstall() = available %t, error %v", available, err)
	}
	if _, err := os.Stat(unitPath); !os.IsNotExist(err) {
		t.Fatalf("unit still exists after uninstall: %v", err)
	}
	if got := calls[len(calls)-2:]; strings.Join(got, "\n") != "--user disable --now "+cfg.SystemdUnitName+"\n--user daemon-reload" {
		t.Fatalf("uninstall systemctl calls = %q", got)
	}
}

func TestSystemdStopStopsWithoutDisablingOrRemovingUnit(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	unitDir := t.TempDir()
	unitPath := filepath.Join(unitDir, cfg.SystemdUnitName)
	if err := os.WriteFile(unitPath, []byte("unit"), 0600); err != nil {
		t.Fatal(err)
	}
	var calls []string
	service := SystemdService{
		Config: cfg, UnitDir: unitDir, GOOS: "linux",
		LookPath: func(string) (string, error) { return "/fake/systemctl", nil },
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, strings.Join(args, " "))
			return nil, nil
		},
	}
	available, err := service.Stop(context.Background())
	if err != nil || !available {
		t.Fatalf("Stop() = available %t, error %v", available, err)
	}
	if got, want := strings.Join(calls, "\n"), "--user show-environment\n--user stop "+cfg.SystemdUnitName; got != want {
		t.Fatalf("systemctl calls = %q, want %q", got, want)
	}
	if _, err := os.Stat(unitPath); err != nil {
		t.Fatalf("unit removed by Stop: %v", err)
	}
}

func TestSystemdHandOverRestartsOnlyWhenUnitOwnsOldServer(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name      string
		installed bool
		oldPID    int
		want      bool
	}{
		{"unit owns old server", true, 1234, true},
		{"other process owns socket", true, 5678, false},
		{"unit not installed", false, 1234, false},
		{"unknown old PID", true, 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unitDir := t.TempDir()
			if tc.installed {
				if err := os.WriteFile(filepath.Join(unitDir, cfg.SystemdUnitName), []byte("old"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			var calls []string
			service := SystemdService{
				Config: cfg, UnitDir: unitDir, Executable: "/new/harnez", GOOS: "linux",
				LookPath: func(string) (string, error) { return "/fake/systemctl", nil },
				Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
					calls = append(calls, strings.Join(args, " "))
					if strings.Contains(strings.Join(args, " "), "MainPID") {
						return []byte("1234\n"), nil
					}
					return nil, nil
				},
			}
			handled, err := service.HandOver(context.Background(), tc.oldPID)
			if err != nil || handled != tc.want {
				t.Fatalf("HandOver() = %t, %v; want %t", handled, err, tc.want)
			}
			restarted := calls[len(calls)-1] == "--user restart "+cfg.SystemdUnitName
			if restarted != tc.want {
				t.Fatalf("restart = %t, calls %q", restarted, calls)
			}
			if !tc.want {
				return
			}
			unit, err := os.ReadFile(filepath.Join(unitDir, cfg.SystemdUnitName))
			if err != nil || !strings.Contains(string(unit), "ExecStart=/new/harnez server") {
				t.Fatalf("unit not rewritten to new executable: %q, %v", unit, err)
			}
		})
	}
}

func TestSystemdUnavailableDoesNotWriteAndSupportsNonLinux(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	unitDir := filepath.Join(t.TempDir(), "units")
	service := SystemdService{Config: cfg, UnitDir: unitDir, GOOS: "darwin"}
	available, err := service.Install(context.Background())
	if err != nil || available {
		t.Fatalf("Install() = available %t, error %v", available, err)
	}
	if _, err := os.Stat(unitDir); !os.IsNotExist(err) {
		t.Fatalf("unit directory created on unsupported host: %v", err)
	}
	service.GOOS = "linux"
	service.LookPath = func(string) (string, error) { return "", errors.New("not found") }
	available, err = service.Install(context.Background())
	if err != nil || available {
		t.Fatalf("Install() without systemctl = available %t, error %v", available, err)
	}
	if _, err := os.Stat(unitDir); !os.IsNotExist(err) {
		t.Fatalf("unit directory created without systemctl: %v", err)
	}
	service.LookPath = func(string) (string, error) { return "/fake/systemctl", nil }
	service.Run = func(context.Context, string, ...string) ([]byte, error) { return nil, errors.New("no user manager") }
	available, err = service.Install(context.Background())
	if err != nil || available {
		t.Fatalf("Install() without user manager = available %t, error %v", available, err)
	}
	if _, err := os.Stat(unitDir); !os.IsNotExist(err) {
		t.Fatalf("unit directory created without user manager: %v", err)
	}
}

func TestSystemdStartAndRestart(t *testing.T) {
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	service := SystemdService{
		Config: cfg, GOOS: "linux",
		LookPath: func(string) (string, error) { return "/fake/systemctl", nil },
		Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
			calls = append(calls, strings.Join(args, " "))
			return nil, nil
		},
	}
	avail, err := service.Start(context.Background())
	if err != nil || !avail {
		t.Fatalf("Start() = %t, %v", avail, err)
	}
	avail, err = service.Restart(context.Background())
	if err != nil || !avail {
		t.Fatalf("Restart() = %t, %v", avail, err)
	}
	want := []string{
		"--user show-environment",
		"--user start " + cfg.SystemdUnitName,
		"--user show-environment",
		"--user restart " + cfg.SystemdUnitName,
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("calls = %q, want %q", calls, want)
	}

	out, avail, err := service.Logs(context.Background(), true, 50)
	if err != nil || !avail {
		t.Fatalf("Logs() = %t, %v", avail, err)
	}
	_ = out
	wantLogs := append(want, "--user show-environment", "--user -u "+cfg.SystemdUnitName+" -n 50 -f")
	if strings.Join(calls, "\n") != strings.Join(wantLogs, "\n") {
		t.Fatalf("calls with logs = %q, want %q", calls, wantLogs)
	}
}
