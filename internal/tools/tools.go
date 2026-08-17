// Package tools plans and reports optional workstation capabilities.
package tools

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// Catalog describes the embedded tool specifications.
type Catalog struct {
	Tools []Tool
}

// Tool is declarative metadata for one capability.
type Tool struct {
	ID          string              `yaml:"id"`
	Title       string              `yaml:"title"`
	Description string              `yaml:"description"`
	Provider    Provider            `yaml:"provider"`
	Platforms   []Platform          `yaml:"platforms"`
	Artifacts   map[string]Artifact `yaml:"artifacts"`
	Probes      Probes              `yaml:"probes"`
}

type Provider struct {
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	Model        string `yaml:"model"`
	ModelSizeMiB int    `yaml:"model_size_mib"`
}

type Platform struct {
	OS             string `yaml:"os"`
	Arch           string `yaml:"arch"`
	Distro         string `yaml:"distro"`
	Version        string `yaml:"version"`
	Desktop        string `yaml:"desktop"`
	Session        string `yaml:"session"`
	CanaryVerified bool   `yaml:"canary_verified"`
	CanaryNote     string `yaml:"canary_note"`
}

type Artifact struct {
	Filename  string `yaml:"filename"`
	URL       string `yaml:"url"`
	SHA256    string `yaml:"sha256"`
	SizeBytes int64  `yaml:"size_bytes"`
}

type Probes struct {
	Commands    []string `yaml:"commands"`
	UserService string   `yaml:"user_service"`
}

// Host is the detected workstation environment.
type Host struct {
	OS, Arch, Distro, Version, Desktop, Session string
}

// Dependencies isolates host reads and subprocesses for tests.
type Dependencies struct {
	GOOS     string
	GOARCH   string
	Getenv   func(string) string
	ReadFile func(string) ([]byte, error)
	LookPath func(string) (string, error)
	Run      func(context.Context, string, ...string) error
	Stdin    io.Reader
	Stdout   io.Writer
}

// DefaultDependencies performs only read-only probes until an approved installer exists.
func DefaultDependencies(in io.Reader, out io.Writer) Dependencies {
	return Dependencies{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, Getenv: os.Getenv, ReadFile: os.ReadFile,
		LookPath: exec.LookPath, Run: func(ctx context.Context, name string, args ...string) error {
			return exec.CommandContext(ctx, name, args...).Run()
		}, Stdin: in, Stdout: out}
}

// LoadCatalog parses all embedded tool specs.
func LoadCatalog(fsys fs.FS) (Catalog, error) {
	entries, err := fs.ReadDir(fsys, "spec/tools")
	if err != nil {
		return Catalog{}, fmt.Errorf("read tool catalog: %w", err)
	}
	catalog := Catalog{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(fsys, "spec/tools/"+entry.Name())
		if err != nil {
			return Catalog{}, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		var tool Tool
		decoder := yaml.NewDecoder(strings.NewReader(string(data)))
		decoder.KnownFields(true)
		if err := decoder.Decode(&tool); err != nil {
			return Catalog{}, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		if err := validateTool(tool); err != nil {
			return Catalog{}, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		catalog.Tools = append(catalog.Tools, tool)
	}
	return catalog, nil
}

func validateTool(tool Tool) error {
	if tool.ID == "" || tool.Title == "" || tool.Description == "" || tool.Provider.Name == "" ||
		tool.Provider.Version == "" || tool.Provider.Model == "" || tool.Provider.ModelSizeMiB <= 0 || len(tool.Platforms) == 0 {
		return errors.New("missing required metadata")
	}
	for _, scope := range []string{"user", "system"} {
		artifact, ok := tool.Artifacts[scope]
		if !ok || artifact.Filename == "" || artifact.URL == "" || artifact.SizeBytes <= 0 {
			return fmt.Errorf("%s artifact is missing required metadata", scope)
		}
		if filepath.Base(artifact.Filename) != artifact.Filename {
			return fmt.Errorf("%s artifact has invalid filename", scope)
		}
		parsedURL, err := url.Parse(artifact.URL)
		if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
			return fmt.Errorf("%s artifact has invalid HTTPS URL", scope)
		}
		checksum, err := hex.DecodeString(artifact.SHA256)
		if err != nil || len(checksum) != sha256.Size {
			return fmt.Errorf("%s artifact has invalid SHA-256", scope)
		}
	}
	if len(tool.Probes.Commands) == 0 || tool.Probes.UserService == "" {
		return errors.New("missing readiness probes")
	}
	return nil
}

// DetectHost reads OS and session metadata without changing the host.
func DetectHost(d Dependencies) Host {
	h := Host{OS: d.GOOS, Arch: d.GOARCH, Desktop: d.Getenv("XDG_CURRENT_DESKTOP"), Session: d.Getenv("XDG_SESSION_TYPE")}
	if data, err := d.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			value = strings.Trim(value, `"`)
			switch key {
			case "ID":
				h.Distro = value
			case "VERSION_ID":
				h.Version = value
			}
		}
	}
	return h
}

func matchingPlatform(t Tool, h Host) *Platform {
	for i := range t.Platforms {
		p := &t.Platforms[i]
		if strings.EqualFold(p.OS, h.OS) && strings.EqualFold(p.Arch, h.Arch) && strings.EqualFold(p.Distro, h.Distro) &&
			p.Version == h.Version && strings.Contains(strings.ToLower(h.Desktop), strings.ToLower(p.Desktop)) && strings.EqualFold(p.Session, h.Session) {
			return p
		}
	}
	return nil
}

// State is a stable readiness state printed by status.
type State string

const (
	Ready       State = "ready"
	Missing     State = "missing"
	Partial     State = "partial"
	Unsupported State = "unsupported"
)

// Status contains the result and human-readable detail.
type Status struct {
	State  State
	Detail string
}

// Probe performs read-only readiness checks.
func Probe(ctx context.Context, t Tool, h Host, d Dependencies) Status {
	p := matchingPlatform(t, h)
	if p == nil {
		return Status{Unsupported, fmt.Sprintf("untested on %s %s/%s (%s, %s)", h.Distro, h.Version, h.Arch, h.Desktop, h.Session)}
	}
	found := 0
	for _, command := range t.Probes.Commands {
		if _, err := d.LookPath(command); err == nil {
			found++
		}
	}
	service := d.Run(ctx, "systemctl", "--user", "is-active", "--quiet", t.Probes.UserService) == nil
	if found == len(t.Probes.Commands) && service && p.CanaryVerified {
		return Status{Ready, "provider, injection backend, and user service are ready"}
	}
	if found == 0 && !service {
		if !p.CanaryVerified {
			return Status{Unsupported, p.CanaryNote}
		}
		return Status{Missing, "provider, injection backend, and user service are absent"}
	}
	detail := "some readiness probes passed"
	if !p.CanaryVerified {
		detail += "; " + p.CanaryNote
	}
	return Status{Partial, detail}
}

// InstallOptions controls an explicit install request.
type InstallOptions struct {
	Scope       string
	DryRun, Yes bool
}

// Install prints the exact safe plan. It fails closed while the hardware recipe is unverified.
func Install(ctx context.Context, t Tool, h Host, d Dependencies, options InstallOptions) error {
	p := matchingPlatform(t, h)
	if p == nil {
		return fmt.Errorf("voice-input is unsupported: untested host")
	}
	artifact, ok := t.Artifacts[options.Scope]
	if !ok {
		return fmt.Errorf("invalid scope %q", options.Scope)
	}
	privilege := "none"
	if options.Scope == "system" {
		privilege = "sudo for the isolated dnf transaction"
	}
	fmt.Fprintf(d.Stdout, "Voice input installation plan (%s scope):\n", options.Scope)
	fmt.Fprintf(d.Stdout, "  provider: %s %s (%s, %.1f MiB)\n", t.Provider.Name, t.Provider.Version, artifact.Filename, float64(artifact.SizeBytes)/(1024*1024))
	fmt.Fprintf(d.Stdout, "  model: %s (about %d MiB, local/offline)\n", t.Provider.Model, t.Provider.ModelSizeMiB)
	fmt.Fprintf(d.Stdout, "  destination: %s\n", map[bool]string{true: "system RPM via dnf", false: "~/.local/bin/voxtype"}[options.Scope == "system"])
	fmt.Fprintf(d.Stdout, "  privileges: %s\n", privilege)
	fmt.Fprintln(d.Stdout, "  persistent changes: model/config, GNOME shortcut, systemd user service")
	fmt.Fprintln(d.Stdout, "  keyboard injection: eitype (GNOME Wayland); no input-group membership")
	fmt.Fprintln(d.Stdout, "  privacy: microphone audio is transcribed locally and not intentionally retained")
	if options.DryRun {
		fmt.Fprintln(d.Stdout, "Dry run: no network, files, services, settings, or privileges were changed.")
		return nil
	}
	if !p.CanaryVerified {
		return fmt.Errorf("installation blocked: %s", p.CanaryNote)
	}
	if !options.Yes {
		fmt.Fprint(d.Stdout, "Continue? [y/N] ")
		line, _ := bufio.NewReader(d.Stdin).ReadString('\n')
		if strings.ToLower(strings.TrimSpace(line)) != "y" {
			return errors.New("installation declined")
		}
	}
	return errors.New("installation recipe is not available")
}

// VerifyAndInstall verifies content and installs it only when the destination is absent.
// The boolean reports whether a new file was written.
func VerifyAndInstall(data []byte, artifact Artifact, destination string) (bool, error) {
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), artifact.SHA256) {
		return false, errors.New("artifact checksum mismatch")
	}
	existing, err := os.ReadFile(destination)
	if err == nil {
		if bytes.Equal(existing, data) {
			return false, nil
		}
		return false, errors.New("destination exists and is not owned by harnez; refusing overwrite")
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect destination: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
		return false, fmt.Errorf("create destination: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(destination), ".harnez-voxtype-*")
	if err != nil {
		return false, fmt.Errorf("create temporary artifact: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Chmod(0755)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return false, fmt.Errorf("write artifact: %w", err)
	}
	if err := os.Rename(name, destination); err != nil {
		return false, fmt.Errorf("install artifact: %w", err)
	}
	return true, nil
}
