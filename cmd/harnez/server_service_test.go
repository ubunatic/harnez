package main

import (
	"bytes"
	"strings"
	"testing"

	"ubunatic.com/harnez/internal/server"
)

func TestPrintServerSystemdStatusShowsOwnerAndHandoff(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	printServerSystemdStatus(&output, cfg, server.SystemdStatus{Available: true, Installed: true, Active: true, MainPID: 100, SocketPID: 200})
	for _, want := range []string{"MainPID=100", "socket owner: another process (PID 200)", "systemctl --user restart " + cfg.SystemdUnitName} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("status output missing %q:\n%s", want, output.String())
		}
	}
}

func TestPrintServerSystemdStatusSuggestsInstallWhenUnitIsMissing(t *testing.T) {
	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	printServerSystemdStatus(&output, cfg, server.SystemdStatus{Available: true, MainPID: 0, SocketPID: 200})
	if !strings.Contains(output.String(), "harnez server install") {
		t.Fatalf("missing install suggestion: %q", output.String())
	}
	if strings.Contains(output.String(), "systemctl --user restart") {
		t.Fatalf("uninstalled unit got restart suggestion: %q", output.String())
	}
}

func TestPrintServerSystemdStatusExplainsUsageFallback(t *testing.T) {
	var output bytes.Buffer
	printServerSystemdStatus(&output, server.Config{}, server.SystemdStatus{SocketPID: 200})
	if !strings.Contains(output.String(), "harnez usage -w") || !strings.Contains(output.String(), "PID 200") {
		t.Fatalf("unsupported status output = %q", output.String())
	}
}
