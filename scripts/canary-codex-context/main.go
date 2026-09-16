// Canary for measuring how Codex configuration changes first-turn input usage.
// Run with: go run ./scripts/canary-codex-context -dir /tmp -candidate features.apps=false
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type configFlags []string

func (f *configFlags) String() string { return strings.Join(*f, ", ") }

func (f *configFlags) Set(value string) error {
	if !strings.Contains(value, "=") {
		return fmt.Errorf("config override %q must be key=value", value)
	}
	*f = append(*f, value)
	return nil
}

type usage struct {
	InputTokens           int `json:"input_tokens"`
	CachedInputTokens     int `json:"cached_input_tokens"`
	CacheWriteInputTokens int `json:"cache_write_input_tokens"`
	OutputTokens          int `json:"output_tokens"`
}

type event struct {
	Type  string `json:"type"`
	Usage usage  `json:"usage"`
	Item  struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"item"`
	Error json.RawMessage `json:"error"`
}

type result struct {
	Usage usage
	Reply string
}

func parseEvents(data []byte) (result, error) {
	var got result
	completed := 0
	scan := bufio.NewScanner(bytes.NewReader(data))
	scan.Buffer(make([]byte, 4096), 4<<20)
	for scan.Scan() {
		var e event
		if err := json.Unmarshal(scan.Bytes(), &e); err != nil {
			return got, fmt.Errorf("decode Codex JSONL: %w", err)
		}
		switch e.Type {
		case "turn.completed":
			completed++
			got.Usage = e.Usage
		case "turn.failed", "error":
			return got, fmt.Errorf("Codex reported %s: %s", e.Type, e.Error)
		case "item.completed":
			if e.Item.Type == "agent_message" {
				got.Reply = e.Item.Text
			}
		}
	}
	if err := scan.Err(); err != nil {
		return got, fmt.Errorf("scan Codex JSONL: %w", err)
	}
	if completed != 1 || got.Usage.InputTokens <= 0 {
		return got, fmt.Errorf("expected one completed turn with input usage, got %d", completed)
	}
	return got, nil
}

func run(ctx context.Context, codex, dir, prompt string, overrides []string) (result, error) {
	args := []string{"exec", "--ephemeral", "--json", "--skip-git-repo-check", "-C", dir}
	for _, override := range overrides {
		args = append(args, "-c", override)
	}
	args = append(args, prompt)
	cmd := exec.CommandContext(ctx, codex, args...)
	cmd.Stdin = strings.NewReader("")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		return result{}, fmt.Errorf("codex exec: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	got, err := parseEvents(stdout)
	if err != nil {
		return got, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if strings.TrimSpace(got.Reply) != "OK" {
		return got, fmt.Errorf("unexpected reply %q; the prompt may have triggered extra work", got.Reply)
	}
	return got, nil
}

func main() {
	dirFlag := flag.String("dir", ".", "working directory for both Codex runs")
	pairsFlag := flag.Int("pairs", 2, "number of baseline/candidate pairs")
	timeoutFlag := flag.Duration("timeout", 2*time.Minute, "timeout for each Codex run")
	var candidate configFlags
	flag.Var(&candidate, "candidate", "Codex config key=value override; repeatable")
	flag.Parse()
	if *pairsFlag < 1 || len(candidate) == 0 {
		fmt.Fprintln(os.Stderr, "usage: go run ./scripts/canary-codex-context -candidate key=value [-candidate key=value] [-pairs 2] [-dir .]")
		os.Exit(2)
	}
	codex, err := exec.LookPath("codex")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	dir, err := filepath.Abs(*dirFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "not a directory: %s\n", dir)
		os.Exit(1)
	}
	version, err := exec.Command(codex, "--version").Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "codex --version: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Codex %s\nDirectory: %s\nCandidate: %s\n", strings.TrimSpace(string(version)), dir, candidate)
	fmt.Println("Fresh ephemeral sessions; input_tokens is the reported total, cached_input_tokens is shown separately.")
	fmt.Println("pair  variant    input  cached  cache-write  output")
	const prompt = "Reply with exactly OK."
	var baselineTotal, candidateTotal int
	for pair := 1; pair <= *pairsFlag; pair++ {
		variants := []string{"baseline", "candidate"}
		if pair%2 == 0 {
			variants[0], variants[1] = variants[1], variants[0]
		}
		for _, variant := range variants {
			var overrides []string
			if variant == "candidate" {
				overrides = candidate
			}
			ctx, cancel := context.WithTimeout(context.Background(), *timeoutFlag)
			got, err := run(ctx, codex, dir, prompt, overrides)
			cancel()
			if err != nil {
				fmt.Fprintf(os.Stderr, "pair %d %s: %v\n", pair, variant, err)
				os.Exit(1)
			}
			fmt.Printf("%4d  %-9s %6d  %6d  %11d  %6d\n", pair, variant,
				got.Usage.InputTokens, got.Usage.CachedInputTokens,
				got.Usage.CacheWriteInputTokens, got.Usage.OutputTokens)
			if variant == "baseline" {
				baselineTotal += got.Usage.InputTokens
			} else {
				candidateTotal += got.Usage.InputTokens
			}
		}
	}
	baselineMean := float64(baselineTotal) / float64(*pairsFlag)
	candidateMean := float64(candidateTotal) / float64(*pairsFlag)
	fmt.Printf("Mean input: baseline %.0f, candidate %.0f; delta %+.0f (%+.1f%%)\n",
		baselineMean, candidateMean, candidateMean-baselineMean,
		100*(candidateMean-baselineMean)/baselineMean)
}
