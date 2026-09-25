// Package finder coordinates external code and documentation search tools.
package finder

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Definition struct {
	Name    string   `yaml:"name" json:"name"`
	Scope   string   `yaml:"scope" json:"scope"`
	Command []string `yaml:"command" json:"command"`
	Timeout string   `yaml:"timeout" json:"timeout"`
	builtin bool
}
type Result struct {
	Path    string  `json:"path"`
	Line    int     `json:"line"`
	Title   string  `json:"title"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
	Kind    string  `json:"kind"`
}
type Status struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}
type Response struct {
	Results []Result `json:"results"`
	Finders []Status `json:"finders"`
}
type config struct {
	Finders []Definition `yaml:"finders"`
}

const maxOutputBytes = 4 << 20

type cappedBuffer struct {
	bytes.Buffer
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	left := maxOutputBytes - b.Len()
	if left > 0 {
		if left > len(p) {
			left = len(p)
		}
		_, _ = b.Buffer.Write(p[:left])
	}
	if left < n {
		b.truncated = true
	}
	return n, nil
}

func Registry(root string) ([]Definition, error) {
	var all []Definition
	if h := os.Getenv("HOME"); h != "" {
		user, err := readErr(filepath.Join(h, ".harnez", "config.yaml"))
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		all = append(all, user...)
	}
	project, err := readErr(filepath.Join(root, "config.yaml"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	by := map[string]Definition{}
	for _, d := range all {
		by[d.Name] = d
	}
	for _, d := range project {
		by[d.Name] = d
	}
	for _, d := range by {
		if err := validate(d); err != nil {
			return nil, err
		}
		all = append(all, d)
	}
	// Replace duplicates after combining user and project definitions.
	unique := map[string]Definition{}
	for _, d := range all {
		unique[d.Name] = d
	}
	all = all[:0]
	for _, d := range unique {
		all = append(all, d)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	return all, nil
}
func readErr(path string) ([]Definition, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c config
	if e = yaml.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	return c.Finders, nil
}
func validate(d Definition) error {
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`).MatchString(d.Name) || (d.Scope != "code" && d.Scope != "docs") || len(d.Command) == 0 || strings.TrimSpace(d.Command[0]) == "" || d.Timeout == "" {
		return fmt.Errorf("invalid finder definition %q", d.Name)
	}
	timeout, e := time.ParseDuration(d.Timeout)
	if e != nil {
		return fmt.Errorf("finder %s timeout: %w", d.Name, e)
	}
	if timeout <= 0 {
		return fmt.Errorf("finder %s timeout must be positive", d.Name)
	}
	return nil
}

func Search(ctx context.Context, root, scope, query, via string, k int, stderr io.Writer) (Response, error) {
	if k <= 0 {
		k = 10
	}
	abs, e := filepath.Abs(root)
	if e != nil {
		return Response{}, e
	}
	defs, e := Registry(abs)
	if e != nil {
		return Response{}, e
	}
	selected := []Definition{}
	for _, d := range defs {
		if d.Scope == scope {
			selected = append(selected, d)
		}
	}
	if via != "" {
		found := false
		for _, d := range defs {
			if d.Name == via {
				found = true
				if d.Scope != scope {
					return Response{}, fmt.Errorf("finder %q is outside %s scope", via, scope)
				}
				selected = []Definition{d}
			}
		}
		if !found {
			return Response{}, fmt.Errorf("unknown finder %q", via)
		}
	}
	if scope != "code" && scope != "docs" {
		return Response{}, fmt.Errorf("unknown scope %q", scope)
	}
	// Defaults are independent fallback finders. neus is preferred where installed.
	if via == "" {
		_, neusErr := exec.LookPath("neus")
		neusAvailable := neusErr == nil
		has := func(name string) bool {
			for _, d := range selected {
				if d.Name == name {
					return true
				}
			}
			return false
		}
		if neusAvailable && !has("neus") {
			selected = append(selected, Definition{Name: "neus", Scope: scope, Timeout: "30s", builtin: true})
		}
		if scope == "code" {
			if !neusAvailable && !has("rg") {
				selected = append(selected, Definition{Name: "rg", Scope: scope, Timeout: "10s", builtin: true})
			}
		} else if !neusAvailable && !has("fuzzy") {
			selected = append(selected, Definition{Name: "fuzzy", Scope: scope, Timeout: "10s", builtin: true})
		}
	}
	resp := Response{Results: []Result{}, Finders: []Status{}}
	type part struct {
		rs   []Result
		s    Status
		rank int
	}
	parts := make([]part, len(selected))
	var wg sync.WaitGroup
	for i, d := range selected {
		wg.Add(1)
		go func(i int, d Definition) {
			defer wg.Done()
			rs, status := run(ctx, abs, d, query, k)
			parts[i] = part{rs, status, i}
		}(i, d)
	}
	wg.Wait()
	type ranked struct {
		r     Result
		score float64
	}
	byPath := map[string]*ranked{}
	for _, p := range parts {
		resp.Finders = appendStatus(resp.Finders, p.s)
		for rank, r := range p.rs {
			key := filepath.Clean(r.Path) + ":" + fmt.Sprint(r.Line)
			item := byPath[key]
			if item == nil {
				item = &ranked{r: r}
				byPath[key] = item
			}
			item.score += 1 / float64(60+rank+1)
			if item.r.Title == "" {
				item.r.Title = r.Title
			}
			if item.r.Snippet == "" {
				item.r.Snippet = r.Snippet
			}
			if item.r.Kind == "" {
				item.r.Kind = r.Kind
			}
		}
		if p.s.Status != "ok" && stderr != nil {
			fmt.Fprintf(stderr, "finder %s: %s %s\n", p.s.Name, p.s.Status, p.s.Error)
		}
	}
	merged := make([]ranked, 0, len(byPath))
	for _, item := range byPath {
		item.r.Score = item.score
		merged = append(merged, *item)
	}
	sort.SliceStable(merged, func(i, j int) bool {
		if merged[i].score != merged[j].score {
			return merged[i].score > merged[j].score
		}
		if merged[i].r.Path != merged[j].r.Path {
			return merged[i].r.Path < merged[j].r.Path
		}
		return merged[i].r.Line < merged[j].r.Line
	})
	for _, x := range merged {
		if len(resp.Results) >= k {
			break
		}
		resp.Results = append(resp.Results, x.r)
	}
	return resp, nil
}
func appendStatus(dst []Status, s Status) []Status { return append(dst, s) }
func run(parent context.Context, root string, d Definition, q string, k int) ([]Result, Status) {
	status := Status{Name: d.Name, Status: "ok"}
	timeout := 10 * time.Second
	if d.Timeout != "" {
		if t, e := time.ParseDuration(d.Timeout); e == nil {
			timeout = t
		}
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	if d.builtin && d.Name == "fuzzy" {
		results, err := BuiltinDocs(ctx, root, q, k)
		if err != nil {
			if ctx.Err() != nil {
				status.Status = "timeout"
				status.Error = ctx.Err().Error()
			} else {
				status.Status = "error"
				status.Error = err.Error()
			}
			return nil, status
		}
		return results, status
	}
	var argv []string
	switch d.Name {
	case "neus":
		if d.builtin {
			argv = []string{"neus", "search", "--json", "--root", root, "--kind", d.Scope, "-k", fmt.Sprint(k), "--timeout", timeout.String(), q}
		} else {
			argv = expand(d.Command, q, root, k)
		}
	case "rg":
		if d.builtin {
			argv = []string{"rg", "--line-number", "--no-heading", "--color", "never", "--", q, root}
		} else {
			argv = expand(d.Command, q, root, k)
		}
	default:
		argv = expand(d.Command, q, root, k)
	}
	if len(argv) == 0 {
		status.Status = "error"
		status.Error = "empty command"
		return nil, status
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var output cappedBuffer
	cmd.Stdout = &output
	e := cmd.Run()
	out := output.Bytes()
	if output.truncated {
		status.Status = "error"
		status.Error = "output exceeded 4 MiB"
		return nil, status
	}
	if e != nil {
		status.Status = "error"
		var ee *exec.ExitError
		if errors.As(e, &ee) && d.Name == "neus" && ee.ExitCode() == 3 {
			status.Status = "not-indexed"
			status.Error = "root is not indexed"
			index := exec.CommandContext(ctx, "neus", "index", root)
			if index.Run() == nil {
				retry := exec.CommandContext(ctx, argv[0], argv[1:]...)
				output = cappedBuffer{}
				retry.Stdout = &output
				e = retry.Run()
				out = output.Bytes()
				if output.truncated {
					status.Status = "error"
					status.Error = "output exceeded 4 MiB"
					return nil, status
				}
				if e == nil {
					status.Status = "ok"
					status.Error = ""
				}
			}
			if e != nil {
				return nil, status
			}
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status.Status = "timeout"
			status.Error = "deadline exceeded"
		} else {
			status.Error = e.Error()
			return nil, status
		}
	}
	if d.builtin && d.Name == "rg" {
		return parseRG(out, root), status
	}
	rs, e := parseJSON(out, root)
	if e != nil {
		status.Status = "error"
		status.Error = e.Error()
		return nil, status
	}
	return rs, status
}
func expand(t []string, q, root string, k int) []string {
	o := make([]string, len(t))
	for i, s := range t {
		s = strings.ReplaceAll(s, "{query}", q)
		s = strings.ReplaceAll(s, "{root}", root)
		s = strings.ReplaceAll(s, "{k}", fmt.Sprint(k))
		o[i] = s
	}
	return o
}
func parseJSON(b []byte, root string) ([]Result, error) {
	var rs []Result
	if json.Unmarshal(b, &rs) == nil {
		for i := range rs {
			rs[i].Path = relative(rs[i].Path, root)
		}
		return rs, nil
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		var r Result
		if e := json.Unmarshal(sc.Bytes(), &r); e != nil {
			return nil, e
		}
		if r.Path == "" {
			continue
		}
		r.Path = relative(r.Path, root)
		rs = append(rs, r)
	}
	return rs, sc.Err()
}
func relative(p, root string) string {
	if filepath.IsAbs(p) {
		if r, e := filepath.Rel(root, p); e == nil && !strings.HasPrefix(r, "..") {
			return filepath.ToSlash(r)
		}
	}
	return filepath.ToSlash(p)
}
func parseRG(b []byte, root string) []Result {
	var rs []Result
	for _, line := range strings.Split(string(b), "\n") {
		p, tail, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, text, ok := strings.Cut(tail, ":")
		if !ok {
			continue
		}
		var ln int
		fmt.Sscan(n, &ln)
		rs = append(rs, Result{Path: relative(p, root), Line: ln, Title: filepath.Base(p), Snippet: text, Kind: "code"})
	}
	return rs
}
func BuiltinDocs(ctx context.Context, root, q string, k int) ([]Result, error) {
	terms := strings.Fields(strings.ToLower(q))
	var out []Result
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if e != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(p), ".md") {
			return nil
		}
		f, e := os.Open(p)
		if e != nil {
			return nil
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for i := 1; scanner.Scan(); i++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			l := scanner.Text()
			low := strings.ToLower(l)
			matched := true
			for _, term := range terms {
				if !strings.Contains(low, term) {
					matched = false
					break
				}
			}
			if matched && len(terms) > 0 {
				out = append(out, Result{Path: relative(p, root), Line: i, Title: filepath.Base(p), Snippet: strings.TrimSpace(l), Kind: "docs"})
				if k > 0 && len(out) >= k {
					f.Close()
					return filepath.SkipAll
				}
			}
		}
		if err := scanner.Err(); err != nil {
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, filepath.SkipAll) {
		return nil, err
	}
	return out, nil
}
