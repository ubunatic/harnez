// Canary prototype for Multi-Doc Git History Token Evolution & Stacked Category History
// Issue 190: Phase 2 Validation
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"ubunatic.com/harnez/internal/assess"
)

func main() {
	jsonFlag := flag.Bool("json", false, "Output results as JSON")
	repoFlag := flag.String("repo", ".", "Path to git repository root")
	colorFlag := flag.Bool("color", true, "Enable ANSI colorized stacked bars and legend")
	noColorFlag := flag.Bool("no-color", false, "Disable ANSI colors")
	flag.Parse()

	useColor := *colorFlag && !*noColorFlag

	targets := flag.Args()
	if len(targets) == 0 {
		targets = []string{"docs", "AGENTS.md"}
	}

	start := time.Now()
	res, err := assess.ExtractMultiDocHistory(*repoFlag, targets)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting multi-doc history: %v\n", err)
		os.Exit(1)
	}
	res.Duration = time.Since(start)

	if *jsonFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			fmt.Fprintf(os.Stderr, "JSON encode error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Print(assess.RenderMultiDocHistory(res, assess.RenderMultiDocOptions{Color: useColor}))
}
