//go:build linux

package exec

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"runtime"
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	landlockWriteDirsEnv = "HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS"
	brokerFDEnv          = "HARNEZ_BROKER_FD"
)

var landlockOptionalWritableDirs = []string{
	"/dev/shm",
	"/dev/pts",
}

var landlockSafeDevices = []string{
	"/dev/null",
	"/dev/zero",
	"/dev/full",
	"/dev/random",
	"/dev/urandom",
	"/dev/tty",
	"/dev/ptmx",
}

// probeLandlockABI queries the running Linux kernel's Landlock ABI version.
// It requires ABI >= 1 and returns a clear error when Landlock is unsupported.
func probeLandlockABI() (int, error) {
	abi, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		0,
		0,
		unix.LANDLOCK_CREATE_RULESET_VERSION,
	)
	if errno != 0 {
		if errno == unix.ENOSYS || errno == unix.EOPNOTSUPP {
			return 0, fmt.Errorf("landlock unsupported by kernel: %w", errno)
		}
		return 0, fmt.Errorf("landlock ABI probe failed: %w", errno)
	}
	if int(abi) < 1 {
		return 0, fmt.Errorf("landlock ABI version %d < 1", int(abi))
	}
	return int(abi), nil
}

// applyLandlockRuleset restricts the calling thread's filesystem access to
// read+execute under "/" plus full read/write/mutate access under writableDirs
// and safe device nodes.
func applyLandlockRuleset(writableDirs []string) error {
	abi, err := probeLandlockABI()
	if err != nil {
		return err
	}

	const readExecAccess uint64 = unix.LANDLOCK_ACCESS_FS_EXECUTE |
		unix.LANDLOCK_ACCESS_FS_READ_FILE |
		unix.LANDLOCK_ACCESS_FS_READ_DIR

	const writeMutateAccess uint64 = unix.LANDLOCK_ACCESS_FS_WRITE_FILE |
		unix.LANDLOCK_ACCESS_FS_REMOVE_DIR |
		unix.LANDLOCK_ACCESS_FS_REMOVE_FILE |
		unix.LANDLOCK_ACCESS_FS_MAKE_CHAR |
		unix.LANDLOCK_ACCESS_FS_MAKE_DIR |
		unix.LANDLOCK_ACCESS_FS_MAKE_REG |
		unix.LANDLOCK_ACCESS_FS_MAKE_SOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_FIFO |
		unix.LANDLOCK_ACCESS_FS_MAKE_BLOCK |
		unix.LANDLOCK_ACCESS_FS_MAKE_SYM

	handledAccessFS := readExecAccess | writeMutateAccess
	if abi >= 2 {
		handledAccessFS |= unix.LANDLOCK_ACCESS_FS_REFER
	}
	if abi >= 3 {
		handledAccessFS |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}

	attr := unix.LandlockRulesetAttr{
		Access_fs: handledAccessFS,
	}
	rulesetFD, _, errno := unix.Syscall(
		unix.SYS_LANDLOCK_CREATE_RULESET,
		uintptr(unsafe.Pointer(&attr)),
		unsafe.Sizeof(attr),
		0,
	)
	if errno != 0 {
		return fmt.Errorf("landlock_create_ruleset: %w", errno)
	}
	defer unix.Close(int(rulesetFD))

	if err := addLandlockPathRule(int(rulesetFD), "/", readExecAccess); err != nil {
		return fmt.Errorf("grant read-only root /: %w", err)
	}

	for _, dir := range writableDirs {
		if dir == "" {
			return errors.New("writable dir path must not be empty")
		}
		if err := addLandlockPathRule(int(rulesetFD), dir, handledAccessFS); err != nil {
			return fmt.Errorf("grant writable dir %q: %w", dir, err)
		}
	}

	for _, dir := range landlockOptionalWritableDirs {
		if err := addLandlockPathRule(int(rulesetFD), dir, handledAccessFS); err != nil {
			if errors.Is(err, unix.ENOENT) || errors.Is(err, unix.ENOTDIR) {
				continue
			}
			return fmt.Errorf("grant optional writable dir %q: %w", dir, err)
		}
	}

	devAccess := uint64(unix.LANDLOCK_ACCESS_FS_READ_FILE | unix.LANDLOCK_ACCESS_FS_WRITE_FILE)
	if abi >= 3 {
		devAccess |= unix.LANDLOCK_ACCESS_FS_TRUNCATE
	}
	for _, dev := range landlockSafeDevices {
		if err := addLandlockPathRule(int(rulesetFD), dev, devAccess); err != nil {
			if errors.Is(err, unix.ENOENT) {
				continue
			}
			return fmt.Errorf("grant device %q: %w", dev, err)
		}
	}

	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS): %w", err)
	}
	if _, _, errno := unix.Syscall(unix.SYS_LANDLOCK_RESTRICT_SELF, rulesetFD, 0, 0); errno != 0 {
		return fmt.Errorf("landlock_restrict_self: %w", errno)
	}
	return nil
}

func addLandlockPathRule(rulesetFD int, path string, allowed uint64) error {
	fd, err := unix.Open(path, unix.O_PATH|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}
	defer unix.Close(fd)

	pathAttr := unix.LandlockPathBeneathAttr{
		Allowed_access: allowed,
		Parent_fd:      int32(fd),
	}
	_, _, errno := unix.Syscall6(
		unix.SYS_LANDLOCK_ADD_RULE,
		uintptr(rulesetFD),
		unix.LANDLOCK_RULE_PATH_BENEATH,
		uintptr(unsafe.Pointer(&pathAttr)),
		0,
		0,
		0,
	)
	if errno != 0 {
		return fmt.Errorf("landlock_add_rule: %w", errno)
	}
	return nil
}

// MaybeRunSandboxHelper executes the internal Landlock confinement helper when
// invoked as `<self> __landlock-exec -- <cmd> [args...]` with
// HARNEZ_INTERNAL_LANDLOCK_WRITE_DIRS set. It must be called at the very top
// of main() and TestMain() before other initialization.
func MaybeRunSandboxHelper() {
	if len(os.Args) < 4 || os.Args[1] != "__landlock-exec" || os.Args[2] != "--" {
		return
	}
	rawDirs := os.Getenv(landlockWriteDirsEnv)
	if rawDirs == "" {
		return
	}
	_ = os.Unsetenv(landlockWriteDirsEnv)

	var writeDirs []string
	if err := json.Unmarshal([]byte(rawDirs), &writeDirs); err != nil {
		fmt.Fprintf(os.Stderr, "harnez: invalid %s: %v\n", landlockWriteDirsEnv, err)
		os.Exit(126)
	}
	if len(writeDirs) == 0 {
		fmt.Fprintf(os.Stderr, "harnez: empty %s\n", landlockWriteDirsEnv)
		os.Exit(126)
	}

	// LockOSThread ensures PR_SET_NO_NEW_PRIVS, landlock_restrict_self, and
	// unix.Exec execute on the same OS thread.
	runtime.LockOSThread()

	if err := applyLandlockRuleset(writeDirs); err != nil {
		fmt.Fprintf(os.Stderr, "harnez: landlock confinement failed: %v\n", err)
		os.Exit(126)
	}

	if rawFD := os.Getenv(brokerFDEnv); rawFD != "" {
		if fd, err := strconv.Atoi(rawFD); err == nil && fd >= 3 {
			if flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFD, 0); err == nil {
				if flags&unix.FD_CLOEXEC != 0 {
					if _, err := unix.FcntlInt(uintptr(fd), unix.F_SETFD, flags&^unix.FD_CLOEXEC); err != nil {
						fmt.Fprintf(os.Stderr, "harnez: clear FD_CLOEXEC on %s=%d: %v\n", brokerFDEnv, fd, err)
						os.Exit(126)
					}
				}
			}
		}
	}

	targetPath, err := osexec.LookPath(os.Args[3])
	if err != nil {
		fmt.Fprintf(os.Stderr, "harnez: resolve command %q: %v\n", os.Args[3], err)
		os.Exit(127)
	}
	if err := unix.Exec(targetPath, os.Args[3:], os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "harnez: exec %q: %v\n", targetPath, err)
		os.Exit(126)
	}
}
