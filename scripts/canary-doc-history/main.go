// Canary prototype for Git-History Time-Series Telemetry & Token/Size Evolution
// Issue 189: Phase 1 Validation
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
	flag.Parse()

	targetFile := "docs/lang/Go.md"
	if flag.NArg() > 0 {
		targetFile = flag.Arg(0)
	}

	start := time.Now()
	res, err := assess.ExtractDocHistory(*repoFlag, targetFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error extracting history for %s: %v\n", targetFile, err)
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

	fmt.Print(assess.RenderDocHistoryTable(res))
}
