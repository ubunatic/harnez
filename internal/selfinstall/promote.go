package selfinstall

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"ubunatic.com/harnez/internal/fsutil"
	"ubunatic.com/harnez/internal/sessioncontract"
)

// PromoteOptions configures one trusted host promotion from a privately staged build.
type PromoteOptions struct {
	StagedDir        string
	ExpectedManFiles []string
	Home             string
	NoAlias          bool
	NoRefresh        bool
	RefreshService   func(context.Context, string, io.Writer) error
}

// PromoteResult reports the promoted shared destinations.
type PromoteResult struct {
	BinaryPath string
	AliasPath  string
	ManDir     string
	ManFiles   []string
}

// ResolvePromoterRoots returns the host's trusted shared binary and man
// directories derived solely from host environment defaults and the ownership
// contract (never caller-supplied arbitrary paths).
func ResolvePromoterRoots(home string) (binDir, manDir string, err error) {
	if _, err := sessioncontract.Load(); err != nil {
		return "", "", fmt.Errorf("load session isolation contract: %w", err)
	}
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("resolve home directory: %w", err)
		}
	}
	if err := validateHostRootPath(home, "HOME"); err != nil {
		return "", "", err
	}

	if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
		if err := validateHostRootPath(gobin, "GOBIN"); err != nil {
			return "", "", err
		}
		binDir = gobin
	} else if gopath := strings.TrimSpace(os.Getenv("GOPATH")); gopath != "" {
		first := strings.TrimSpace(filepath.SplitList(gopath)[0])
		if err := validateHostRootPath(first, "GOPATH"); err != nil {
			return "", "", err
		}
		binDir = filepath.Join(first, "bin")
	} else {
		binDir = filepath.Join(home, "go", "bin")
	}

	if xdgData := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdgData != "" {
		if err := validateHostRootPath(xdgData, "XDG_DATA_HOME"); err != nil {
			return "", "", err
		}
		manDir = filepath.Join(xdgData, "man", "man1")
	} else {
		manDir = filepath.Join(home, ".local", "share", "man", "man1")
	}
	return binDir, manDir, nil
}

func validateHostRootPath(p, label string) error {
	if !filepath.IsAbs(p) || filepath.Clean(p) != p {
		return fmt.Errorf("%s must be an absolute clean path: %q", label, p)
	}
	for _, seg := range strings.Split(strings.TrimPrefix(p, string(filepath.Separator)), string(filepath.Separator)) {
		if seg == ".." || seg == "." || strings.ContainsRune(seg, '\x00') {
			return fmt.Errorf("%s contains unsafe path segment: %q", label, p)
		}
	}
	return nil
}

// PromoteStaged verifies a privately staged directory (harnez binary + exact man
// manifest) and atomically promotes it into trusted host destinations using
// O_DIRECTORY|O_NOFOLLOW-pinned directory descriptors and immutable temp copies.
func PromoteStaged(ctx context.Context, opts PromoteOptions, out io.Writer) (PromoteResult, error) {
	if os.Getenv("TAIA_SESSION_DIR") != "" {
		return PromoteResult{}, fmt.Errorf("promote staged artifacts: host-only operation in runner mode")
	}
	contract, err := sessioncontract.Load()
	if err != nil {
		return PromoteResult{}, fmt.Errorf("load session isolation contract: %w", err)
	}
	expectedMan, err := validateExpectedManManifest(opts.ExpectedManFiles)
	if err != nil {
		return PromoteResult{}, err
	}
	binDir, manDir, err := ResolvePromoterRoots(opts.Home)
	if err != nil {
		return PromoteResult{}, err
	}
	if err := validateHostRootPath(opts.StagedDir, "staged directory"); err != nil {
		return PromoteResult{}, err
	}
	res, err := promoteStagedOS(ctx, opts, contract, expectedMan, binDir, manDir)
	if err != nil {
		return PromoteResult{}, err
	}
	if out != nil {
		fmt.Fprintf(out, "promoted %s -> %s (%d man pages -> %s)\n",
			contract.Promoter.Binary, fsutil.ContractHome(res.BinaryPath), len(res.ManFiles), fsutil.ContractHome(res.ManDir))
	}
	if !opts.NoRefresh {
		refresh := opts.RefreshService
		if refresh == nil {
			refresh = refreshSystemdCollector
		}
		if err := refresh(ctx, res.BinaryPath, out); err != nil {
			return PromoteResult{}, err
		}
	}
	return res, nil
}

func validateExpectedManManifest(files []string) ([]string, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("promote: expected man manifest must be non-empty")
	}
	seen := make(map[string]bool, len(files))
	out := make([]string, 0, len(files))
	for _, name := range files {
		if name == "" || filepath.Base(name) != name || strings.Contains(name, "..") || !strings.HasSuffix(name, ".1") || strings.HasPrefix(name, ".") {
			return nil, fmt.Errorf("promote: invalid man manifest entry %q", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("promote: duplicate man manifest entry %q", name)
		}
		seen[name] = true
		out = append(out, name)
	}
	slices.Sort(out)
	return out, nil
}

func verifyFileSHA256(f *os.File, wantDigest [32]byte) error {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("empty promoted file")
	}
	var got [32]byte
	copy(got[:], h.Sum(nil))
	if got != wantDigest {
		return fmt.Errorf("immutable verification digest mismatch")
	}
	return nil
}
