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
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
	"ubunatic.com/harnez/internal/agymeter"
	"ubunatic.com/harnez/internal/claude"
)

// InteractiveOptions configures a provider's foreground terminal session.
type InteractiveOptions struct {
	Model           Model
	Prompt          string
	SessionID       string
	Name            string
	Dir             string
	Stdin           io.Reader
	Stdout          io.Writer
	Stderr          io.Writer
	ControlSocket   string
	Inbox           *Inbox
	TurnState       *TurnState
	QuietWindow     time.Duration
	TypingGuard     time.Duration
	Started         func(int) error
	ProviderIDFound func(string) error
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
			if opts.Prompt != "" {
				args = append(args, opts.Prompt)
			}
		} else {
			args = []string{"resume", providerID, "-m", opts.Model.Name, "-C", opts.Dir}
			if opts.Prompt != "" {
				args = append(args, opts.Prompt)
			}
		}
	case "claude":
		command = "claude"
		if providerID == "" {
			args = []string{"--model", opts.Model.Name, "--name", opts.Name, "--session-id", opts.SessionID}
			if opts.Prompt != "" {
				args = append(args, "--", opts.Prompt)
			}
		} else {
			args = []string{"--resume", providerID, "--model", opts.Model.Name}
			if opts.Prompt != "" {
				args = append(args, "--", opts.Prompt)
			}
		}
	case "agy":
		command = "agy"
		if providerID == "" {
			args = []string{"--model", opts.Model.Name}
			if opts.Prompt != "" {
				args = append(args, "--prompt-interactive", opts.Prompt)
			}
		} else {
			args = []string{"--conversation", providerID, "--model", opts.Model.Name}
			if opts.Prompt != "" {
				args = append(args, "--prompt-interactive", opts.Prompt)
			}
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
	environ := os.Environ()
	var home string
	if command == "agy" || command == "codex" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("%s: resolve home directory: %w", command, err)
		}
	}
	if command == "agy" {
		var err error
		environ, err = agyInteractiveLaunchEnv(environ, home)
		if err != nil {
			return err
		}
		environ = replaceEnvironmentValue(environ, "HARNEZ_SESSION_ID", opts.SessionID)
		environ = replaceEnvironmentValue(environ, "HARNEZ_AGY_METER_SESSION_ID", opts.SessionID)
	}
	runPTY := func(env []string) error {
		return runInteractivePTY(ctx, command, args, opts, env, home)
	}
	if command == "agy" {
		return agymeter.RunWithEnvDirRunner(ctx, home, command, args, environ, opts.Dir, opts.Stderr, runPTY)
	}
	return runPTY(nil)
}

func runInteractivePTY(ctx context.Context, command string, args []string, opts InteractiveOptions, environ []string, home string) error {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = opts.Dir
	if environ != nil {
		cmd.Env = environ
	}
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
	watchCtx, stopWatch := context.WithCancel(ctx)
	watchDone := make(chan struct{})
	if opts.ProviderIDFound != nil && (command == "codex" || command == "agy") {
		started := time.Now()
		go func() {
			defer close(watchDone)
			watchInteractiveProviderID(watchCtx, command, home, cmd.Process.Pid, opts.Dir, started, opts.ProviderIDFound)
		}()
	} else {
		close(watchDone)
	}
	defer func() {
		stopWatch()
		<-watchDone
	}()
	turnState := opts.TurnState
	if turnState == nil && opts.Inbox != nil {
		turnState = NewTurnState(nil)
	}
	terminalMu := &sync.Mutex{}
	var restore func()
	if input, ok := opts.Stdin.(*os.File); ok && term.IsTerminal(int(input.Fd())) {
		if state, rawErr := term.MakeRaw(int(input.Fd())); rawErr == nil {
			restore = func() { _ = term.Restore(int(input.Fd()), state) }
			defer restore()
			_ = pty.InheritSize(input, ptmx)
		}
	}
	go func() {
		reader := opts.Stdin
		if turnState != nil {
			reader = keystrokeReader{reader: opts.Stdin, state: turnState}
		}
		_, _ = io.Copy(ptmx, reader)
	}()
	var controls sync.WaitGroup
	controls.Add(1)
	go func() {
		defer controls.Done()
		serveControls(listener, ptmx, cmd.Process, opts.Inbox, turnState, terminalMu)
	}()
	deliveryCtx, stopDelivery := context.WithCancel(ctx)
	var deliveryDone chan struct{}
	if opts.Inbox != nil && turnState != nil {
		deliveryDone = make(chan struct{})
		go func() {
			defer close(deliveryDone)
			deliverInbox(deliveryCtx, opts.Inbox, turnState, ptmx, terminalMu, opts.QuietWindow, opts.TypingGuard)
		}()
	}
	_, _ = io.Copy(opts.Stdout, ptmx)
	stopDelivery()
	if deliveryDone != nil {
		<-deliveryDone
	}
	_ = listener.Close()
	controls.Wait()
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s interactive session: %w", command, err)
	}
	return nil
}

func watchInteractiveProviderID(ctx context.Context, command, home string, pid int, dir string, started time.Time, found func(string) error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var id string
		switch command {
		case "codex":
			id = findCodexInteractiveSessionID(home, pid, dir, started)
		case "agy":
			id = findAgyInteractiveSessionID(home, "/proc", pid)
		}
		if id != "" {
			if err := found(id); err == nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func agyInteractiveLaunchEnv(environ []string, home string) ([]string, error) {
	if _, _, err := claude.EnsureBashShim(home); err != nil {
		return nil, fmt.Errorf("agy: ensure bash shim: %w", err)
	}
	return AgyLaunchEnv(environ, home), nil
}

// ControlRequest is one request sent over an interactive session control socket.
type ControlRequest struct {
	Action string `json:"action"`
	Prompt string `json:"prompt,omitempty"`
	Sender string `json:"sender,omitempty"`
	Event  string `json:"event,omitempty"`
}

// ControlResponse is the result returned by an interactive session broker.
type ControlResponse struct {
	Error     string         `json:"error,omitempty"`
	MessageID string         `json:"message_id,omitempty"`
	Queued    int            `json:"queued,omitempty"`
	Messages  []InboxMessage `json:"messages,omitempty"`
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

func serveControls(listener net.Listener, terminal io.Writer, process *os.Process, inbox *Inbox, turnState *TurnState, terminalMu *sync.Mutex) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		var request ControlRequest
		err = json.NewDecoder(bufio.NewReader(conn)).Decode(&request)
		response := ControlResponse{}
		if err == nil {
			switch request.Action {
			case "prompt":
				if terminal != nil {
					err = writeTerminal(terminal, terminalMu, request.Prompt+"\n")
				}
			case "compact":
				if terminal != nil {
					err = writeTerminal(terminal, terminalMu, "/compact\n")
				}
			case "enqueue":
				if inbox == nil {
					err = fmt.Errorf("interactive inbox is unavailable")
					break
				}
				var msg *InboxMessage
				msg, err = inbox.Enqueue(request.Sender, request.Prompt)
				if err == nil {
					response.MessageID = msg.ID
					response.Queued = inbox.Depth()
				}
			case "inbox":
				if inbox == nil {
					err = fmt.Errorf("interactive inbox is unavailable")
					break
				}
				response.Messages = inbox.Messages()
			case "turn":
				if turnState == nil {
					err = fmt.Errorf("interactive turn state is unavailable")
					break
				}
				switch request.Event {
				case "tool-start":
					turnState.ToolStarted()
				case "tool-end":
					turnState.ToolFinished()
				default:
					err = fmt.Errorf("unknown turn event %q", request.Event)
				}
			case "stop":
				if process != nil {
					err = process.Signal(syscall.SIGTERM)
				}
			default:
				err = fmt.Errorf("unknown control action %q", request.Action)
			}
		}
		if err != nil {
			response.Error = err.Error()
		}
		_ = json.NewEncoder(conn).Encode(response)
		_ = conn.Close()
	}
}

// SendControl delivers an action to a live Harnez-owned interactive terminal.
func SendControl(ctx context.Context, socket, action, prompt string) error {
	_, err := SendControlRequest(ctx, socket, ControlRequest{Action: action, Prompt: prompt})
	return err
}

// SendControlRequest sends a JSON request to a live interactive terminal.
func SendControlRequest(ctx context.Context, socket string, request ControlRequest) (ControlResponse, error) {
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
		return ControlResponse{}, ctx.Err()
	}
	if err != nil {
		return ControlResponse{}, fmt.Errorf("connect to interactive session: %w", err)
	}
	defer conn.Close()
	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return ControlResponse{}, fmt.Errorf("send interactive control: %w", err)
	}
	var response ControlResponse
	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		return ControlResponse{}, fmt.Errorf("read interactive control response: %w", err)
	}
	if response.Error != "" {
		return response, fmt.Errorf("interactive control: %s", response.Error)
	}
	return response, nil
}

type keystrokeReader struct {
	reader io.Reader
	state  *TurnState
}

func (reader keystrokeReader) Read(data []byte) (int, error) {
	n, err := reader.reader.Read(data)
	if n > 0 {
		reader.state.Keystroke()
	}
	return n, err
}

func writeTerminal(terminal io.Writer, mu *sync.Mutex, text string) error {
	if mu != nil {
		mu.Lock()
		defer mu.Unlock()
	}
	_, err := io.WriteString(terminal, text)
	return err
}

func deliverInbox(ctx context.Context, inbox *Inbox, state *TurnState, terminal io.Writer, terminalMu *sync.Mutex, quiet, typingGuard time.Duration) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-inbox.wake:
		case <-ticker.C:
		}
		for {
			if ctx.Err() != nil || !state.Idle(time.Now(), quiet, typingGuard) {
				break
			}
			msg := inbox.Next()
			if msg == nil {
				break
			}
			if err := writeTerminal(terminal, terminalMu, "\x1b[200~"+msg.Text+"\x1b[201~\r"); err != nil {
				if markErr := inbox.Mark(msg.ID, "failed", err.Error()); markErr != nil {
					_ = inbox.Mark(msg.ID, "queued", markErr.Error())
				}
				continue
			}
			if err := inbox.Mark(msg.ID, "delivered", ""); err != nil {
				continue
			}
		}
	}
}

var _ InteractiveRunner = CLIInteractiveRunner{}
