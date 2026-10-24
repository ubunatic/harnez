// Package dependency installs optional tools described by the embedded spec.
package dependency

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"ubunatic.com/harnez"
)

type packageSpec struct {
	Module    string   `yaml:"module"`
	Bootstrap []string `yaml:"bootstrap"`
}

func loadPackages() (map[string]packageSpec, error) {
	data, err := harnez.DefaultFS.ReadFile("spec/dependencies.yaml")
	if err != nil {
		return nil, err
	}
	var spec struct {
		Packages map[string]packageSpec `yaml:"packages"`
	}
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	if len(spec.Packages) == 0 {
		return nil, fmt.Errorf("dependency spec: no packages")
	}
	for name, pkg := range spec.Packages {
		if pkg.Module == "" || len(pkg.Bootstrap) == 0 || pkg.Bootstrap[0] == "" {
			return nil, fmt.Errorf("dependency spec: incomplete package %q", name)
		}
	}
	return spec.Packages, nil
}

// Install runs the Go installation followed by the package's bootstrap command.
// Dry runs only print argv; they do not resolve or execute any program.
func Install(ctx context.Context, name string, dryRun bool, in io.Reader, out, stderr io.Writer) error {
	packages, err := loadPackages()
	if err != nil {
		return err
	}
	pkg, ok := packages[name]
	if !ok {
		names := make([]string, 0, len(packages))
		for name := range packages {
			names = append(names, name)
		}
		sort.Strings(names)
		return fmt.Errorf("unknown dependency %q; supported: %s", name, strings.Join(names, ", "))
	}
	for _, argv := range [][]string{{"go", "install", pkg.Module}, pkg.Bootstrap} {
		if _, err := fmt.Fprintln(out, strings.Join(argv, " ")); err != nil {
			return err
		}
		if dryRun {
			continue
		}
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", strings.Join(argv, " "), err)
		}
	}
	return nil
}
