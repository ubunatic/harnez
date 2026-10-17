// Command gotest-summary turns `go test -json` output into a Markdown failure
// report and GitHub error annotations. CI logs need admin rights to download,
// but the step summary (web) and annotations (`gh run view`) do not.
//
// Usage: gotest-summary [-tail N] [-summary FILE] [-annotate] [test.json]
//
// Lines that are not test2json events (gofmt, go vet output) are kept and
// reported as a tail too. The exit status is 0 whether or not tests failed.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type event struct {
	Action     string
	Package    string
	Test       string
	Output     string
	ImportPath string
}

type report struct {
	failed []string            // "pkg Test" or "pkg"
	output map[string][]string // key -> output lines
	other  []string            // non-JSON lines
}

func key(pkg, test string) string {
	if test == "" {
		return pkg
	}
	return pkg + " " + test
}

func parse(r io.Reader) (*report, error) {
	rep := &report{output: map[string][]string{}}
	seen := map[string]bool{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 16<<20)
	for sc.Scan() {
		line := sc.Text()
		var e event
		if !strings.HasPrefix(line, "{") || json.Unmarshal([]byte(line), &e) != nil || e.Action == "" {
			rep.other = append(rep.other, line)
			continue
		}
		switch e.Action {
		case "output":
			k := key(e.Package, e.Test)
			rep.output[k] = append(rep.output[k], strings.TrimRight(e.Output, "\n"))
		case "build-output":
			// ImportPath is "pkg [pkg.test]"; the package's own fail event
			// (with FailedBuild) lists it, so only keep the output here.
			pkg, _, _ := strings.Cut(e.ImportPath, " [")
			rep.output[pkg] = append(rep.output[pkg], strings.TrimRight(e.Output, "\n"))
		case "fail":
			k := key(e.Package, e.Test)
			if !seen[k] {
				seen[k] = true
				rep.failed = append(rep.failed, k)
			}
		}
	}
	return rep, sc.Err()
}

// tests returns the failing tests; a failed package is listed only when
// none of its tests failed (build errors, TestMain, panics outside tests).
func (r *report) tests() []string {
	withTests := map[string]bool{}
	for _, k := range r.failed {
		if pkg, _, ok := strings.Cut(k, " "); ok {
			withTests[pkg] = true
		}
	}
	var out []string
	for _, k := range r.failed {
		if strings.Contains(k, " ") || !withTests[k] {
			out = append(out, k)
		}
	}
	// Subtests fail with their parent; keep the leaf names only.
	var leaves []string
	for _, k := range out {
		leaf := true
		for _, other := range out {
			if other != k && strings.HasPrefix(other, k+"/") {
				leaf = false
				break
			}
		}
		if leaf {
			leaves = append(leaves, k)
		}
	}
	sort.Strings(leaves)
	return leaves
}

func tail(lines []string, n int) []string {
	if len(lines) > n {
		return lines[len(lines)-n:]
	}
	return lines
}

func markdown(w io.Writer, r *report, n int) {
	failed := r.tests()
	if len(failed) == 0 {
		fmt.Fprintln(w, "### go test: no failing tests")
	} else {
		fmt.Fprintf(w, "### go test: %d failing\n\n", len(failed))
		for _, k := range failed {
			fmt.Fprintf(w, "- `%s`\n", k)
		}
		for _, k := range failed {
			fmt.Fprintf(w, "\n<details open><summary><code>%s</code></summary>\n\n```text\n%s\n```\n</details>\n", k, strings.Join(tail(r.output[k], n), "\n"))
		}
	}
	if len(r.other) > 0 {
		fmt.Fprintf(w, "\n### Other output (last %d lines)\n\n```text\n%s\n```\n", n, strings.Join(tail(r.other, n), "\n"))
	}
}

// escape encodes a workflow command message (%, CR, LF).
func escape(s string) string {
	return strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A").Replace(s)
}

// escapeProp also encodes the property separators : and ,.
func escapeProp(s string) string {
	return strings.NewReplacer(":", "%3A", ",", "%2C").Replace(escape(s))
}

func annotate(w io.Writer, r *report, n int) {
	for _, k := range r.tests() {
		fmt.Fprintf(w, "::error title=%s::%s\n", escapeProp(k), escape(strings.Join(tail(r.output[k], n), "\n")))
	}
}

func main() {
	n := flag.Int("tail", 40, "output lines per failing test")
	summary := flag.String("summary", os.Getenv("GITHUB_STEP_SUMMARY"), "append Markdown here (default stdout)")
	ann := flag.Bool("annotate", os.Getenv("GITHUB_ACTIONS") == "true", "print GitHub ::error annotations to stdout")
	flag.Parse()
	in := io.Reader(os.Stdin)
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		in = f
	}
	rep, err := parse(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out := io.Writer(os.Stdout)
	if *summary != "" {
		f, err := os.OpenFile(*summary, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		out = f
	}
	markdown(out, rep, *n)
	if *ann {
		annotate(os.Stdout, rep, 10)
	}
}
