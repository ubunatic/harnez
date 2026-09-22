package subagent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// InteractiveOptions configures a provider's foreground terminal session.
type InteractiveOptions struct {
	Model         Model
	SessionID     string
	Name          string
	Dir           string
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
	ControlSocket string
	Started       func(int) error
}

// InteractiveRunner launches or attaches to provider terminal sessions.
type InteractiveRunner interface {
	Chat(context.Context, InteractiveOptions) error
	Attach(context.Context, InteractiveOptions, string) error
}

// CLIInteractiveRunner runs the installed provider CLI with inherited streams.
type CLIInteractiveRunner struct{}

func (CLIInteractiveRunner) Chat(ctx context.Context, opts InteractiveOptions) error {
	command, args, err := interactiveCommand(opts, "")
	if err != nil {
		return err
	}
	return runInteractiveCommand(ctx, command, args, opts)
}

func (CLIInteractiveRunner) Attach(ctx context.Context, opts InteractiveOptions, providerID string) error {
	command, args, err := interactiveCommand(opts, providerID)
	if err != nil {
		return err
	}
	return runInteractiveCommand(ctx, command, args, opts)
}

func interactiveCommand(opts InteractiveOptions, providerID string) (string, []string, error) {
	var command string
	var args []string
	switch opts.Model.Provider {
	case "codex":
		command = "codex"
		if providerID == "" {
			args = []string{"-m", opts.Model.Name, "-C", opts.Dir}
		} else {
			args = []string{"resume", providerID, "-m", opts.Model.Name, "-C", opts.Dir}
		}
	case "claude":
		command = "claude"
		if providerID == "" {
			args = []string{"--model", opts.Model.Name, "--name", opts.Name, "--session-id", opts.SessionID}
		} else {
			args = []string{"--resume", providerID, "--model", opts.Model.Name}
		}
	case "agy":
		command = "agy"
		if providerID == "" {
			args = []string{"--model", opts.Model.Name}
		} else {
			args = []string{"--conversation", providerID, "--model", opts.Model.Name}
		}
		if opts.Model.SupportsEffort() && opts.Model.Tier != "" {
			args = append(args, "--effort", agyEffort(opts.Model.Tier))
		}
	default:
		return "", nil, fmt.Errorf("interactive chat is not supported for provider %q", opts.Model.Provider)
	}
	return command, args, nil
}

func runInteractiveCommand(ctx context.Context, command string, args []string, opts InteractiveOptions) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = opts.Dir
	listener, err := listenControl(opts.ControlSocket)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(opts.ControlSocket)
	}()
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return fmt.Errorf("%s interactive session: %w", command, err)
	}
	defer ptmx.Close()
	if opts.Started != nil {
		if err := opts.Started(cmd.Process.Pid); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}
	}
	var restore func()
	if input, ok := opts.Stdin.(*os.File); ok && term.IsTerminal(int(input.Fd())) {
		if state, rawErr := term.MakeRaw(int(input.Fd())); rawErr == nil {
			restore = func() { _ = term.Restore(int(input.Fd()), state) }
			defer restore()
			_ = pty.InheritSize(input, ptmx)
		}
	}
	go func() { _, _ = io.Copy(ptmx, opts.Stdin) }()
	var controls sync.WaitGroup
	controls.Add(1)
	go func() {
		defer controls.Done()
		serveControls(listener, ptmx, cmd.Process)
	}()
	_, _ = io.Copy(opts.Stdout, ptmx)
	_ = listener.Close()
	controls.Wait()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s interactive session: %w", command, err)
	}
	return nil
}

type controlRequest struct {
	Action string `json:"action"`
	Prompt string `json:"prompt,omitempty"`
}

type controlResponse struct {
	Error string `json:"error,omitempty"`
}

func listenControl(path string) (net.Listener, error) {
	if path == "" {
		return nil, fmt.Errorf("interactive control socket is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create control socket directory: %w", err)
	}
	_ = os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("listen on control socket: %w", err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("secure control socket: %w", err)
	}
	return listener, nil
}

func serveControls(listener net.Listener, terminal io.Writer, process *os.Process) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		var request controlRequest
		err = json.NewDecoder(bufio.NewReader(conn)).Decode(&request)
		if err == nil {
			switch request.Action {
			case "prompt":
				if terminal != nil {
					_, err = io.WriteString(terminal, request.Prompt+"\n")
				}
			case "compact":
				if terminal != nil {
					_, err = io.WriteString(terminal, "/compact\n")
				}
			case "stop":
				if process != nil {
					err = process.Signal(os.Interrupt)
				}
			default:
				err = fmt.Errorf("unknown control action %q", request.Action)
			}
		}
		response := controlResponse{}
		if err != nil {
			response.Error = err.Error()
		}
		_ = json.NewEncoder(conn).Encode(response)
		_ = conn.Close()
	}
}

// SendControl delivers an action to a live Harnez-owned interactive terminal.
func SendControl(ctx context.Context, socket, action, prompt string) error {
	var conn net.Conn
	var err error
	deadline := time.Now().Add(750 * time.Millisecond)
	for {
		dialer := net.Dialer{}
		conn, err = dialer.DialContext(ctx, "unix", socket)
		if err == nil || time.Now().After(deadline) || ctx.Err() != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("connect to interactive session: %w", err)
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(controlRequest{Action: action, Prompt: prompt}); err != nil {
		return fmt.Errorf("send interactive control: %w", err)
	}
	var response controlResponse
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return fmt.Errorf("read interactive control response: %w", err)
	}
	if response.Error != "" {
		return fmt.Errorf("interactive control: %s", response.Error)
	}
	return nil
}

var _ InteractiveRunner = CLIInteractiveRunner{}
