package server

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"ubunatic.com/harnez"
)

type SystemdService struct {
	Config     Config
	FS         fs.FS
	UnitDir    string
	Executable string
	LookPath   func(string) (string, error)
	Run        func(context.Context, string, ...string) ([]byte, error)
	GOOS       string
}

type SystemdStatus struct {
	Available bool
	Installed bool
	Active    bool
	SocketPID int
	MainPID   int
}

func NewSystemdService(config Config) SystemdService {
	return SystemdService{Config: config, FS: harnez.DefaultFS, GOOS: runtime.GOOS}
}

func (s SystemdService) defaults() (SystemdService, error) {
	if s.FS == nil {
		s.FS = harnez.DefaultFS
	}
	if s.GOOS == "" {
		s.GOOS = runtime.GOOS
	}
	if s.LookPath == nil {
		s.LookPath = exec.LookPath
	}
	if s.Run == nil {
		s.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		}
	}
	if s.UnitDir == "" {
		base := os.Getenv("XDG_CONFIG_HOME")
		if !filepath.IsAbs(base) {
			home, err := os.UserHomeDir()
			if err != nil {
				return s, fmt.Errorf("resolve home directory: %w", err)
			}
			base = filepath.Join(home, ".config")
		}
		s.UnitDir = filepath.Join(base, "systemd", "user")
	}
	if s.Executable == "" {
		exe, err := os.Executable()
		if err != nil {
			return s, fmt.Errorf("resolve harnez executable: %w", err)
		}
		s.Executable = exe
	}
	return s, nil
}

func (s SystemdService) available(ctx context.Context) (SystemdService, string, bool, error) {
	s, err := s.defaults()
	if err != nil {
		return s, "", false, err
	}
	if s.GOOS != "linux" {
		return s, "", false, nil
	}
	systemctl, err := s.LookPath("systemctl")
	if err != nil {
		return s, "", false, nil
	}
	if _, err := s.Run(ctx, systemctl, "--user", "show-environment"); err != nil {
		return s, systemctl, false, nil
	}
	return s, systemctl, true, nil
}

func (s SystemdService) Install(ctx context.Context) (bool, error) {
	s, systemctl, available, err := s.available(ctx)
	if err != nil || !available {
		return available, err
	}
	data, err := fs.ReadFile(s.FS, s.Config.SystemdUnitSource)
	if err != nil {
		return true, fmt.Errorf("read systemd unit template: %w", err)
	}
	content := strings.NewReplacer(
		"{{HARNEZ_BIN}}", s.Executable,
		"{{RESTART_POLICY}}", s.Config.SystemdRestartPolicy,
		"{{RESTART_DELAY}}", s.Config.SystemdRestartDelay.String(),
	).Replace(string(data))
	if strings.Contains(content, "{{") {
		return true, fmt.Errorf("systemd unit template contains an unresolved placeholder")
	}
	if err := os.MkdirAll(s.UnitDir, 0755); err != nil {
		return true, fmt.Errorf("create systemd user unit directory: %w", err)
	}
	path := filepath.Join(s.UnitDir, s.Config.SystemdUnitName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return true, fmt.Errorf("write systemd user unit: %w", err)
	}
	if _, err := s.Run(ctx, systemctl, "--user", "daemon-reload"); err != nil {
		return true, fmt.Errorf("systemctl --user daemon-reload: %w", err)
	}
	if _, err := s.Run(ctx, systemctl, "--user", "enable", "--now", s.Config.SystemdUnitName); err != nil {
		return true, fmt.Errorf("enable systemd user service: %w", err)
	}
	return true, nil
}

func (s SystemdService) Uninstall(ctx context.Context) (bool, error) {
	s, systemctl, available, err := s.available(ctx)
	if err != nil || !available {
		return available, err
	}
	if _, err := s.Run(ctx, systemctl, "--user", "disable", "--now", s.Config.SystemdUnitName); err != nil {
		return true, fmt.Errorf("disable systemd user service: %w", err)
	}
	path := filepath.Join(s.UnitDir, s.Config.SystemdUnitName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return true, fmt.Errorf("remove systemd user unit: %w", err)
	}
	if _, err := s.Run(ctx, systemctl, "--user", "daemon-reload"); err != nil {
		return true, fmt.Errorf("systemctl --user daemon-reload: %w", err)
	}
	return true, nil
}

// HandOver points the unit at this executable and restarts it when the unit's
// MainPID is oldPID, so a newer build replaces the server under systemd instead
// of taking the socket outside it.
func (s SystemdService) HandOver(ctx context.Context, oldPID int) (bool, error) {
	status, err := s.Status(ctx, oldPID)
	if err != nil || !status.Installed || oldPID <= 0 || status.MainPID != oldPID {
		return false, nil
	}
	if _, err := s.Install(ctx); err != nil {
		return true, err
	}
	s, systemctl, _, err := s.available(ctx)
	if err != nil {
		return true, err
	}
	if _, err := s.Run(ctx, systemctl, "--user", "restart", s.Config.SystemdUnitName); err != nil {
		return true, fmt.Errorf("restart systemd user service: %w", err)
	}
	return true, nil
}

func (s SystemdService) Status(ctx context.Context, socketPID int) (SystemdStatus, error) {
	status := SystemdStatus{SocketPID: socketPID}
	s, systemctl, available, err := s.available(ctx)
	if err != nil || !available {
		status.Available = available
		return status, err
	}
	status.Available = true
	unitPath := filepath.Join(s.UnitDir, s.Config.SystemdUnitName)
	if _, err := os.Stat(unitPath); err == nil {
		status.Installed = true
	}
	mainPID, err := s.Run(ctx, systemctl, "--user", "show", s.Config.SystemdUnitName, "--property=MainPID", "--value")
	if err != nil {
		return status, fmt.Errorf("query systemd MainPID: %w", err)
	}
	status.MainPID, _ = strconv.Atoi(strings.TrimSpace(string(mainPID)))
	active, err := s.Run(ctx, systemctl, "--user", "is-active", s.Config.SystemdUnitName)
	status.Active = err == nil && strings.TrimSpace(string(active)) == "active"
	return status, nil
}
