//go:build linux

package selfinstall

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
	"ubunatic.com/harnez/internal/sessioncontract"
)

type stagedTempCopy struct {
	tmpName    string
	targetName string
}

type pinnedDir struct {
	fd int
}

func (d *pinnedDir) close() {
	if d != nil && d.fd >= 0 {
		_ = unix.Close(d.fd)
		d.fd = -1
	}
}

func openPinnedExistingDir(absPath string, wantPerm os.FileMode) (*pinnedDir, error) {
	if !filepath.IsAbs(absPath) || filepath.Clean(absPath) != absPath {
		return nil, fmt.Errorf("directory must be clean absolute path: %q", absPath)
	}
	fd, err := openDirChain(absPath, false)
	if err != nil {
		return nil, err
	}
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		_ = unix.Close(fd)
		return nil, err
	}
	if wantPerm != 0 && os.FileMode(st.Mode&0o777) != wantPerm {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("directory %s must have mode %04o (got %04o)", absPath, wantPerm, st.Mode&0o777)
	}
	return &pinnedDir{fd: fd}, nil
}

func ensurePinnedDir(absPath string) (*pinnedDir, error) {
	if !filepath.IsAbs(absPath) || filepath.Clean(absPath) != absPath {
		return nil, fmt.Errorf("destination directory must be clean absolute path: %q", absPath)
	}
	fd, err := openDirChain(absPath, true)
	if err != nil {
		return nil, err
	}
	return &pinnedDir{fd: fd}, nil
}

func openDirChain(absPath string, create bool) (int, error) {
	curFD, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, fmt.Errorf("open root directory: %w", err)
	}
	rel := strings.TrimPrefix(absPath, "/")
	if rel == "" {
		return curFD, nil
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			_ = unix.Close(curFD)
			return -1, fmt.Errorf("unsafe directory component %q in %s", seg, absPath)
		}
		nextFD, openErr := unix.Openat(curFD, seg, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if openErr != nil && create && errors.Is(openErr, syscall.ENOENT) {
			if mkErr := unix.Mkdirat(curFD, seg, 0o755); mkErr != nil && !errors.Is(mkErr, syscall.EEXIST) {
				_ = unix.Close(curFD)
				return -1, fmt.Errorf("create destination directory component %s in %s: %w", seg, absPath, mkErr)
			}
			nextFD, openErr = unix.Openat(curFD, seg, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		}
		_ = unix.Close(curFD)
		if openErr != nil {
			return -1, fmt.Errorf("pin directory component %s in %s (symlinks forbidden): %w", seg, absPath, openErr)
		}
		curFD = nextFD
	}
	return curFD, nil
}

func openChildPinnedDir(parent *pinnedDir, name string) (*pinnedDir, error) {
	fd, err := unix.Openat(parent.fd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open staged directory %s (must be real directory): %w", name, err)
	}
	return &pinnedDir{fd: fd}, nil
}

func listPinnedDirNames(dir *pinnedDir) ([]string, error) {
	dupFD, err := unix.FcntlInt(uintptr(dir.fd), unix.F_DUPFD_CLOEXEC, 3)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(dupFD), "pinned-dir")
	defer f.Close()
	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	slices.Sort(names)
	return names, nil
}

func assertExactEntries(dir *pinnedDir, label string, want []string) error {
	got, err := listPinnedDirNames(dir)
	if err != nil {
		return fmt.Errorf("read %s entries: %w", label, err)
	}
	sortedWant := append([]string(nil), want...)
	slices.Sort(sortedWant)
	if !slices.Equal(got, sortedWant) {
		return fmt.Errorf("%s entries %v do not match allowlist %v", label, got, sortedWant)
	}
	return nil
}

func openStagedPinnedDirs(stagedDir, binaryName string, expectedMan []string) (stagedRoot, stagedBin, stagedShare, stagedShareMan, stagedMan1 *pinnedDir, err error) {
	stagedRoot, err = openPinnedExistingDir(stagedDir, 0o700)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("validate staged root: %w", err)
	}
	closeOnErr := func() {
		stagedMan1.close()
		stagedShareMan.close()
		stagedShare.close()
		stagedBin.close()
		stagedRoot.close()
	}
	if err = assertExactEntries(stagedRoot, "staged root", []string{"bin", "share"}); err != nil {
		closeOnErr()
		return nil, nil, nil, nil, nil, err
	}
	stagedBin, err = openChildPinnedDir(stagedRoot, "bin")
	if err != nil {
		closeOnErr()
		return nil, nil, nil, nil, nil, err
	}
	if err = assertExactEntries(stagedBin, "staged bin", []string{binaryName}); err != nil {
		closeOnErr()
		return nil, nil, nil, nil, nil, err
	}
	stagedShare, err = openChildPinnedDir(stagedRoot, "share")
	if err != nil {
		closeOnErr()
		return nil, nil, nil, nil, nil, err
	}
	if err = assertExactEntries(stagedShare, "staged share", []string{"man"}); err != nil {
		closeOnErr()
		return nil, nil, nil, nil, nil, err
	}
	stagedShareMan, err = openChildPinnedDir(stagedShare, "man")
	if err != nil {
		closeOnErr()
		return nil, nil, nil, nil, nil, err
	}
	if err = assertExactEntries(stagedShareMan, "staged share/man", []string{"man1"}); err != nil {
		closeOnErr()
		return nil, nil, nil, nil, nil, err
	}
	stagedMan1, err = openChildPinnedDir(stagedShareMan, "man1")
	if err != nil {
		closeOnErr()
		return nil, nil, nil, nil, nil, err
	}
	if err = assertExactEntries(stagedMan1, "staged man1", expectedMan); err != nil {
		closeOnErr()
		return nil, nil, nil, nil, nil, err
	}
	return stagedRoot, stagedBin, stagedShare, stagedShareMan, stagedMan1, nil
}

func promoteStagedOS(_ context.Context, opts PromoteOptions, contract sessioncontract.Contract, expectedMan []string, binDir, manDir string) (PromoteResult, error) {
	stagedRoot, stagedBin, stagedShare, stagedShareMan, stagedMan1, err := openStagedPinnedDirs(opts.StagedDir, contract.Promoter.Binary, expectedMan)
	if err != nil {
		return PromoteResult{}, err
	}
	defer stagedMan1.close()
	defer stagedShareMan.close()
	defer stagedShare.close()
	defer stagedBin.close()
	defer stagedRoot.close()

	dstBin, err := ensurePinnedDir(binDir)
	if err != nil {
		return PromoteResult{}, err
	}
	defer dstBin.close()
	dstMan, err := ensurePinnedDir(manDir)
	if err != nil {
		return PromoteResult{}, err
	}
	defer dstMan.close()

	var manTemps []stagedTempCopy
	cleanup := func(temps []stagedTempCopy, dir *pinnedDir) {
		for _, tc := range temps {
			_ = unix.Unlinkat(dir.fd, tc.tmpName, 0)
		}
	}
	var binTemps []stagedTempCopy
	defer func() {
		cleanup(manTemps, dstMan)
		cleanup(binTemps, dstBin)
	}()

	for _, name := range expectedMan {
		tmpName, err := copyVerifiedFileAt(stagedMan1, name, dstMan, 0o644)
		if err != nil {
			return PromoteResult{}, fmt.Errorf("stage man page %s: %w", name, err)
		}
		manTemps = append(manTemps, stagedTempCopy{tmpName: tmpName, targetName: name})
	}

	tmpBin, err := copyVerifiedFileAt(stagedBin, contract.Promoter.Binary, dstBin, 0o755)
	if err != nil {
		return PromoteResult{}, fmt.Errorf("stage binary %s: %w", contract.Promoter.Binary, err)
	}
	binTemps = append(binTemps, stagedTempCopy{tmpName: tmpBin, targetName: contract.Promoter.Binary})

	var tmpSym string
	if !opts.NoAlias {
		symTmpName, err := createPinnedTempSymlink(dstBin, contract.Promoter.AliasTarget)
		if err != nil {
			return PromoteResult{}, err
		}
		tmpSym = symTmpName
		binTemps = append(binTemps, stagedTempCopy{tmpName: tmpSym, targetName: contract.Promoter.Alias})
	}

	for _, tc := range manTemps {
		if err := unix.Renameat(dstMan.fd, tc.tmpName, dstMan.fd, tc.targetName); err != nil {
			return PromoteResult{}, fmt.Errorf("promote man page %s: %w", tc.targetName, err)
		}
	}
	manTemps = nil

	if err := unix.Renameat(dstBin.fd, tmpBin, dstBin.fd, contract.Promoter.Binary); err != nil {
		return PromoteResult{}, fmt.Errorf("promote binary %s: %w", contract.Promoter.Binary, err)
	}
	binTemps = binTemps[1:]

	aliasPath := ""
	if !opts.NoAlias {
		if err := unix.Renameat(dstBin.fd, tmpSym, dstBin.fd, contract.Promoter.Alias); err != nil {
			return PromoteResult{}, fmt.Errorf("promote alias %s: %w", contract.Promoter.Alias, err)
		}
		binTemps = nil
		aliasPath = filepath.Join(binDir, contract.Promoter.Alias)
	}

	return PromoteResult{
		BinaryPath: filepath.Join(binDir, contract.Promoter.Binary),
		AliasPath:  aliasPath,
		ManDir:     manDir,
		ManFiles:   append([]string(nil), expectedMan...),
	}, nil
}

func copyVerifiedFileAt(srcDir *pinnedDir, srcName string, dstDir *pinnedDir, perm uint32) (string, error) {
	srcFD, err := unix.Openat(srcDir.fd, srcName, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	srcFile := os.NewFile(uintptr(srcFD), srcName)
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return "", fmt.Errorf("%s must be a non-empty regular file", srcName)
	}

	tmpName, dstFD, err := createPinnedTempFile(dstDir, srcName, perm)
	if err != nil {
		return "", err
	}
	dstFile := os.NewFile(uintptr(dstFD), tmpName)
	cleanupDst := true
	defer func() {
		_ = dstFile.Close()
		if cleanupDst {
			_ = unix.Unlinkat(dstDir.fd, tmpName, 0)
		}
	}()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(dstFile, h), srcFile); err != nil {
		return "", err
	}
	if err := unix.Fchmod(int(dstFile.Fd()), perm); err != nil {
		return "", err
	}
	var wantDigest [32]byte
	copy(wantDigest[:], h.Sum(nil))

	// Re-open the destination temp file via O_NOFOLLOW to verify written bytes match
	// what we copied and were not altered before atomic rename.
	verifyFD, err := unix.Openat(dstDir.fd, tmpName, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return "", err
	}
	verifyFile := os.NewFile(uintptr(verifyFD), tmpName)
	defer verifyFile.Close()
	if err := verifyFileSHA256(verifyFile, wantDigest); err != nil {
		return "", err
	}
	cleanupDst = false
	return tmpName, nil
}

func createPinnedTempFile(dir *pinnedDir, prefix string, perm uint32) (string, int, error) {
	pid := os.Getpid()
	for seq := 0; seq < 64; seq++ {
		name := fmt.Sprintf(".promote-%s-%d-%d.tmp", prefix, pid, seq)
		fd, err := unix.Openat(dir.fd, name, unix.O_RDWR|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, perm)
		if err == nil {
			return name, fd, nil
		}
		if !errors.Is(err, syscall.EEXIST) {
			return "", -1, err
		}
	}
	return "", -1, fmt.Errorf("exhausted temp filenames in pinned dir")
}

func createPinnedTempSymlink(dir *pinnedDir, target string) (string, error) {
	pid := os.Getpid()
	for seq := 0; seq < 64; seq++ {
		name := fmt.Sprintf(".promote-alias-%d-%d.tmp", pid, seq)
		err := unix.Symlinkat(target, dir.fd, name)
		if err == nil {
			return name, nil
		}
		if !errors.Is(err, syscall.EEXIST) {
			return "", err
		}
	}
	return "", fmt.Errorf("exhausted temp symlink names in pinned dir")
}
