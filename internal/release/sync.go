package release

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// SyncResult details the outcome of language version file updates.
type SyncResult struct {
	UpdatedFiles []string
	CreatedFiles []string
}

var (
	goVersionVarRegex   = regexp.MustCompile(`(?m)^(var|const)\s+Version\s*=\s*"[^"]*"`)
	pyVersionVarRegex   = regexp.MustCompile(`(?m)^(__version__|VERSION|version)\s*=\s*["'][^"']+["']`)
	pyprojectVerRegex   = regexp.MustCompile(`(?m)^(version\s*=\s*)["'][^"']+["']`)
	zigZonVerRegex      = regexp.MustCompile(`(?m)(\.version\s*=\s*)["'][^"']+["']`)
	zigVersionVarRegex  = regexp.MustCompile(`(?m)^(pub\s+const\s+version\s*=\s*)["'][^"']+["']`)
	cargoVersionRegex   = regexp.MustCompile(`(?m)(^\[package\][\s\S]*?^version\s*=\s*)["'][^"']+["']`)
	jsonVersionRegex    = regexp.MustCompile(`(?m)("version"\s*:\s*)"([^"]*)"`)
)

// AutoDetectCurrentVersion inspects the repo files or git tags to find the latest version.
func AutoDetectCurrentVersion(dir string) (string, error) {
	// 1. Check version.yaml
	if spec, _, err := LoadVersionSpec(dir); err == nil && spec.Version != "" {
		return spec.Version, nil
	}

	// 2. Check Go version.go
	if v, ok := readGoVersion(filepath.Join(dir, "version.go")); ok {
		return v, nil
	}

	// 3. Check Zig build.zig.zon or version.zig
	if v, ok := readZigZonVersion(filepath.Join(dir, "build.zig.zon")); ok {
		return v, nil
	}
	if v, ok := readZigVersion(filepath.Join(dir, "src", "version.zig")); ok {
		return v, nil
	}
	if v, ok := readZigVersion(filepath.Join(dir, "version.zig")); ok {
		return v, nil
	}

	// 4. Check Python version files
	if v, ok := readPyVersion(filepath.Join(dir, "__version__.py")); ok {
		return v, nil
	}
	if v, ok := readPyVersion(filepath.Join(dir, "version.py")); ok {
		return v, nil
	}
	if v, ok := readPyprojectVersion(filepath.Join(dir, "pyproject.toml")); ok {
		return v, nil
	}

	// 5. Check Cargo.toml
	if v, ok := readCargoVersion(filepath.Join(dir, "Cargo.toml")); ok {
		return v, nil
	}

	// 6. Check WebExtension manifest.json
	if v, ok := readManifestVersion(filepath.Join(dir, "manifest.json")); ok {
		return v, nil
	}

	// 7. Check package.json
	if v, ok := readPackageJSONVersion(filepath.Join(dir, "package.json")); ok {
		return v, nil
	}

	// 8. Check git tags
	cmd := exec.Command("git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		tag := strings.TrimSpace(string(out))
		if tag != "" {
			if parsed, err := ParseSemver(tag); err == nil {
				return parsed.String(), nil
			}
		}
	}

	// Default fallback
	return "0.1.0", nil
}

// SyncLanguageFiles propagates the version to language-native source files.
func SyncLanguageFiles(dir string, version string, explicitFiles []string) (*SyncResult, error) {
	res := &SyncResult{}

	// If explicit files are provided in spec, update those
	if len(explicitFiles) > 0 {
		for _, f := range explicitFiles {
			p := filepath.Join(dir, f)
			if err := syncSingleFile(p, version, res); err != nil {
				return nil, err
			}
		}
		return res, nil
	}

	// Auto-detect and sync Go
	goVersionPath := filepath.Join(dir, "version.go")
	if fileExists(goVersionPath) {
		if err := updateGoVersion(goVersionPath, version, res); err != nil {
			return nil, err
		}
	} else if fileExists(filepath.Join(dir, "go.mod")) {
		// Create standard version.go
		pkgName := detectGoPackage(dir)
		content := fmt.Sprintf("package %s\n\nvar Version = %q\n", pkgName, version)
		if err := os.WriteFile(goVersionPath, []byte(content), 0644); err != nil {
			return nil, fmt.Errorf("create %s: %w", goVersionPath, err)
		}
		res.CreatedFiles = append(res.CreatedFiles, "version.go")
	}

	// Auto-detect and sync Zig
	zigZonPath := filepath.Join(dir, "build.zig.zon")
	if fileExists(zigZonPath) {
		if err := updateZigZonVersion(zigZonPath, version, res); err != nil {
			return nil, err
		}
	}
	for _, zigVerPath := range []string{filepath.Join(dir, "version.zig"), filepath.Join(dir, "src", "version.zig")} {
		if fileExists(zigVerPath) {
			if err := updateZigVersion(zigVerPath, version, res); err != nil {
				return nil, err
			}
		}
	}

	// Auto-detect and sync Python
	for _, pyVerPath := range []string{
		filepath.Join(dir, "__version__.py"),
		filepath.Join(dir, "version.py"),
		filepath.Join(dir, filepath.Base(dir), "__version__.py"),
		filepath.Join(dir, filepath.Base(dir), "version.py"),
	} {
		if fileExists(pyVerPath) {
			if err := updatePyVersion(pyVerPath, version, res); err != nil {
				return nil, err
			}
		}
	}
	pyprojectPath := filepath.Join(dir, "pyproject.toml")
	if fileExists(pyprojectPath) {
		if err := updatePyprojectVersion(pyprojectPath, version, res); err != nil {
			return nil, err
		}
	}

	// Auto-detect and sync Rust
	cargoPath := filepath.Join(dir, "Cargo.toml")
	if fileExists(cargoPath) {
		if err := updateCargoVersion(cargoPath, version, res); err != nil {
			return nil, err
		}
	}

	// Auto-detect and sync WebExtension manifest.json
	manifestPath := filepath.Join(dir, "manifest.json")
	if fileExists(manifestPath) {
		if err := updateManifestVersion(manifestPath, version, res); err != nil {
			return nil, err
		}
	}

	// Auto-detect and sync package.json
	pkgJsonPath := filepath.Join(dir, "package.json")
	if fileExists(pkgJsonPath) {
		if err := updatePackageJSONVersion(pkgJsonPath, version, res); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func syncSingleFile(path string, version string, res *SyncResult) error {
	base := filepath.Base(path)
	ext := filepath.Ext(path)
	switch {
	case base == "version.go" || ext == ".go":
		return updateGoVersion(path, version, res)
	case base == "build.zig.zon":
		return updateZigZonVersion(path, version, res)
	case ext == ".zig":
		return updateZigVersion(path, version, res)
	case base == "pyproject.toml":
		return updatePyprojectVersion(path, version, res)
	case ext == ".py":
		return updatePyVersion(path, version, res)
	case base == "Cargo.toml":
		return updateCargoVersion(path, version, res)
	case base == "manifest.json":
		return updateManifestVersion(path, version, res)
	case base == "package.json":
		return updatePackageJSONVersion(path, version, res)
	}
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func detectGoPackage(dir string) string {
	// Look at any existing .go file in dir
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
				if data, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
					lines := strings.Split(string(data), "\n")
					for _, l := range lines {
						l = strings.TrimSpace(l)
						if strings.HasPrefix(l, "package ") {
							return strings.TrimSpace(strings.TrimPrefix(l, "package "))
						}
					}
				}
			}
		}
	}
	// Fallback to directory name or main
	base := filepath.Base(dir)
	if base == "." || base == "/" || base == "" {
		return "main"
	}
	return base
}

// Helpers for reading/updating Go
func readGoVersion(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	match := goVersionVarRegex.FindString(string(data))
	if match != "" {
		parts := strings.Split(match, `"`)
		if len(parts) >= 2 {
			return parts[1], true
		}
	}
	return "", false
}

func updateGoVersion(path string, version string, res *SyncResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(data)
	if goVersionVarRegex.MatchString(s) {
		newContent := goVersionVarRegex.ReplaceAllString(s, fmt.Sprintf(`${1} Version = %q`, version))
		if newContent != s {
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return err
			}
			res.UpdatedFiles = append(res.UpdatedFiles, path)
		}
	} else {
		// Append var Version
		newContent := strings.TrimRight(s, "\n") + fmt.Sprintf("\n\nvar Version = %q\n", version)
		if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
			return err
		}
		res.UpdatedFiles = append(res.UpdatedFiles, path)
	}
	return nil
}

// Helpers for reading/updating Zig
func readZigZonVersion(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	match := zigZonVerRegex.FindStringSubmatch(string(data))
	if len(match) > 0 {
		idx := strings.Index(match[0], `"`)
		lastIdx := strings.LastIndex(match[0], `"`)
		if idx >= 0 && lastIdx > idx {
			return match[0][idx+1 : lastIdx], true
		}
	}
	return "", false
}

func updateZigZonVersion(path string, version string, res *SyncResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(data)
	if zigZonVerRegex.MatchString(s) {
		newContent := zigZonVerRegex.ReplaceAllString(s, fmt.Sprintf(`${1}%q`, version))
		if newContent != s {
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return err
			}
			res.UpdatedFiles = append(res.UpdatedFiles, path)
		}
	}
	return nil
}

func readZigVersion(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	match := zigVersionVarRegex.FindString(string(data))
	if match != "" {
		idx := strings.Index(match, `"`)
		lastIdx := strings.LastIndex(match, `"`)
		if idx >= 0 && lastIdx > idx {
			return match[idx+1 : lastIdx], true
		}
	}
	return "", false
}

func updateZigVersion(path string, version string, res *SyncResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(data)
	if zigVersionVarRegex.MatchString(s) {
		newContent := zigVersionVarRegex.ReplaceAllString(s, fmt.Sprintf(`${1}%q;`, version))
		if newContent != s {
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return err
			}
			res.UpdatedFiles = append(res.UpdatedFiles, path)
		}
	}
	return nil
}

// Helpers for reading/updating Python
func readPyVersion(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	match := pyVersionVarRegex.FindString(string(data))
	if match != "" {
		parts := strings.FieldsFunc(match, func(r rune) bool {
			return r == '"' || r == '\''
		})
		if len(parts) >= 2 {
			return parts[1], true
		}
	}
	return "", false
}

func updatePyVersion(path string, version string, res *SyncResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(data)
	if pyVersionVarRegex.MatchString(s) {
		newContent := pyVersionVarRegex.ReplaceAllString(s, fmt.Sprintf(`${1} = %q`, version))
		if newContent != s {
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return err
			}
			res.UpdatedFiles = append(res.UpdatedFiles, path)
		}
	}
	return nil
}

func readPyprojectVersion(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	match := pyprojectVerRegex.FindString(string(data))
	if match != "" {
		parts := strings.FieldsFunc(match, func(r rune) bool {
			return r == '"' || r == '\''
		})
		if len(parts) >= 2 {
			return parts[1], true
		}
	}
	return "", false
}

func updatePyprojectVersion(path string, version string, res *SyncResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(data)
	if pyprojectVerRegex.MatchString(s) {
		newContent := pyprojectVerRegex.ReplaceAllString(s, fmt.Sprintf(`${1}%q`, version))
		if newContent != s {
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return err
			}
			res.UpdatedFiles = append(res.UpdatedFiles, path)
		}
	}
	return nil
}

// Helpers for reading/updating Rust
func readCargoVersion(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	match := cargoVersionRegex.FindString(string(data))
	if match != "" {
		parts := strings.FieldsFunc(match, func(r rune) bool {
			return r == '"' || r == '\''
		})
		if len(parts) >= 2 {
			return parts[1], true
		}
	}
	return "", false
}

func updateCargoVersion(path string, version string, res *SyncResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(data)
	if cargoVersionRegex.MatchString(s) {
		newContent := cargoVersionRegex.ReplaceAllString(s, fmt.Sprintf(`${1}%q`, version))
		if newContent != s {
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return err
			}
			res.UpdatedFiles = append(res.UpdatedFiles, path)
		}
	}
	return nil
}

// Helpers for reading/updating WebExtension manifest.json & package.json
func readManifestVersion(path string) (string, bool) {
	return readJSONVersion(path)
}

func readPackageJSONVersion(path string) (string, bool) {
	return readJSONVersion(path)
}

func readJSONVersion(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	matches := jsonVersionRegex.FindStringSubmatch(string(data))
	if len(matches) >= 3 {
		return matches[2], true
	}
	return "", false
}

func updateManifestVersion(path string, version string, res *SyncResult) error {
	return updateJSONVersion(path, version, res)
}

func updatePackageJSONVersion(path string, version string, res *SyncResult) error {
	return updateJSONVersion(path, version, res)
}

func updateJSONVersion(path string, version string, res *SyncResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	s := string(data)
	if jsonVersionRegex.MatchString(s) {
		newContent := jsonVersionRegex.ReplaceAllString(s, fmt.Sprintf(`${1}%q`, version))
		if newContent != s {
			if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
				return err
			}
			res.UpdatedFiles = append(res.UpdatedFiles, path)
		}
	}
	return nil
}

