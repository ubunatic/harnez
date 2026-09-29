package skillreg

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// VendorSource is one bundled_skills entry, with Dir resolved on disk.
type VendorSource struct {
	URL    string
	Commit string
	Dir    string // absolute vendor dir, e.g. <repo>/third_party/skills/mattpocock
	Skills []string
}

// VendoredSkill reports what vendoring did to one skill.
type VendoredSkill struct {
	Name       string
	Status     string // Changed, Unchanged, Gone, or "new" on first vendoring
	DiffStat   string
	Candidates []string
	Hints      []string
}

// VendorReport is the result of Vendor for one source.
type VendorReport struct {
	Commit     string
	Skills     []VendoredSkill
	Removed    []string // vendored dirs no longer listed
	NotBundled []string // upstream skills not in the list
}

// Vendor copies the listed skills from URL at Commit into Dir, plus the
// repository's LICENSE files, and reports per-skill changes. A skill gone
// upstream keeps its vendored copy and gets rename hints, as in PlanUpdate.
func Vendor(src VendorSource) (VendorReport, error) {
	var rep VendorReport
	for _, name := range src.Skills {
		if !NameOK(name) {
			return rep, fmt.Errorf("bundled skill %q: not a safe directory name", name)
		}
	}
	tmp, err := os.MkdirTemp("", "harnez-vendor-")
	if err != nil {
		return rep, err
	}
	defer os.RemoveAll(tmp)
	clone := filepath.Join(tmp, "repo")
	if rep.Commit, err = Clone(src.URL, src.Commit, clone); err != nil {
		return rep, err
	}
	top, err := os.ReadDir(clone)
	if err != nil {
		return rep, err
	}
	var licenses []string
	for _, e := range top {
		n := strings.ToUpper(e.Name())
		if !e.IsDir() && (strings.HasPrefix(n, "LICENSE") || strings.HasPrefix(n, "COPYING")) {
			licenses = append(licenses, e.Name())
		}
	}
	if len(licenses) == 0 {
		return rep, fmt.Errorf("%s has no LICENSE file; not vendoring code without a license", src.URL)
	}
	skills, err := FindSkills(clone)
	if err != nil {
		return rep, err
	}
	if err := os.MkdirAll(src.Dir, 0o755); err != nil {
		return rep, err
	}
	for _, name := range src.Skills {
		v := VendoredSkill{Name: name}
		dst := filepath.Join(src.Dir, name)
		i := slices.IndexFunc(skills, func(s Skill) bool { return s.Name == name })
		if i < 0 {
			v.Status = Gone
			v.Candidates, v.Hints = renameHints(clone, name, skills)
			rep.Skills = append(rep.Skills, v)
			continue
		}
		newDir := filepath.Join(clone, skills[i].Path)
		if _, err := os.Stat(dst); err != nil {
			v.Status = "new"
		} else {
			d := filepath.Join(tmp, "diff-"+name)
			if err := os.MkdirAll(d, 0o755); err != nil {
				return rep, err
			}
			stat, full, err := diffDirs(dst, newDir, d)
			if err != nil {
				return rep, err
			}
			v.DiffStat, v.Status = stat, Unchanged
			if full != "" {
				v.Status = Changed
			}
		}
		if v.Status != Unchanged {
			if err := os.RemoveAll(dst); err != nil {
				return rep, err
			}
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return rep, err
			}
			if err := copyTree(newDir, dst); err != nil {
				return rep, err
			}
		}
		rep.Skills = append(rep.Skills, v)
	}
	for _, s := range skills {
		if !slices.Contains(src.Skills, s.Name) {
			rep.NotBundled = append(rep.NotBundled, s.Name)
		}
	}
	entries, err := os.ReadDir(src.Dir)
	if err != nil {
		return rep, err
	}
	for _, e := range entries {
		// Only former skill dirs go, so a wrong dir never wipes unrelated files.
		if _, err := os.Stat(filepath.Join(src.Dir, e.Name(), "SKILL.md")); e.IsDir() && err == nil && !slices.Contains(src.Skills, e.Name()) {
			if err := os.RemoveAll(filepath.Join(src.Dir, e.Name())); err != nil {
				return rep, err
			}
			rep.Removed = append(rep.Removed, e.Name())
		}
	}
	for _, l := range licenses {
		data, err := os.ReadFile(filepath.Join(clone, l))
		if err != nil {
			return rep, err
		}
		if err := os.WriteFile(filepath.Join(src.Dir, l), data, 0o644); err != nil {
			return rep, err
		}
	}
	return rep, nil
}
