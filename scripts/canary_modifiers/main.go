package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Linux input subsystem constants from <linux/input.h> and <linux/input-event-codes.h>
const (
	evKey = 0x01

	keyLeftCtrl   = 29
	keyRightCtrl  = 97
	keyLeftShift  = 42
	keyRightShift = 54
	keyLeftAlt    = 56
	keyRightAlt   = 100
	keyLeftMeta   = 125 // Super / Windows key
	keyRightMeta  = 126
	keyCapsLock   = 58

	keyMax = 0x2ff // 767
)

type modifierKey struct {
	code int
	name string
}

var watchedModifiers = []modifierKey{
	{keyLeftCtrl, "LeftCtrl"},
	{keyRightCtrl, "RightCtrl"},
	{keyLeftAlt, "LeftAlt"},
	{keyRightAlt, "RightAlt"},
	{keyLeftMeta, "LeftSuper"},
	{keyRightMeta, "RightSuper"},
	{keyLeftShift, "LeftShift"},
	{keyRightShift, "RightShift"},
}

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

func eviocgbit(ev int, len int) uintptr {
	const iocRead = 2
	return uintptr((iocRead << 30) | (int('E') << 8) | (0x20 + ev) | (len << 16))
}

func eviocgkey(len int) uintptr {
	const iocRead = 2
	return uintptr((iocRead << 30) | (int('E') << 8) | 0x18 | (len << 16))
}

func eviocgname(len int) uintptr {
	const iocRead = 2
	return uintptr((iocRead << 30) | (int('E') << 8) | 0x06 | (len << 16))
}

type keyboardDevice struct {
	path string
	name string
	fd   int
}

func isBitSet(buf []byte, bit int) bool {
	byteIdx := bit / 8
	bitIdx := bit % 8
	if byteIdx >= len(buf) {
		return false
	}
	return (buf[byteIdx] & (1 << bitIdx)) != 0
}

func findKeyboardDevices() ([]*keyboardDevice, []string, error) {
	matches, err := filepath.Glob("/dev/input/event*")
	if err != nil {
		return nil, nil, err
	}

	var keyboards []*keyboardDevice
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
		if err := ioctl(fd, eviocgbit(evKey, len(keyBits)), unsafe.Pointer(&keyBits[0])); err != nil {
			_ = syscall.Close(fd)
			continue
		}

		if isBitSet(keyBits[:], keyLeftCtrl) && isBitSet(keyBits[:], keyLeftAlt) && isBitSet(keyBits[:], 30 /* KEY_A */) {
			var nameBuf [256]byte
			name := "Unknown Keyboard"
			if err := ioctl(fd, eviocgname(len(nameBuf)), unsafe.Pointer(&nameBuf[0])); err == nil {
				name = string(bytes.TrimRight(nameBuf[:], "\x00"))
			}
			keyboards = append(keyboards, &keyboardDevice{
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

func queryPressedModifiers(dev *keyboardDevice) []string {
	var keyState [(keyMax + 7) / 8]byte
	if err := ioctl(dev.fd, eviocgkey(len(keyState)), unsafe.Pointer(&keyState[0])); err != nil {
		return nil
	}

	var active []string
	for _, mod := range watchedModifiers {
		if isBitSet(keyState[:], mod.code) {
			active = append(active, mod.name)
		}
	}
	return active
}

func main() {
	fmt.Println("================================================================")
	fmt.Println("  CANARY: Physical Modifier Key Reader & Reactive Gate")
	fmt.Println("================================================================")

	keyboards, denied, err := findKeyboardDevices()
	if err != nil {
		fmt.Printf("ERROR: Failed to scan /dev/input: %v\n", err)
		os.Exit(1)
	}

	if len(keyboards) == 0 {
		if len(denied) > 0 {
			fmt.Printf("❌ Permission Denied on %d input event nodes.\n", len(denied))
			fmt.Println("   Direct evdev access requires read permissions on /dev/input/event*")
			fmt.Println("   (e.g., membership in the 'input' group or running with sudo).")
			fmt.Println()
			fmt.Println("   To test this canary right now, run:")
			fmt.Println("     sudo go run ./scripts/canary_modifiers")
			fmt.Println()
			fmt.Println("   Or add your user to the input group for persistent access:")
			fmt.Println("     sudo usermod -a -G input $USER")
		} else {
			fmt.Println("❌ No keyboard devices detected in /dev/input.")
		}
		os.Exit(1)
	}

	defer func() {
		for _, k := range keyboards {
			_ = syscall.Close(k.fd)
		}
	}()

	fmt.Printf("✓ Successfully opened %d physical keyboard device(s):\n", len(keyboards))
	for _, k := range keyboards {
		fmt.Printf("  • %s (%s)\n", k.path, k.name)
	}
	fmt.Println()
	fmt.Println("Hold or press Ctrl, Alt, Super, Shift to test real-time detection.")
	fmt.Println("When you RELEASE all modifiers, it will safely emit: \"Peace.\"")
	fmt.Println("Press Ctrl+C to exit.")
	fmt.Println("----------------------------------------------------------------")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	var lastActiveKeySummary string
	wasBlocked := false

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nCanary stopped.")
			return
		case <-ticker.C:
			var allActive []string
			for _, k := range keyboards {
				active := queryPressedModifiers(k)
				allActive = append(allActive, active...)
			}

			// Deduplicate active modifiers across multiple keyboards
			sort.Strings(allActive)
			uniqueActive := make([]string, 0, len(allActive))
			for i, mod := range allActive {
				if i == 0 || mod != allActive[i-1] {
					uniqueActive = append(uniqueActive, mod)
				}
			}

			summary := strings.Join(uniqueActive, "+")

			if len(uniqueActive) > 0 {
				wasBlocked = true
				if summary != lastActiveKeySummary {
					fmt.Printf("⛔ [BLOCKED] Modifiers active: [%s] -> Holding buffer...\n", summary)
					lastActiveKeySummary = summary
				}
			} else {
				if wasBlocked {
					fmt.Printf("✅ [SAFE] All modifiers released! Discharging buffer -> \"Peace.\"\n")
					wasBlocked = false
					lastActiveKeySummary = ""
				}
			}
		}
	}
}
