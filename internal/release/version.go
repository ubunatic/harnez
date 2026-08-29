package release

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// VersionSpec represents the version.yaml specification format.
type VersionSpec struct {
	Version    string   `yaml:"version,omitempty" json:"version,omitempty"`
	Major      *int     `yaml:"major,omitempty" json:"major,omitempty"`
	Minor      *int     `yaml:"minor,omitempty" json:"minor,omitempty"`
	Patch      *int     `yaml:"patch,omitempty" json:"patch,omitempty"`
	Prerelease string   `yaml:"prerelease,omitempty" json:"prerelease,omitempty"`
	Build      string   `yaml:"build,omitempty" json:"build,omitempty"`
	Files      []string `yaml:"files,omitempty" json:"files,omitempty"`
}

var semverRegex = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$`)

// Semver represents a parsed semantic version.
type Semver struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Build      string
}

// String returns the canonical semver string (without leading v).
func (s Semver) String() string {
	res := fmt.Sprintf("%d.%d.%d", s.Major, s.Minor, s.Patch)
	if s.Prerelease != "" {
		res += "-" + s.Prerelease
	}
	if s.Build != "" {
		res += "+" + s.Build
	}
	return res
}

// TagName returns the canonical git tag name (e.g. v0.1.0).
func (s Semver) TagName() string {
	return "v" + s.String()
}

// ParseSemver parses a semver string like "0.1.0" or "v1.2.3-beta.1+2026".
func ParseSemver(raw string) (Semver, error) {
	raw = strings.TrimSpace(raw)
	matches := semverRegex.FindStringSubmatch(raw)
	if matches == nil {
		return Semver{}, fmt.Errorf("invalid semver: %q", raw)
	}

	maj, _ := strconv.Atoi(matches[1])
	min, _ := strconv.Atoi(matches[2])
	pat, _ := strconv.Atoi(matches[3])

	return Semver{
		Major:      maj,
		Minor:      min,
		Patch:      pat,
		Prerelease: matches[4],
		Build:      matches[5],
	}, nil
}

// BumpVersion increments a semver by "patch", "minor", "major", or sets an explicit semver.
func BumpVersion(current string, bumpType string) (Semver, error) {
	bumpType = strings.TrimSpace(bumpType)
	if bumpType == "" {
		bumpType = "patch"
	}

	// Check if bumpType is an explicit semver string
	if semverRegex.MatchString(bumpType) {
		return ParseSemver(bumpType)
	}

	parsed, err := ParseSemver(current)
	if err != nil {
		return Semver{}, fmt.Errorf("parse current version %q: %w", current, err)
	}

	switch strings.ToLower(bumpType) {
	case "patch":
		parsed.Patch++
		parsed.Prerelease = ""
		parsed.Build = ""
	case "minor":
		parsed.Minor++
		parsed.Patch = 0
		parsed.Prerelease = ""
		parsed.Build = ""
	case "major":
		parsed.Major++
		parsed.Minor = 0
		parsed.Patch = 0
		parsed.Prerelease = ""
		parsed.Build = ""
	default:
		return Semver{}, fmt.Errorf("unknown bump type %q (use 'patch', 'minor', 'major', or explicit semver like '1.2.3')", bumpType)
	}

	return parsed, nil
}

// FindVersionSpecPath locates version.yaml or spec/version.yaml in dir.
func FindVersionSpecPath(dir string) string {
	candidates := []string{
		filepath.Join(dir, "version.yaml"),
		filepath.Join(dir, "version.yml"),
		filepath.Join(dir, "spec", "version.yaml"),
		filepath.Join(dir, "spec", "version.yml"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// LoadVersionSpec loads and parses version.yaml from dir.
func LoadVersionSpec(dir string) (*VersionSpec, string, error) {
	specPath := FindVersionSpecPath(dir)
	if specPath == "" {
		return nil, "", os.ErrNotExist
	}

	data, err := os.ReadFile(specPath)
	if err != nil {
		return nil, specPath, fmt.Errorf("read %s: %w", specPath, err)
	}

	var spec VersionSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, specPath, fmt.Errorf("parse %s: %w", specPath, err)
	}

	// Normalise version string if major/minor/patch integers were provided
	if spec.Version == "" && spec.Major != nil && spec.Minor != nil && spec.Patch != nil {
		sv := Semver{
			Major:      *spec.Major,
			Minor:      *spec.Minor,
			Patch:      *spec.Patch,
			Prerelease: spec.Prerelease,
			Build:      spec.Build,
		}
		spec.Version = sv.String()
	}

	return &spec, specPath, nil
}

// SaveVersionSpec writes version.yaml to disk.
func SaveVersionSpec(specPath string, spec *VersionSpec) error {
	dir := filepath.Dir(specPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(spec)
	if err != nil {
		return fmt.Errorf("marshal version spec: %w", err)
	}

	header := "# yaml-language-server: $schema=spec/schemas/version.schema.json\n"
	content := header + string(data)

	if err := os.WriteFile(specPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write %s: %w", specPath, err)
	}
	return nil
}
