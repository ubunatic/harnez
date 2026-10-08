package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"ubunatic.com/harnez/internal/agymeter"
	"ubunatic.com/harnez/internal/procs"
	"ubunatic.com/harnez/internal/subagent"
)

func newAgyMeterCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "agy-meter-run -- command [args...]",
		Short:  "Run an AGY process with its private usage meter",
		Hidden: true,
		Args:   cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env := subagent.AgyLaunchEnv(os.Environ(), os.Getenv("HOME"))
			var err error
			if term.IsTerminal(int(os.Stdin.Fd())) {
				err = runAgyMeterSupervised(cmd.Context(), args[0], args[1:], env, os.Stdin, os.Stdout, os.Stderr, os.Getenv("HOME"))
			} else {
				err = agymeter.RunWithEnv(cmd.Context(), os.Getenv("HOME"), args[0], args[1:], env, os.Stdin, os.Stdout, os.Stderr)
			}
			if err == nil {
				return nil
			}
			if code := exitCodeFromError(err); code >= 0 {
				return silenceIfExitCode(cmd, &exitCodeError{Code: code})
			}
			return err
		},
	}
}

func runAgyMeterSupervised(ctx context.Context, command string, args, environ []string, stdin io.Reader, stdout, stderr io.Writer, home string) (runErr error) {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	storeDir := subagent.DefaultStoreDir()
	store, err := subagent.NewSessionStore(storeDir)
	if err != nil {
		return err
	}
	name, err := agyMeterSessionName(dir)
	if err != nil {
		return err
	}
	now := time.Now()
	session := &subagent.Session{ID: uuid.NewString(), Name: name, Provider: "agy", Source: "harnez-agy", Model: "agy-default", WorkingDir: dir, CallerPID: os.Getpid(), HarnessType: "interactive", Status: "active", CreatedAt: now, LastActiveAt: now}
	session.ControlSocket = filepath.Join(storeDir, session.ID+".sock")
	if err := store.Create(session); err != nil {
		return err
	}
	defer func() {
		latest, getErr := store.Get(session.ID)
		if getErr == nil {
			latest.LastActiveAt = time.Now()
			latest.ProcessPID = 0
			latest.ControlSocket = ""
			if latest.Status != "stopped" {
				latest.Status = "completed"
			}
			if saveErr := store.Save(latest); saveErr != nil && runErr == nil {
				runErr = saveErr
			}
		} else if runErr == nil {
			runErr = getErr
		}
		_ = os.Remove(session.ControlSocket)
	}()
	inbox, err := subagent.OpenInbox(filepath.Join(storeDir, session.ID+"-inbox"))
	if err != nil {
		return err
	}
	turnState := subagent.NewTurnState(nil)
	environ = setEnvironmentValue(environ, "HARNEZ_SESSION_ID", session.ID)
	environ = setEnvironmentValue(environ, "HARNEZ_AGY_METER_SESSION_ID", session.ID)
	observer := agyTurnObserver{state: turnState}
	opts := subagent.InteractiveOptions{Model: subagent.Model{Provider: "agy", Name: session.Model}, SessionID: session.ID, Name: session.Name, Dir: dir, Stdin: stdin, Stdout: stdout, Stderr: stderr, ControlSocket: session.ControlSocket, Inbox: inbox, TurnState: turnState, Started: func(pid int) error {
		session.ProcessPID = pid
		session.ProcessStarttime = procs.ProcessStarttime(pid)
		return store.Save(session)
	}}
	return subagent.RunSupervisedAgy(ctx, command, args, opts, environ, home, observer)
}

func agyMeterSessionName(dir string) (string, error) {
	if configured := strings.TrimSpace(os.Getenv("HARNEZ_AGY_NAME")); configured != "" {
		return configured, nil
	}
	var suffix [2]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", fmt.Errorf("generate AGY session name: %w", err)
	}
	base := filepath.Base(dir)
	if base == "." || base == string(filepath.Separator) || base == "" {
		base = "dir"
	}
	return "agy-" + base + "-" + hex.EncodeToString(suffix[:]), nil
}

func setEnvironmentValue(environ []string, key, value string) []string {
	for i, entry := range environ {
		if strings.HasPrefix(entry, key+"=") {
			environ[i] = key + "=" + value
			return environ
		}
	}
	return append(environ, key+"="+value)
}

type agyTurnObserver struct{ state *subagent.TurnState }

func (o agyTurnObserver) RequestStarted() { o.state.RequestStarted() }

func (o agyTurnObserver) ResponseFinished(functionCall bool) { o.state.ResponseFinished(functionCall) }
