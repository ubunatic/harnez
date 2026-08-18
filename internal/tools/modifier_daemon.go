package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
	"unsafe"

	"github.com/spf13/cobra"
)

// DaemonOptions configures the modifier daemon.
type DaemonOptions struct {
	StateFile string
	Interval  time.Duration
}

// DefaultDaemonOptions returns standard daemon defaults.
func DefaultDaemonOptions() DaemonOptions {
	return DaemonOptions{
		StateFile: DefaultModifierStatePath,
		Interval:  10 * time.Millisecond,
	}
}

// NewModifierDaemonCommand returns the `harnez tools daemon modifier-service` command.
func NewModifierDaemonCommand(d Dependencies) *cobra.Command {
	opts := DefaultDaemonOptions()

	cmd := &cobra.Command{
		Use:   "modifier-service",
		Short: "Dedicated evdev modifier key state monitoring daemon",
		Long: "Monitors physical modifier keys (Ctrl, Alt, Super, Shift) across all /dev/input/event* keyboards,\n" +
			"and exports an atomic 1-byte bitmask state to a world-readable file (/run/harnez/modifiers).\n" +
			"Never captures or records non-modifier keycodes, preserving strict privacy.",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunModifierDaemon(cmd.Context(), d, opts)
		},
	}

	cmd.Flags().StringVar(&opts.StateFile, "state-file", opts.StateFile, "path to world-readable modifier state file (default: /run/harnez/modifiers)")
	cmd.Flags().DurationVar(&opts.Interval, "interval", opts.Interval, "polling and update interval (default: 10ms)")

	return cmd
}

// ioctl helper for evdev ioctls.
func evdevIoctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func evdevEviocgbit(ev int, length int) uintptr {
	const iocRead = 2
	return uintptr((iocRead << 30) | (int('E') << 8) | (0x20 + ev) | (length << 16))
}

func evdevEviocgkey(length int) uintptr {
	const iocRead = 2
	return uintptr((iocRead << 30) | (int('E') << 8) | 0x18 | (length << 16))
}

func evdevEviocgname(length int) uintptr {
	const iocRead = 2
	return uintptr((iocRead << 30) | (int('E') << 8) | 0x06 | (length << 16))
}

type evdevKeyboardDevice struct {
	path string
	name string
	fd   int
}

func isBitSetInSlice(buf []byte, bit int) bool {
	byteIdx := bit / 8
	bitIdx := bit % 8
	if byteIdx >= len(buf) {
		return false
	}
	return (buf[byteIdx] & (1 << bitIdx)) != 0
}

// FindPhysicalKeyboards scans /dev/input/event* and filters for physical keyboards via EVIOCGBIT.
func FindPhysicalKeyboards() ([]*evdevKeyboardDevice, []string, error) {
	matches, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, nil, err
	}

	var keyboards []*evdevKeyboardDevice
	var permissionDenied []string

	for _, path := range matches {
		fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			if os.IsPermission(err) || err == syscall.EACCES {
				permissionDenied = append(permissionDenied, path)
			}
			continue
		}

		var keyBits [(keyMax + 7) / 8]byte
		if err := evdevIoctl(fd, evdevEviocgbit(evKey, len(keyBits)), unsafe.Pointer(&keyBits[0])); err != nil {
			_ = syscall.Close(fd)
			continue
		}

		// Verify keyboard capability: must have LeftCtrl, LeftAlt, and KeyA (code 30)
		if isBitSetInSlice(keyBits[:], keyLeftCtrl) && isBitSetInSlice(keyBits[:], keyLeftAlt) && isBitSetInSlice(keyBits[:], 30 /* KEY_A */) {
			var nameBuf [256]byte
			name := "Keyboard"
			if err := evdevIoctl(fd, evdevEviocgname(len(nameBuf)), unsafe.Pointer(&nameBuf[0])); err == nil {
				name = string(bytes.TrimRight(nameBuf[:], "\x00"))
			}
			keyboards = append(keyboards, &evdevKeyboardDevice{
				path: path,
				name: name,
				fd:   fd,
			})
		} else {
			_ = syscall.Close(fd)
		}
	}

	sort.Slice(keyboards, func(i, j int) bool {
		return keyboards[i].path < keyboards[j].path
	})

	return keyboards, permissionDenied, nil
}

// QueryDeviceModifiers reads only modifier key state from an open keyboard device.
// Non-modifier keys are completely ignored, guaranteeing zero keylogging.
func QueryDeviceModifiers(fd int) ModifierMask {
	var keyState [(keyMax + 7) / 8]byte
	if err := evdevIoctl(fd, evdevEviocgkey(len(keyState)), unsafe.Pointer(&keyState[0])); err != nil {
		return 0
	}

	var mask ModifierMask
	for _, def := range WatchedModifierKeys {
		if isBitSetInSlice(keyState[:], def.Code) {
			mask |= def.Mask
		}
	}
	return mask
}

// WriteAtomicModifierState updates the state file atomically with world-readable (0644) permissions.
func WriteAtomicModifierState(path string, mask ModifierMask) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create state directory %s: %w", dir, err)
	}

	tmpFile := filepath.Join(dir, fmt.Sprintf(".modifiers-%d.tmp", os.Getpid()))
	data := []byte{byte(mask)}

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("write temp state: %w", err)
	}

	if err := os.Rename(tmpFile, path); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("rename state file: %w", err)
	}

	return nil
}

// RunModifierDaemon runs the modifier monitoring event loop.
func RunModifierDaemon(ctx context.Context, d Dependencies, opts DaemonOptions) error {
	keyboards, denied, err := FindPhysicalKeyboards()
	if err != nil {
		return fmt.Errorf("scan /dev/input: %w", err)
	}

	if len(keyboards) == 0 {
		if len(denied) > 0 {
			return fmt.Errorf("permission denied opening %d /dev/input devices (needs input group or root)", len(denied))
		}
		return fmt.Errorf("no physical keyboard devices found in /dev/input")
	}

	defer func() {
		for _, k := range keyboards {
			_ = syscall.Close(k.fd)
		}
	}()

	fmt.Fprintf(d.Stdout, "harnez-modifierd started. Monitoring %d keyboard device(s):\n", len(keyboards))
	for _, k := range keyboards {
		fmt.Fprintf(d.Stdout, "  • %s (%s)\n", k.path, k.name)
	}
	fmt.Fprintf(d.Stdout, "State output: %s\n", opts.StateFile)

	// Clean up state file on exit
	defer func() {
		_ = WriteAtomicModifierState(opts.StateFile, 0)
	}()

	ticker := time.NewTicker(opts.Interval)
	defer ticker.Stop()

	var lastMask ModifierMask = 0xFF // Force initial write

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(d.Stdout, "harnez-modifierd stopping...")
			return nil
		case <-ticker.C:
			var currentMask ModifierMask
			for _, k := range keyboards {
				currentMask |= QueryDeviceModifiers(k.fd)
			}

			if currentMask != lastMask {
				if err := WriteAtomicModifierState(opts.StateFile, currentMask); err != nil {
					fmt.Fprintf(d.Stdout, "Error writing state: %v\n", err)
				}
				lastMask = currentMask
			}
		}
	}
}
