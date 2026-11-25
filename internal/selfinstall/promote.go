package selfinstall

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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
	ExpectedSHA256   string // optional 64-hex out-of-band binary digest pin (--sha256)
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
	contract, err := sessioncontract.Load()
	if err != nil {
		return "", "", fmt.Errorf("load session isolation contract: %w", err)
	}
	if home == "" {
		home, err = os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("resolve home directory: %w", err)
		}
	}
	home = strings.TrimSpace(home)
	if err := validateHostRootPath(home, "HOME"); err != nil {
		return "", "", err
	}

	binDir, err = resolveContractBinDir(contract.Promoter.BinaryRoots, home)
	if err != nil {
		return "", "", err
	}
	manDir, err = resolveContractManDir(contract.Promoter.ManRoots, home)
	if err != nil {
		return "", "", err
	}
	return binDir, manDir, nil
}

func resolveContractBinDir(roots []string, home string) (string, error) {
	for _, specRoot := range roots {
		switch specRoot {
		case "GOBIN":
			if gobin := strings.TrimSpace(os.Getenv("GOBIN")); gobin != "" {
				if err := validateHostRootPath(gobin, "GOBIN"); err != nil {
					return "", err
				}
				return gobin, nil
			}
		case "GOPATH_bin":
			if rawGP := strings.TrimSpace(os.Getenv("GOPATH")); rawGP != "" {
				parts := filepath.SplitList(rawGP)
				first := strings.TrimSpace(parts[0])
				if first == "" {
					return "", fmt.Errorf("GOPATH contains empty leading entry %q", rawGP)
				}
				if err := validateHostRootPath(first, "GOPATH"); err != nil {
					return "", err
				}
				return filepath.Join(first, "bin"), nil
			}
		case "HOME_go_bin":
			return filepath.Join(home, "go", "bin"), nil
		}
	}
	return "", fmt.Errorf("resolve promoter bin dir from contract: no matching root in %v", roots)
}

func resolveContractManDir(roots []string, home string) (string, error) {
	for _, specRoot := range roots {
		switch specRoot {
		case "XDG_DATA_HOME_man_man1":
			if xdgData := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdgData != "" {
				if err := validateHostRootPath(xdgData, "XDG_DATA_HOME"); err != nil {
					return "", err
				}
				return filepath.Join(xdgData, "man", "man1"), nil
			}
		case "HOME_local_share_man_man1":
			return filepath.Join(home, ".local", "share", "man", "man1"), nil
		}
	}
	return "", fmt.Errorf("resolve promoter man dir from contract: no matching root in %v", roots)
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
	if opts.ExpectedSHA256 != "" {
		raw, decErr := hex.DecodeString(strings.TrimSpace(opts.ExpectedSHA256))
		if decErr != nil || len(raw) != sha256.Size {
			return PromoteResult{}, fmt.Errorf("promote: --sha256 must be a 64-character hex digest")
		}
	}
	if err := ctx.Err(); err != nil {
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
		if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, "/\\") || !strings.HasSuffix(name, ".1") || strings.HasPrefix(name, ".") {
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

func verifyFileSHA256(f *os.File, wantDigest [32]byte, expectedHex string) error {
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
	if expectedHex != "" && !strings.EqualFold(hex.EncodeToString(got[:]), strings.TrimSpace(expectedHex)) {
		return fmt.Errorf("staged binary SHA-256 %x does not match expected %s", got, strings.TrimSpace(expectedHex))
	}
	return nil
}
